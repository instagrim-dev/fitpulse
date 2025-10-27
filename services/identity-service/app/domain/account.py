from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime


@dataclass(slots=True)
class Account:
    """Aggregate root for tenant-scoped user identity."""

    account_id: str
    tenant_id: str
    email: str
    created_at: datetime
    disabled: bool = False
