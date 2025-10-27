"""Shared Pydantic models for activity domain events."""

from __future__ import annotations

from datetime import datetime
from enum import Enum
from pydantic import BaseModel, Field


class ActivityState(str, Enum):
    pending = "pending"
    synced = "synced"
    failed = "failed"


class ActivityCreated(BaseModel):
    activity_id: str = Field(..., alias="activity_id")
    tenant_id: str
    user_id: str
    activity_type: str
    started_at: datetime
    duration_min: int
    source: str
    version: str = "v1"


class ActivityStateChanged(BaseModel):
    activity_id: str
    tenant_id: str
    user_id: str
    state: ActivityState
    occurred_at: datetime
    reason: str | None = None

    class Config:
        populate_by_name = True
        use_enum_values = True
