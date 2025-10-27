"""FastAPI application wiring for the identity service."""

from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI
from psycopg_pool import ConnectionPool

from .api.routes import router as v1_router
from .config import get_settings
from .domain.service import AccountService
from .repository import AccountRepository

settings = get_settings()


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Initialise shared resources (Postgres pool, services) for the app lifecycle."""
    pool = ConnectionPool(settings.database_url, open=False)
    pool.open()
    app.state.pool = pool
    app.state.account_service = AccountService(AccountRepository(pool))
    try:
        yield
    finally:
        pool.close()
        pool.wait_close()


app = FastAPI(title=settings.app_name, version=settings.version, lifespan=lifespan)


@app.get("/healthz", tags=["health"])
def healthz() -> dict[str, str]:
    """Return a minimal readiness indicator used by orchestration systems."""
    return {"status": "ok"}


app.include_router(v1_router)
