"""Redis-backed sliding window rate limiter."""

from __future__ import annotations

import time
from typing import Final

from redis import Redis
from redis.exceptions import ResponseError


class RedisSlidingWindowRateLimiter:
    """Distributed sliding window limiter implemented with Redis sorted sets."""

    _LUA_SCRIPT: Final[str] = """
    local key = KEYS[1]
    local counter_key = key .. ':seq'
    local window_ms = tonumber(ARGV[1])
    local max_requests = tonumber(ARGV[2])
    local now_ms = tonumber(ARGV[3])

    redis.call('ZREMRANGEBYSCORE', key, 0, now_ms - window_ms)
    local current = redis.call('ZCARD', key)
    if current >= max_requests then
        return 0
    end
    local seq = redis.call('INCR', counter_key)
    redis.call('PEXPIRE', counter_key, window_ms)
    local member = tostring(now_ms) .. ':' .. tostring(seq)
    redis.call('ZADD', key, now_ms, member)
    redis.call('PEXPIRE', key, window_ms)
    return 1
    """

    def __init__(
        self,
        client: Redis,
        *,
        max_requests: int,
        window_seconds: int,
        key_prefix: str = "rate"
    ) -> None:
        """Initialise the Redis client, window configuration, and Lua script cache."""
        self._client = client
        self._max_requests = max_requests
        self._window_ms = window_seconds * 1000
        self._key_prefix = key_prefix
        self._script = client.register_script(self._LUA_SCRIPT)

    def allow(self, key: str) -> bool:
        """Return ``True`` when the key is still within the distributed rate limit."""
        now_ms = int(time.time() * 1000)
        redis_key = f"{self._key_prefix}:{key}"
        try:
            result = self._script(keys=[redis_key], args=[self._window_ms, self._max_requests, now_ms])
            return int(result) == 1
        except ResponseError as exc:
            message = str(exc).lower()
            if "unknown command `evalsha`" in message or "unknown command `eval`" in message:
                return self._allow_fallback(redis_key, now_ms)
            raise

    def _allow_fallback(self, redis_key: str, now_ms: int) -> bool:
        """Fallback pure-Python implementation used when Lua is unavailable."""
        window_start = now_ms - self._window_ms
        self._client.zremrangebyscore(redis_key, 0, window_start)
        current = self._client.zcard(redis_key)
        if current >= self._max_requests:
            return False
        seq = self._client.incr(f"{redis_key}:seq")
        self._client.pexpire(f"{redis_key}:seq", self._window_ms)
        member = f"{now_ms}:{seq}"
        self._client.zadd(redis_key, {member: now_ms})
        self._client.pexpire(redis_key, self._window_ms)
        return True
