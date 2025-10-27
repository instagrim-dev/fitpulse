"""Database repository for identity/account data."""

from __future__ import annotations

import hashlib
import uuid
from dataclasses import dataclass
from datetime import datetime, timezone
from typing import Tuple

from psycopg.rows import tuple_row
from psycopg_pool import ConnectionPool

from .domain.account import Account
from .domain.contracts import CreateAccountInput


@dataclass(slots=True)
class AccountRecord:
    account_id: str
    tenant_id: str
    email: str
    created_at: datetime
    disabled: bool


class AccountRepository:
    """Postgres-backed account persistence with idempotency support."""

    def __init__(self, pool: ConnectionPool) -> None:
        self._pool = pool

    def _hash_email(self, email: str) -> bytes:
        return hashlib.sha256(email.lower().encode("utf-8")).digest()

    def create_account(
        self,
        payload: CreateAccountInput,
        idempotency_key: str | None,
    ) -> Tuple[Account, bool]:
        """Persist an account record and return (account, replay)."""
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
        """Fetch an account by identifier or return None."""
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
        return Account(
            account_id=row[0],
            tenant_id=row[1],
            email=row[2],
            created_at=row[3],
            disabled=row[4],
        )
