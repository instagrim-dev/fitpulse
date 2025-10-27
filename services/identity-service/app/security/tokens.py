"""Utilities for issuing and validating application JWTs."""

from __future__ import annotations

import time
from typing import Any

import jwt

from ..config import get_settings


def issue_access_token(*, subject: str, tenant_id: str, scopes: list[str] | None = None) -> tuple[str, int]:
    """Create a signed JWT representing an authenticated account."""

    settings = get_settings()
    now = int(time.time())
    expires_in = settings.jwt_ttl_seconds
    default_scopes = scopes or ["activities:write", "activities:read", "ontology:read"]
    payload: dict[str, Any] = {
        "iss": settings.jwt_issuer,
        "sub": subject,
        "tenant_id": tenant_id,
        "scopes": default_scopes,
        "iat": now,
        "exp": now + expires_in,
    }

    token = jwt.encode(payload, settings.jwt_secret, algorithm="HS256")
    # PyJWT returns str for HS256 even in PyJWT>=2
    return token, expires_in


def decode_access_token(token: str) -> dict[str, Any]:
    """Decode and verify a JWT returning its payload."""

    settings = get_settings()
    return jwt.decode(
        token,
        settings.jwt_secret,
        algorithms=["HS256"],
        audience=None,
        issuer=settings.jwt_issuer,
    )
