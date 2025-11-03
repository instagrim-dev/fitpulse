"""Database repository for identity/account data."""

from __future__ import annotations

import hashlib
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Any, Optional, Tuple

from psycopg.rows import tuple_row
from psycopg_pool import ConnectionPool
from psycopg.types.json import Json

from .domain.account import Account
from .domain.contracts import CreateAccountInput


@dataclass(slots=True)
class AccountRecord:
    """Row projection used when mapping database tuples to domain aggregates."""

    account_id: str
    tenant_id: str
    email: str
    created_at: datetime
    disabled: bool


@dataclass(slots=True)
class RefreshTokenRecord:
    """DTO mapping the refresh_tokens table for repository consumers."""

    token_id: str
    account_id: str
    tenant_id: str
    expires_at: datetime
    revoked_at: datetime | None


@dataclass(slots=True)
class AuditLogRecord:
    """Row projection for items in identity_audit_log."""

    audit_id: int
    account_id: str | None
    tenant_id: str | None
    event_type: str
    actor: str | None
    metadata: dict[str, Any]
    created_at: datetime


class AccountRepository:
    """Postgres-backed account persistence with idempotency support."""

    def __init__(self, pool: ConnectionPool) -> None:
        """Store the connection pool used for all database interactions."""
        self._pool = pool

    def _hash_email(self, email: str) -> bytes:
        """Normalise an email address and return its SHA-256 digest."""
        return hashlib.sha256(email.lower().encode("utf-8")).digest()

    def create_account(
        self,
        payload: CreateAccountInput,
        idempotency_key: str | None,
    ) -> Tuple[Account, bool]:
        """Persist an account record and return a tuple of (account, replay flag)."""
        with self._pool.connection() as conn:
            with conn.cursor(row_factory=tuple_row) as cur:
                cur.execute("SELECT set_config('app.tenant_id', %s, true)", (payload.tenant_id,))

                if idempotency_key:
                    cur.execute(
                        """
                        SELECT account_id
                        FROM account_idempotency
                        WHERE tenant_id = %s AND idempotency_key = %s
                        """,
                        (payload.tenant_id, idempotency_key),
                    )
                    row = cur.fetchone()
                    if row:
                        cur.execute(
                            """
                            SELECT account_id, tenant_id, email_cipher, created_at, disabled
                            FROM accounts
                            WHERE account_id = %s AND tenant_id = %s
                            """,
                            (row[0], payload.tenant_id),
                        )
                        account_row = cur.fetchone()
                        return self._map_record(account_row), True

                account_id = str(uuid.uuid4())
                now = datetime.now(timezone.utc)
                email_hash = self._hash_email(payload.email)

                cur.execute(
                    """
                    INSERT INTO accounts (account_id, tenant_id, email_hash, email_cipher, disabled, created_at, updated_at)
                    VALUES (%s, %s, %s, %s, %s, %s, %s)
                    RETURNING account_id, tenant_id, email_cipher, created_at, disabled
                    """,
                    (
                        account_id,
                        payload.tenant_id,
                        email_hash,
                        payload.email,
                        payload.disabled,
                        now,
                        now,
                    ),
                )
                record = cur.fetchone()

                if idempotency_key:
                    cur.execute(
                        """
                        INSERT INTO account_idempotency (tenant_id, idempotency_key, account_id, created_at)
                        VALUES (%s, %s, %s, %s)
                        ON CONFLICT (tenant_id, idempotency_key) DO NOTHING
                        """,
                        (payload.tenant_id, idempotency_key, account_id, now),
                    )

                conn.commit()

        return self._map_record(record), False

    def get_account(self, account_id: str, tenant_id: str) -> Account | None:
        """Fetch an account belonging to the specified tenant or return ``None``."""
        with self._pool.connection() as conn:
            with conn.cursor(row_factory=tuple_row) as cur:
                cur.execute("SELECT set_config('app.tenant_id', %s, true)", (tenant_id,))
                cur.execute(
                    """
                    SELECT account_id, tenant_id, email_cipher, created_at, disabled
                    FROM accounts
                    WHERE account_id = %s AND tenant_id = %s
                    """,
                    (account_id, tenant_id),
                )
                row = cur.fetchone()
                if not row:
                    return None
        return self._map_record(row)

    def _map_record(self, row: tuple) -> Account:
        """Convert a raw database tuple into the domain ``Account`` dataclass."""
        return Account(
            account_id=row[0],
            tenant_id=row[1],
            email=row[2],
            created_at=row[3],
            disabled=row[4],
        )

    def create_refresh_token(
        self,
        *,
        account_id: str,
        tenant_id: str,
        token_hash: str,
        expires_at: datetime,
        metadata: dict[str, Any] | None = None,
    ) -> RefreshTokenRecord:
        """Persist a hashed refresh token associated with an account."""
        token_id = str(uuid.uuid4())
        with self._pool.connection() as conn:
            with conn.cursor(row_factory=tuple_row) as cur:
                cur.execute(
                    """
                    INSERT INTO refresh_tokens (token_id, account_id, tenant_id, token_hash, expires_at, metadata)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    RETURNING token_id, account_id, tenant_id, expires_at, revoked_at
                    """,
                    (token_id, account_id, tenant_id, token_hash, expires_at, Json(metadata or {})),
                )
                row = cur.fetchone()
                conn.commit()
        return RefreshTokenRecord(*row)

    def find_refresh_token(self, token_hash: str) -> RefreshTokenRecord | None:
        """Return an active refresh token record for the provided hash."""
        with self._pool.connection() as conn:
            with conn.cursor(row_factory=tuple_row) as cur:
                cur.execute(
                    """
                    SELECT token_id, account_id, tenant_id, expires_at, revoked_at
                    FROM refresh_tokens
                    WHERE token_hash = %s AND revoked_at IS NULL
                    """,
                    (token_hash,),
                )
                row = cur.fetchone()
                if not row:
                    return None
        return RefreshTokenRecord(*row)

    def revoke_refresh_token(self, token_id: str) -> None:
        """Mark the given refresh token as revoked."""
        with self._pool.connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE refresh_tokens
                    SET revoked_at = NOW()
                    WHERE token_id = %s AND revoked_at IS NULL
                    """,
                    (token_id,),
                )
                conn.commit()

    def write_audit_event(
        self,
        *,
        account_id: str | None,
        tenant_id: str | None,
        event_type: str,
        actor: str | None,
        metadata: dict[str, Any] | None = None,
    ) -> None:
        """Record an audit trail entry capturing identity workflow activity."""
        with self._pool.connection() as conn:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO identity_audit_log (account_id, tenant_id, event_type, actor, metadata)
                    VALUES (%s, %s, %s, %s, %s)
                    """,
                    (account_id, tenant_id, event_type, actor, Json(metadata or {})),
                )
                conn.commit()

    def list_audit_events(
        self,
        *,
        tenant_id: str,
        account_id: str | None = None,
        event_type: str | None = None,
        created_after: datetime | None = None,
        created_before: datetime | None = None,
        limit: int = 50,
        cursor: Tuple[datetime, int] | None = None,
    ) -> tuple[list[AuditLogRecord], Optional[Tuple[datetime, int]]]:
        """Return audit log entries scoped to a tenant with optional filters and cursor pagination."""
        limit = max(1, min(limit, 100))
        clauses = ["tenant_id = %s"]
        params: list[Any] = [tenant_id]

        if account_id:
            clauses.append("account_id = %s")
            params.append(account_id)
        if event_type:
            clauses.append("event_type = %s")
            params.append(event_type)
        if created_after:
            clauses.append("created_at >= %s")
            params.append(created_after)
        if created_before:
            clauses.append("created_at <= %s")
            params.append(created_before)
        if cursor:
            clauses.append("(created_at, audit_id) < (%s, %s)")
            params.extend(cursor)

        where_sql = " AND ".join(clauses)
        query = f"""
            SELECT audit_id, account_id, tenant_id, event_type, actor, metadata, created_at
            FROM identity_audit_log
            WHERE {where_sql}
            ORDER BY created_at DESC, audit_id DESC
            LIMIT %s
        """
        params.append(limit)

        records: list[AuditLogRecord] = []
        with self._pool.connection() as conn:
            with conn.cursor(row_factory=tuple_row) as cur:
                cur.execute(query, params)
                for row in cur.fetchall():
                    records.append(
                        AuditLogRecord(
                            audit_id=row[0],
                            account_id=row[1],
                            tenant_id=row[2],
                            event_type=row[3],
                            actor=row[4],
                            metadata=row[5] or {},
                            created_at=row[6],
                        )
                    )

        next_cursor: Tuple[datetime, int] | None = None
        if len(records) == limit:
            last = records[-1]
            next_cursor = (last.created_at, last.audit_id)
        return records, next_cursor
