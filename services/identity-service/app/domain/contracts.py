"""Domain-level request contracts shared by multiple layers."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(slots=True)
class CreateAccountInput:
    tenant_id: str
    email: str
    disabled: bool = False
