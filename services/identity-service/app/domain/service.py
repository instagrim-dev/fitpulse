"""Account service orchestrating persistence and token issuance."""

from __future__ import annotations

from typing import Tuple

from .account import Account
from .contracts import CreateAccountInput
from ..repository import AccountRepository
from ..security.tokens import issue_access_token


class AccountService:
    """Account workflows backed by Postgres storage."""

    def __init__(self, repository: AccountRepository) -> None:
        """Store dependencies used to orchestrate persistence and token issuance."""
        self._repository = repository

    def create_account(
        self, payload: CreateAccountInput, idempotency_key: str | None
    ) -> Tuple[Account, bool]:
        """Create or replay an account record using repository idempotency semantics."""
        return self._repository.create_account(payload, idempotency_key)

    def get_account(self, account_id: str, tenant_id: str) -> Account | None:
        """Retrieve an account by identifier ensuring the tenant scope matches."""
        return self._repository.get_account(account_id, tenant_id)

    def issue_token(
        self, account_id: str, tenant_id: str, scopes: list[str] | None = None
    ) -> tuple[str, int]:
        """Issue a JWT for the given account or raise when the tenant/account mismatch."""
        account = self._repository.get_account(account_id, tenant_id)
        if account is None:
            raise ValueError("account not found")
        if account.tenant_id != tenant_id:
            raise ValueError("tenant mismatch")

        return issue_access_token(
            subject=account.account_id,
            tenant_id=tenant_id,
            scopes=scopes,
        )
