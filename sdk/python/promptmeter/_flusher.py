"""Background flush worker for the Promptmeter SDK."""

from __future__ import annotations

import logging
import threading
import time
from typing import Any, Dict, List, Optional

import httpx

from ._queue import _EventQueue
from ._version import __version__

logger = logging.getLogger("promptmeter")


class _FlushWorker:
    """Daemon thread that periodically flushes events to the Ingestion API.

    Implements retry with exponential backoff for 5xx and 429 responses.
    On 401, disables the SDK permanently (bad API key).
    On 400, drops the batch (invalid data, won't be fixed by retrying).
    """

    def __init__(
        self,
        queue: _EventQueue,
        api_key: str,
        endpoint: str,
        batch_size: int = 50,
        flush_interval: float = 5.0,
        flush_timeout: float = 5.0,
        max_retries: int = 3,
        debug: bool = False,
    ) -> None:
        self._queue = queue
        self._api_key = api_key
        self._endpoint = endpoint.rstrip("/")
        self._batch_size = batch_size
        self._flush_interval = flush_interval
        self._flush_timeout = flush_timeout
        self._max_retries = max_retries
        self._debug = debug

        self._stop_event = threading.Event()
        self._auth_failed = False
        self._thread: Optional[threading.Thread] = None

        self._http_client = httpx.Client(
            timeout=10.0,
            headers={
                "Authorization": f"Bearer {self._api_key}",
                "Content-Type": "application/json",
                "User-Agent": f"promptmeter-python/{__version__}",
            },
        )

    def start(self) -> None:
        """Start the background flush thread."""
        self._thread = threading.Thread(target=self._run, daemon=True, name="promptmeter-flusher")
        self._thread.start()

    def stop(self, timeout: Optional[float] = None) -> None:
        """Stop the flush thread and perform a final flush."""
        self._stop_event.set()
        if self._thread is not None:
            self._thread.join(timeout=timeout or self._flush_timeout)

        # Final flush
        self._flush()

        try:
            self._http_client.close()
        except Exception:
            pass

    def _run(self) -> None:
        """Main loop: sleep for flush_interval, then flush."""
        while not self._stop_event.is_set():
            self._stop_event.wait(timeout=self._flush_interval)
            self._flush()

    def _flush(self) -> None:
        """Drain the queue and send batches."""
        if self._auth_failed:
            return

        while True:
            events = self._queue.drain(max_items=self._batch_size)
            if not events:
                break
            self._send_batch(events)

    def _send_batch(self, events: List[Dict[str, Any]]) -> None:
        """Send a batch of events with retry logic."""
        if self._auth_failed:
            return

        for attempt in range(self._max_retries + 1):
            try:
                if self._debug:
                    logger.debug(
                        "promptmeter: sending batch of %d events (attempt %d)",
                        len(events),
                        attempt + 1,
                    )

                response = self._http_client.post(
                    f"{self._endpoint}/v1/events/batch",
                    json={"events": events},
                )

                if response.status_code == 202:
                    if self._debug:
                        logger.debug("promptmeter: batch accepted (%d events)", len(events))
                    return

                if response.status_code == 401:
                    logger.error(
                        "promptmeter: invalid API key. SDK disabled. "
                        "No further events will be sent."
                    )
                    self._auth_failed = True
                    return

                if response.status_code == 400:
                    logger.warning(
                        "promptmeter: batch rejected (400). Dropping %d events.",
                        len(events),
                    )
                    return

                if response.status_code == 429:
                    retry_after = _parse_retry_after(response)
                    backoff = max(retry_after, _exponential_backoff(attempt))
                    if self._debug:
                        logger.debug("promptmeter: rate limited, retrying in %.1fs", backoff)
                    time.sleep(backoff)
                    continue

                if response.status_code >= 500:
                    backoff = _exponential_backoff(attempt)
                    logger.warning(
                        "promptmeter: server error %d, retrying in %.1fs",
                        response.status_code,
                        backoff,
                    )
                    time.sleep(backoff)
                    continue

                # Unexpected status
                logger.warning(
                    "promptmeter: unexpected status %d. Dropping batch.",
                    response.status_code,
                )
                return

            except (httpx.HTTPError, OSError) as exc:
                backoff = _exponential_backoff(attempt)
                logger.warning(
                    "promptmeter: network error: %s. Retrying in %.1fs",
                    str(exc),
                    backoff,
                )
                time.sleep(backoff)
                continue

        # All retries exhausted
        logger.warning(
            "promptmeter: all %d retries exhausted. Dropping %d events.",
            self._max_retries,
            len(events),
        )

    @property
    def auth_failed(self) -> bool:
        return self._auth_failed


def _exponential_backoff(attempt: int) -> float:
    """Calculate exponential backoff: 1s, 2s, 4s, ..."""
    return min(2**attempt, 30)


def _parse_retry_after(response: httpx.Response) -> float:
    """Parse Retry-After header value in seconds."""
    value = response.headers.get("Retry-After", "")
    try:
        return float(value)
    except (ValueError, TypeError):
        return 1.0
