from __future__ import annotations

import logging

from fastapi import APIRouter, Depends, Header, HTTPException, Request, Response, status
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
    account_id: str
    tenant_id: str
    email: EmailStr
    created_at: str
    disabled: bool

    @classmethod
    def from_domain(cls, account: Account) -> "AccountResponse":
        return cls(
            account_id=account.account_id,
            tenant_id=account.tenant_id,
            email=account.email,
            created_at=account.created_at.isoformat(),
            disabled=account.disabled,
        )


class CreateAccountRequest(BaseModel):
    tenant_id: str = Field(..., alias="tenant_id")
    email: EmailStr
    disabled: bool = False


class CreateAccountResponse(BaseModel):
    account: AccountResponse
    idempotent_replay: bool


class TokenRequest(BaseModel):
    account_id: str
    tenant_id: str
    scopes: list[str] | None = None


class TokenResponse(BaseModel):
    access_token: str
    token_type: str = "bearer"
    expires_in: int
    tenant_id: str


settings = get_settings()


def _build_rate_limiter() -> SlidingWindowRateLimiter | RedisSlidingWindowRateLimiter:
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
    token, expires_in = service.issue_token(payload.account_id, payload.tenant_id, payload.scopes)
    return TokenResponse(
        access_token=token,
        expires_in=expires_in,
        tenant_id=payload.tenant_id,
    )
