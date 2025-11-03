"""FastAPI application wiring for the identity service."""

from __future__ import annotations

from contextlib import asynccontextmanager

from fastapi import FastAPI, Response
from fastapi.middleware.cors import CORSMiddleware
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

# CORS for local frontend dev
app.add_middleware(
    CORSMiddleware,
    allow_origins=["http://localhost:5173", "http://127.0.0.1:5173"],
    allow_origin_regex=".*",  # dev: allow any origin
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
    max_age=600,
)


@app.get("/healthz", tags=["health"])
def healthz() -> dict[str, str]:
    """Return a minimal readiness indicator used by orchestration systems."""
    return {"status": "ok"}


app.include_router(v1_router)


# Prometheus metrics endpoint for Prometheus scrapes
try:
    from prometheus_client import CONTENT_TYPE_LATEST, generate_latest

    @app.get("/metrics")
    def metrics() -> Response:
        return Response(content=generate_latest(), media_type=CONTENT_TYPE_LATEST)
except Exception:  # pragma: no cover - metrics are optional in dev
    pass
