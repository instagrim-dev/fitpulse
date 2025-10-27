from __future__ import annotations

import time
from collections import deque
from threading import Lock
from typing import Deque, DefaultDict


class SlidingWindowRateLimiter:
    """Thread-safe sliding window rate limiter."""

    def __init__(self, max_requests: int, window_seconds: int) -> None:
        self._max_requests = max_requests
        self._window = window_seconds
        self._events: DefaultDict[str, Deque[float]] = DefaultDict(deque)
        self._lock = Lock()

    def allow(self, key: str) -> bool:
        now = time.time()
        with self._lock:
            queue = self._events[key]
            while queue and now - queue[0] > self._window:
                queue.popleft()
            if len(queue) >= self._max_requests:
                return False
            queue.append(now)
            return True
