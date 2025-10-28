"""HTTP route definitions for the identity service."""

from __future__ import annotations

import hashlib
import logging

from datetime import datetime
from typing import Any

from fastapi import APIRouter, Depends, Header, HTTPException, Query, Request, Response, status
from pydantic import BaseModel, EmailStr, Field

from ..config import get_settings
from ..domain.account import Account
from ..domain.contracts import CreateAccountInput
from ..domain.service import AccountService
from ..security.rate_limiter import SlidingWindowRateLimiter
from ..security.redis_rate_limiter import RedisSlidingWindowRateLimiter

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/v1")


class AccountResponse(BaseModel):
    """Serialised representation of an `Account` aggregate."""

    account_id: str
    tenant_id: str
    email: EmailStr
    created_at: str
    disabled: bool

    @classmethod
    def from_domain(cls, account: Account) -> "AccountResponse":
        """Build a response model from the domain aggregate."""
        return cls(
            account_id=account.account_id,
            tenant_id=account.tenant_id,
            email=account.email,
            created_at=account.created_at.isoformat(),
            disabled=account.disabled,
        )


class CreateAccountRequest(BaseModel):
    """Payload accepted when creating a tenant-scoped account."""

    tenant_id: str = Field(..., alias="tenant_id")
    email: EmailStr
    disabled: bool = False


class CreateAccountResponse(BaseModel):
    """Response returned after processing an account creation request."""

    account: AccountResponse
    idempotent_replay: bool


class TokenRequest(BaseModel):
    """JSON body used to request a JWT for an account."""

    account_id: str
    tenant_id: str
    scopes: list[str] | None = None


class TokenResponse(BaseModel):
    """Token issuance response containing the bearer token and metadata."""

    access_token: str
    token_type: str = "bearer"
    expires_in: int
    refresh_token: str
    refresh_expires_in: int
    tenant_id: str


class RefreshTokenRequest(BaseModel):
    """Request body for exchanging refresh tokens for new access credentials."""

    refresh_token: str
    scopes: list[str] | None = None


class AuditLogEntry(BaseModel):
    """Audit log response entry."""

    audit_id: int
    account_id: str | None
    tenant_id: str | None
    event_type: str
    actor: str | None
    metadata: dict[str, Any]
    created_at: datetime


class AuditLogResponse(BaseModel):
    """Envelope for paginated audit log data."""

    items: list[AuditLogEntry]
    next_cursor: str | None = None


settings = get_settings()


def _build_rate_limiter() -> SlidingWindowRateLimiter | RedisSlidingWindowRateLimiter:
    """Instantiate the configured rate limiter backend, preferring Redis when available."""
    if settings.rate_limit_backend == "redis" and settings.redis_url:
        try:
            import redis

            client = redis.from_url(settings.redis_url)
            # ensure connectivity early to fail fast and fall back
            client.ping()
            logger.info("rate limiter configured for redis backend at %s", settings.redis_url)
            return RedisSlidingWindowRateLimiter(
                client,
                max_requests=settings.rate_limit_requests,
                window_seconds=settings.rate_limit_window_seconds,
            )
        except Exception as exc:  # pragma: no cover - defensive
            logger.warning("redis rate limiter unavailable, falling back to in-memory: %s", exc)

    logger.info("rate limiter using in-memory backend")
    return SlidingWindowRateLimiter(
        max_requests=settings.rate_limit_requests,
        window_seconds=settings.rate_limit_window_seconds,
    )


rate_limiter = _build_rate_limiter()


def get_service(request: Request) -> AccountService:
    """Resolve the `AccountService` stored on the FastAPI application state."""
    service: AccountService = request.app.state.account_service
    return service


@router.post("/accounts", response_model=CreateAccountResponse, status_code=status.HTTP_201_CREATED)
def create_account(
    response: Response,
    payload: CreateAccountRequest,
    service: AccountService = Depends(get_service),
    idempotency_key: str | None = Header(default=None, alias="Idempotency-Key"),
) -> CreateAccountResponse:
    """Create an account with optional idempotency semantics."""
    rate_key = f"create:{payload.tenant_id}"
    if not rate_limiter.allow(rate_key):
        raise HTTPException(status_code=status.HTTP_429_TOO_MANY_REQUESTS, detail="rate limited")
    account, replay = service.create_account(
        CreateAccountInput(
            tenant_id=payload.tenant_id,
            email=payload.email,
            disabled=payload.disabled,
        ),
        idempotency_key,
    )
    response.status_code = status.HTTP_200_OK if replay else status.HTTP_201_CREATED
    return CreateAccountResponse(
        account=AccountResponse.from_domain(account),
        idempotent_replay=replay,
    )


