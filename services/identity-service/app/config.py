from __future__ import annotations

from dataclasses import dataclass
from functools import lru_cache
import os


@dataclass(frozen=True)
class Settings:
    """Runtime configuration values exposed to FastAPI components."""

    app_name: str = "identity-service"
    version: str = "0.1.0"
    database_url: str = os.getenv(
        "POSTGRES_URL",
        "postgres://platform:platform@postgres:5432/fitness",
    )
    http_host: str = os.getenv("HTTP_HOST", "0.0.0.0")
    http_port: int = int(os.getenv("HTTP_PORT", "8000"))
    jwt_secret: str = os.getenv("JWT_SECRET", "dev-secret-change-me")
    jwt_issuer: str = os.getenv("JWT_ISSUER", "i5e.identity")
    jwt_ttl_seconds: int = int(os.getenv("JWT_TTL_SECONDS", "3600"))
    rate_limit_requests: int = int(os.getenv("RATE_LIMIT_REQUESTS", "20"))
    rate_limit_window_seconds: int = int(os.getenv("RATE_LIMIT_WINDOW_SECONDS", "60"))
    rate_limit_backend: str = os.getenv("RATE_LIMIT_BACKEND", "memory").lower()
    redis_url: str = os.getenv("REDIS_URL", "")


@lru_cache(maxsize=1)
def get_settings() -> Settings:
    """Return the cached Settings instance for the running process."""
    return Settings()
