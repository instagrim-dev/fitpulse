"""Account-related DTOs shared across services."""

from __future__ import annotations

from datetime import datetime
from pydantic import BaseModel, EmailStr


class Account(BaseModel):
    account_id: str
    tenant_id: str
    email: EmailStr
    created_at: datetime
    disabled: bool = False