@router.get("/accounts/{account_id}", response_model=AccountResponse)
def get_account(
    account_id: str,
    tenant_id: str = Header(..., alias="X-Tenant-ID"),
    service: AccountService = Depends(get_service),
) -> AccountResponse:
    """Retrieve an account belonging to the requester tenant."""
    account = service.get_account(account_id, tenant_id)
    if account is None:
        raise HTTPException(status_code=status.HTTP_404_NOT_FOUND, detail="account not found")
    return AccountResponse.from_domain(account)


@router.post("/token", response_model=TokenResponse)
def issue_token(
    payload: TokenRequest,
    service: AccountService = Depends(get_service),
) -> TokenResponse:
    """Issue a signed access token for the specified account."""
    rate_key = f"token:{payload.tenant_id}:{payload.account_id}"
    if not rate_limiter.allow(rate_key):
        raise HTTPException(status_code=status.HTTP_429_TOO_MANY_REQUESTS, detail="rate limited")
    try:
        bundle = service.issue_token(payload.account_id, payload.tenant_id, payload.scopes)
    except ValueError as exc:
        raise _http_error_from_value_error(exc) from exc
    return TokenResponse(
        access_token=bundle.access_token,
        expires_in=bundle.access_expires_in,
        refresh_token=bundle.refresh_token,
        refresh_expires_in=bundle.refresh_expires_in,
        tenant_id=bundle.tenant_id,
    )


@router.post("/token/refresh", response_model=TokenResponse)
def refresh_token(
    payload: RefreshTokenRequest,
    service: AccountService = Depends(get_service),
) -> TokenResponse:
    token_hash = hashlib.sha256(payload.refresh_token.encode("utf-8")).hexdigest()[:12]
    rate_key = f"token-refresh:{token_hash}"
    if not rate_limiter.allow(rate_key):
        raise HTTPException(status_code=status.HTTP_429_TOO_MANY_REQUESTS, detail="rate limited")
    try:
        bundle = service.refresh_access_token(payload.refresh_token, payload.scopes)
    except ValueError as exc:
        raise _http_error_from_value_error(exc) from exc
    return TokenResponse(
        access_token=bundle.access_token,
        expires_in=bundle.access_expires_in,
        refresh_token=bundle.refresh_token,
        refresh_expires_in=bundle.refresh_expires_in,
        tenant_id=bundle.tenant_id,
    )


@router.get("/audit/logs", response_model=AuditLogResponse)
def list_audit_logs(
    tenant_id: str = Header(..., alias="X-Tenant-ID"),
    account_id: str | None = Query(default=None),
    event_type: str | None = Query(default=None),
    created_after: datetime | None = Query(default=None),
    created_before: datetime | None = Query(default=None),
    limit: int = Query(default=50, ge=1, le=100),
    cursor: str | None = Query(default=None),
    service: AccountService = Depends(get_service),
) -> AuditLogResponse:
    """Return paginated audit events for the tenant with optional filtering."""
    try:
        records, next_cursor = service.list_audit_events(
            tenant_id=tenant_id,
            account_id=account_id,
            event_type=event_type,
            created_after=created_after,
            created_before=created_before,
            limit=limit,
            cursor=cursor,
        )
    except ValueError as exc:
        raise HTTPException(status_code=status.HTTP_400_BAD_REQUEST, detail=str(exc)) from exc

    items = [
        AuditLogEntry(
            audit_id=record.audit_id,
            account_id=record.account_id,
            tenant_id=record.tenant_id,
            event_type=record.event_type,
            actor=record.actor,
            metadata=record.metadata,
            created_at=record.created_at,
        )
        for record in records
    ]
    return AuditLogResponse(items=items, next_cursor=next_cursor)


def _http_error_from_value_error(exc: ValueError) -> HTTPException:
    message = str(exc).lower()
    status_code = status.HTTP_400_BAD_REQUEST
    if "not found" in message:
        status_code = status.HTTP_404_NOT_FOUND
    return HTTPException(status_code=status_code, detail=str(exc))
