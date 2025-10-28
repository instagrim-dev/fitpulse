"""Utilities for issuing and validating application JWTs."""

from __future__ import annotations

import hashlib
import secrets
import time
from typing import Any

import jwt

from ..config import get_settings


def issue_access_token(*, subject: str, tenant_id: str, scopes: list[str] | None = None) -> tuple[str, int]:
    """Create a signed JWT representing an authenticated account.

    Parameters
    ----------
    subject:
        Account identifier to embed in the token `sub` claim.
    tenant_id:
        Tenant context assigned to the token for downstream RLS checks.
    scopes:
        Optional scope list; defaults to the identity service's standard read/write scopes.

    Returns
    -------
    tuple[str, int]
        A tuple containing the encoded JWT string and its TTL (in seconds).
    """

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
    """Decode and verify a JWT returning its payload.

    Parameters
    ----------
    token:
        Encoded JWT issued by this service.

    Returns
    -------
    dict[str, Any]
        The decoded payload if signature and issuer checks succeed.

    Raises
    ------
    jwt.PyJWTError
        Propagated when the token is invalid, expired, or signed by another issuer.
    """

    settings = get_settings()
    return jwt.decode(
        token,
        settings.jwt_secret,
        algorithms=["HS256"],
        audience=None,
        issuer=settings.jwt_issuer,
    )


def generate_refresh_token() -> tuple[str, str]:
    """Generate a refresh token string and its SHA-256 hash."""
    token = secrets.token_urlsafe(48)
    return token, hash_refresh_token(token)


def hash_refresh_token(token: str) -> str:
    """Return the SHA-256 hex digest for a refresh token string."""
    return hashlib.sha256(token.encode("utf-8")).hexdigest()
