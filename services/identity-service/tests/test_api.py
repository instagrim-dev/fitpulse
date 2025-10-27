from __future__ import annotations

import uuid
from datetime import datetime, timezone

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.api import routes
from app.domain.account import Account
from app.domain.contracts import CreateAccountInput
from app.domain.service import AccountService
from app.security.tokens import decode_access_token


class FakeRepository:
    """In-memory repository mimicking Postgres-backed behaviors."""

    def __init__(self) -> None:
        self._accounts: dict[tuple[str, str], Account] = {}
        self._idempotency: dict[tuple[str, str], str] = {}

    def create_account(self, payload: CreateAccountInput, idempotency_key: str | None):
        if idempotency_key:
            existing = self._idempotency.get((payload.tenant_id, idempotency_key))
            if existing:
                return self._accounts[(payload.tenant_id, existing)], True

        account_id = str(uuid.uuid4())
        account = Account(
            account_id=account_id,
            tenant_id=payload.tenant_id,
            email=payload.email,
            created_at=datetime.now(timezone.utc),
            disabled=payload.disabled,
        )
        self._accounts[(payload.tenant_id, account_id)] = account
        if idempotency_key:
            self._idempotency[(payload.tenant_id, idempotency_key)] = account_id
        return account, False

    def get_account(self, account_id: str, tenant_id: str):
        return self._accounts.get((tenant_id, account_id))


@pytest.fixture
def api_client():
    """Provide a FastAPI test client with isolated state."""
    repository = FakeRepository()
    service = AccountService(repository)

    app = FastAPI()
    app.include_router(routes.router)
    app.state.account_service = service

    original_limiter = routes.rate_limiter
    routes.rate_limiter = routes.SlidingWindowRateLimiter(max_requests=2, window_seconds=60)

    with TestClient(app) as client:
        yield client, service

    routes.rate_limiter = original_limiter


def test_issue_token_includes_default_scopes(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-1", email="user@example.com"),
        idempotency_key=None,
    )

    response = client.post(
        "/v1/token",
        json={
            "account_id": account.account_id,
            "tenant_id": account.tenant_id,
        },
    )
    assert response.status_code == 200
    data = response.json()
    assert data["tenant_id"] == account.tenant_id

    claims = decode_access_token(data["access_token"])
    assert claims["tenant_id"] == account.tenant_id
    assert set(claims["scopes"]) == {"activities:write", "activities:read", "ontology:read"}


def test_issue_token_applies_requested_scopes(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-2", email="scope@example.com"),
        idempotency_key=None,
    )

    response = client.post(
        "/v1/token",
        json={
            "account_id": account.account_id,
            "tenant_id": account.tenant_id,
            "scopes": ["activities:write", "ontology:admin"],
        },
    )
    assert response.status_code == 200
    claims = decode_access_token(response.json()["access_token"])
    assert claims["scopes"] == ["activities:write", "ontology:admin"]


def test_token_endpoint_respects_rate_limits(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-rl", email="limit@example.com"),
        idempotency_key=None,
    )

    payload = {"account_id": account.account_id, "tenant_id": account.tenant_id}

    first = client.post("/v1/token", json=payload)
    second = client.post("/v1/token", json=payload)
    third = client.post("/v1/token", json=payload)

    assert first.status_code == 200
    assert second.status_code == 200
    assert third.status_code == 429
    assert third.json()["detail"] == "rate limited"
