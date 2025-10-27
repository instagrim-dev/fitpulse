"""Tests for the Redis-backed sliding window rate limiter."""

from __future__ import annotations

import time

import fakeredis
import pytest

from app.security.redis_rate_limiter import RedisSlidingWindowRateLimiter


@pytest.fixture()
def redis_client() -> fakeredis.FakeStrictRedis:
    client = fakeredis.FakeStrictRedis()
    client.flushall()
    return client


def test_redis_rate_limiter_allows_within_threshold(redis_client):
    limiter = RedisSlidingWindowRateLimiter(
        redis_client, max_requests=3, window_seconds=1, key_prefix="test"
    )
    key = "tenant:account"
    assert limiter.allow(key)
    assert limiter.allow(key)
    assert limiter.allow(key)


def test_redis_rate_limiter_blocks_excess(redis_client):
    limiter = RedisSlidingWindowRateLimiter(
        redis_client, max_requests=2, window_seconds=1, key_prefix="test"
    )
    key = "tenant:account"
    assert limiter.allow(key)
    assert limiter.allow(key)
    assert not limiter.allow(key)


def test_redis_rate_limiter_expires_entries(redis_client):
    limiter = RedisSlidingWindowRateLimiter(
        redis_client, max_requests=1, window_seconds=1, key_prefix="test"
    )
    key = "tenant:account"
    assert limiter.allow(key)
    assert not limiter.allow(key)
    time.sleep(1.1)
    assert limiter.allow(key)
