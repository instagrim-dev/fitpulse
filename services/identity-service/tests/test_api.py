from __future__ import annotations

import uuid
from dataclasses import dataclass
from datetime import datetime, timedelta, timezone

import pytest
from fastapi import FastAPI
from fastapi.testclient import TestClient

from app.api import routes
from app.config import get_settings
from app.domain.account import Account
from app.domain.contracts import CreateAccountInput
from app.domain.service import AccountService
from app.security.tokens import decode_access_token


class FakeRepository:
    """In-memory repository mimicking Postgres-backed behaviors."""

    def __init__(self) -> None:
        self._accounts: dict[tuple[str, str], Account] = {}
        self._idempotency: dict[tuple[str, str], str] = {}
        self._refresh_tokens: dict[str, FakeRefreshToken] = {}
        self.audit_log: list[FakeAuditLogRecord] = []
        self._audit_seq = 0

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

    def create_refresh_token(
        self,
        *,
        account_id: str,
        tenant_id: str,
        token_hash: str,
        expires_at: datetime,
        metadata: dict | None = None,
    ):
        token = FakeRefreshToken(
            token_id=str(uuid.uuid4()),
            account_id=account_id,
            tenant_id=tenant_id,
            expires_at=expires_at,
            revoked_at=None,
        )
        self._refresh_tokens[token_hash] = token
        return token

    def find_refresh_token(self, token_hash: str):
        record = self._refresh_tokens.get(token_hash)
        if record and record.revoked_at is None:
            return record
        return None

    def revoke_refresh_token(self, token_id: str) -> None:
        for stored_hash, record in list(self._refresh_tokens.items()):
            if record.token_id == token_id:
                record.revoked_at = datetime.now(timezone.utc)
                break

    def write_audit_event(
        self,
        *,
        account_id: str | None,
        tenant_id: str | None,
        event_type: str,
        actor: str | None,
        metadata: dict | None = None,
    ) -> None:
        self._audit_seq += 1
        self.audit_log.append(
            FakeAuditLogRecord(
                audit_id=self._audit_seq,
                account_id=account_id,
                tenant_id=tenant_id,
                event_type=event_type,
                actor=actor,
                metadata=metadata or {},
                created_at=datetime.now(timezone.utc),
            )
        )

    def list_audit_events(
        self,
        *,
        tenant_id: str,
        account_id: str | None = None,
        event_type: str | None = None,
        created_after: datetime | None = None,
        created_before: datetime | None = None,
        limit: int = 50,
        cursor: tuple[datetime, int] | None = None,
    ):
        results = [record for record in self.audit_log if record.tenant_id == tenant_id]
        if account_id:
            results = [record for record in results if record.account_id == account_id]
        if event_type:
            results = [record for record in results if record.event_type == event_type]
        if created_after:
            results = [record for record in results if record.created_at >= created_after]
        if created_before:
            results = [record for record in results if record.created_at <= created_before]
        results.sort(key=lambda r: (r.created_at, r.audit_id), reverse=True)
        if cursor:
            filtered = []
            for record in results:
                if (record.created_at, record.audit_id) < cursor:
                    filtered.append(record)
            results = filtered
        slice_ = results[:limit]
        next_cursor = None
        if len(results) > limit:
            last = slice_[-1]
            next_cursor = (last.created_at, last.audit_id)
        return slice_, next_cursor


@dataclass
class FakeRefreshToken:
    token_id: str
    account_id: str
    tenant_id: str
    expires_at: datetime
    revoked_at: datetime | None


@dataclass
class FakeAuditLogRecord:
    audit_id: int
    account_id: str | None
    tenant_id: str | None
    event_type: str
    actor: str | None
    metadata: dict
    created_at: datetime


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
    assert data["refresh_token"]
    assert data["refresh_expires_in"] == get_settings().refresh_ttl_seconds

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


def test_refresh_token_flow(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-refresh", email="refresh@example.com"),
        idempotency_key=None,
    )

    issued = client.post(
        "/v1/token",
        json={"account_id": account.account_id, "tenant_id": account.tenant_id},
    ).json()

    refresh_response = client.post(
        "/v1/token/refresh", json={"refresh_token": issued["refresh_token"]}
    )
    assert refresh_response.status_code == 200


def test_audit_log_endpoint_returns_paginated_entries(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-audit", email="audit@example.com"),
        idempotency_key=None,
    )

    repo: FakeRepository = service._repository  # type: ignore[attr-defined]
    # Seed extra audit events
    for idx in range(5):
        repo.write_audit_event(
            account_id=account.account_id,
            tenant_id=account.tenant_id,
            event_type="custom.event",
            actor=f"actor-{idx}",
            metadata={"sequence": idx},
        )

    resp = client.get(
        "/v1/audit/logs",
        params={"limit": 3},
        headers={"X-Tenant-ID": account.tenant_id},
    )
    assert resp.status_code == 200
    body = resp.json()
    assert len(body["items"]) == 3
    assert body["next_cursor"]
    for entry in body["items"]:
        assert entry["tenant_id"] == account.tenant_id

    next_resp = client.get(
        "/v1/audit/logs",
        params={"cursor": body["next_cursor"], "limit": 3},
        headers={"X-Tenant-ID": account.tenant_id},
    )
    assert next_resp.status_code == 200
    next_body = next_resp.json()
    # remaining events including earlier ones + implicit account created entries
    assert len(next_body["items"]) >= 1

    filter_resp = client.get(
        "/v1/audit/logs",
        params={"account_id": "non-existent"},
        headers={"X-Tenant-ID": account.tenant_id},
    )
    assert filter_resp.status_code == 200
    assert filter_resp.json()["items"] == []


def test_audit_log_endpoint_rejects_bad_cursor(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-cursor", email="cursor@example.com"),
        idempotency_key=None,
    )

    resp = client.get(
        "/v1/audit/logs",
        params={"cursor": "not-valid"},
        headers={"X-Tenant-ID": account.tenant_id},
    )
    assert resp.status_code == 400


def test_refresh_token_rejects_expired(api_client):
    client, service = api_client
    account, _ = service.create_account(
        CreateAccountInput(tenant_id="tenant-expired", email="expired@example.com"),
        idempotency_key=None,
    )
    token = client.post(
        "/v1/token", json={"account_id": account.account_id, "tenant_id": account.tenant_id}
    ).json()
    # expire the token manually
    for record in service._repository._refresh_tokens.values():
        record.expires_at = datetime.now(timezone.utc) - timedelta(minutes=1)
    resp = client.post("/v1/token/refresh", json={"refresh_token": token["refresh_token"]})
    assert resp.status_code == 400
    assert "expired" in resp.json()["detail"]
