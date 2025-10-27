"""Exercise ontology event contracts."""

from __future__ import annotations

from datetime import datetime
from pydantic import BaseModel, Field


class ExerciseUpserted(BaseModel):
    exercise_id: str
    name: str
    difficulty: str | None = None
    targets: list[str] = Field(default_factory=list)
    requires: list[str] = Field(default_factory=list)
    contraindications: list[str] = Field(default_factory=list)
    complementary_to: list[str] = Field(default_factory=list)
    updated_at: datetime
    tenant_id: str | None = None


class ExerciseDeleted(BaseModel):
    exercise_id: str
    deleted_at: datetime
