"""Account service orchestrating persistence, token issuance, and auditing."""

from __future__ import annotations

from base64 import urlsafe_b64decode, urlsafe_b64encode
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone
import json
from typing import Tuple, Optional

from .account import Account
from .contracts import CreateAccountInput
from ..config import get_settings
from ..repository import AccountRepository, AuditLogRecord
from ..security.tokens import generate_refresh_token, hash_refresh_token, issue_access_token


@dataclass(slots=True)
class TokenBundle:
    """Encapsulates the access/refresh token pair returned to API consumers."""

    access_token: str
    access_expires_in: int
    refresh_token: str
    refresh_expires_in: int
    tenant_id: str


class AccountService:
    """Account workflows backed by Postgres storage."""

    def __init__(self, repository: AccountRepository) -> None:
        """Store dependencies used to orchestrate persistence and token issuance."""
        self._repository = repository

    def create_account(
        self, payload: CreateAccountInput, idempotency_key: str | None
    ) -> Tuple[Account, bool]:
        """Create or replay an account record using repository idempotency semantics."""
        account, replay = self._repository.create_account(payload, idempotency_key)
        if not replay:
            self._repository.write_audit_event(
                account_id=account.account_id,
                tenant_id=account.tenant_id,
                event_type="account.created",
                actor=account.account_id,
                metadata={"email": account.email},
            )
        else:
            self._repository.write_audit_event(
                account_id=account.account_id,
                tenant_id=account.tenant_id,
                event_type="account.replayed",
                actor=account.account_id,
                metadata={},
            )
        return account, replay

    def get_account(self, account_id: str, tenant_id: str) -> Account | None:
        """Retrieve an account by identifier ensuring the tenant scope matches."""
        return self._repository.get_account(account_id, tenant_id)

    def issue_token(
        self, account_id: str, tenant_id: str, scopes: list[str] | None = None
    ) -> TokenBundle:
        """Issue access and refresh tokens for the given account."""
        account = self._repository.get_account(account_id, tenant_id)
        if account is None:
            raise ValueError("account not found")

        effective_scopes = scopes or ["activities:write", "activities:read", "ontology:read", "ontology:write"]
        if "ontology:write" not in effective_scopes:
            effective_scopes = list({*effective_scopes, "ontology:write"})
        access_token, expires_in = issue_access_token(
            subject=account.account_id,
            tenant_id=tenant_id,
            scopes=effective_scopes,
        )

        settings = get_settings()
        refresh_token, token_hash = generate_refresh_token()
        refresh_expires = datetime.now(timezone.utc) + timedelta(seconds=settings.refresh_ttl_seconds)
        self._repository.create_refresh_token(
            account_id=account.account_id,
            tenant_id=tenant_id,
            token_hash=token_hash,
            expires_at=refresh_expires,
            metadata={"scopes": effective_scopes},
        )

        self._repository.write_audit_event(
            account_id=account.account_id,
            tenant_id=tenant_id,
            event_type="token.issued",
            actor=account.account_id,
            metadata={"scopes": effective_scopes},
        )

        return TokenBundle(
            access_token=access_token,
            access_expires_in=expires_in,
            refresh_token=refresh_token,
            refresh_expires_in=settings.refresh_ttl_seconds,
            tenant_id=tenant_id,
        )

    def refresh_access_token(
        self, refresh_token: str, scopes: list[str] | None = None
    ) -> TokenBundle:
        """Exchange a refresh token for a new access/refresh pair.

        Parameters
        ----------
        refresh_token:
            Raw refresh token obtained from a previous call to :meth:`issue_token`.
        scopes:
            Optional overrides for the scope list baked into the newly minted access token.
        """
        token_hash = hash_refresh_token(refresh_token)
        record = self._repository.find_refresh_token(token_hash)
        if record is None:
            raise ValueError("invalid refresh token")
        if record.revoked_at is not None:
            raise ValueError("refresh token revoked")
        if record.expires_at <= datetime.now(timezone.utc):
            self._repository.revoke_refresh_token(record.token_id)
            raise ValueError("refresh token expired")

        account = self._repository.get_account(record.account_id, record.tenant_id)
        if account is None or account.disabled:
            self._repository.revoke_refresh_token(record.token_id)
            raise ValueError("account unavailable")

        self._repository.revoke_refresh_token(record.token_id)

        new_bundle = self.issue_token(account.account_id, record.tenant_id, scopes)
        self._repository.write_audit_event(
            account_id=record.account_id,
            tenant_id=record.tenant_id,
            event_type="token.refreshed",
            actor=record.account_id,
            metadata={
                "scopes": scopes or [],
                "previous_token_id": record.token_id,
            },
        )
        return new_bundle

    def list_audit_events(
        self,
        *,
        tenant_id: str,
        account_id: str | None = None,
        event_type: str | None = None,
        created_after: datetime | None = None,
        created_before: datetime | None = None,
        limit: int = 50,
        cursor: str | None = None,
    ) -> tuple[list[AuditLogRecord], str | None]:
        """Return audit log records for the tenant with optional filters and cursor pagination."""
        decoded_cursor: Optional[Tuple[datetime, int]] = None
        if cursor:
            decoded_cursor = self._decode_cursor(cursor)
        records, next_cursor_tuple = self._repository.list_audit_events(
            tenant_id=tenant_id,
            account_id=account_id,
            event_type=event_type,
            created_after=created_after,
            created_before=created_before,
            limit=limit,
            cursor=decoded_cursor,
        )
        next_cursor = self._encode_cursor(next_cursor_tuple) if next_cursor_tuple else None
        return records, next_cursor

    def _encode_cursor(self, cursor: Tuple[datetime, int] | None) -> str | None:
        if cursor is None:
            return None
        created_at, audit_id = cursor
        payload = json.dumps({"created_at": created_at.isoformat(), "audit_id": audit_id})
        return urlsafe_b64encode(payload.encode("utf-8")).decode("utf-8")

    def _decode_cursor(self, cursor: str) -> Tuple[datetime, int]:
        try:
            data = json.loads(urlsafe_b64decode(cursor.encode("utf-8")).decode("utf-8"))
            created_at = datetime.fromisoformat(data["created_at"])
            audit_id = int(data["audit_id"])
            return created_at, audit_id
        except Exception as exc:
            raise ValueError("invalid cursor") from exc
