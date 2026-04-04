"""Bounded event queue for the Promptmeter SDK."""

from __future__ import annotations

import logging
import queue
from typing import Any, Dict, List

logger = logging.getLogger("promptmeter")


class _EventQueue:
    """Thread-safe bounded queue for LLM events.

    When the queue is full, new events are dropped (not old ones).
    This ensures the queue never blocks the caller.
    """

    def __init__(self, max_size: int = 10000) -> None:
        self._queue: queue.Queue[Dict[str, Any]] = queue.Queue(maxsize=max_size)
        self._max_size = max_size

    def put(self, event: Dict[str, Any]) -> bool:
        """Non-blocking put. Returns False if queue is full (event dropped)."""
        try:
            self._queue.put_nowait(event)
            return True
        except queue.Full:
            logger.warning(
                "promptmeter: queue full (%d events), dropping event. "
                "Consider increasing max_queue_size or check network connectivity.",
                self._max_size,
            )
            return False

    def drain(self, max_items: int) -> List[Dict[str, Any]]:
        """Non-blocking drain up to max_items from the queue."""
        items: List[Dict[str, Any]] = []
        while len(items) < max_items:
            try:
                items.append(self._queue.get_nowait())
            except queue.Empty:
                break
        return items

    @property
    def size(self) -> int:
        """Return the approximate number of items in the queue."""
        return self._queue.qsize()

    @property
    def empty(self) -> bool:
        """Return True if the queue is empty."""
        return self._queue.empty()
