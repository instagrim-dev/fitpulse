"""Domain-level request contracts shared by multiple layers."""

from __future__ import annotations

from dataclasses import dataclass


@dataclass(slots=True)
class CreateAccountInput:
    """Validated inputs required to create an account within a tenant."""

    tenant_id: str
    email: str
    disabled: bool = False
