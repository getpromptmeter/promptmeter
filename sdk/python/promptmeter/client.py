"""PromptMeter client -- the main entry point for the SDK."""

from __future__ import annotations

import asyncio
import functools
import logging
import os
from typing import Any, Callable, Dict, Optional, TypeVar, Union

from ._context import get_trace_tags, merge_trace_tags, set_trace_tags
from ._flusher import _FlushWorker
from ._queue import _EventQueue
from ._types import Event
from ._wrap import wrap_client

logger = logging.getLogger("promptmeter")

F = TypeVar("F", bound=Callable[..., Any])


class PromptMeter:
    """Promptmeter SDK client for tracking LLM API calls.

    Usage::

        pm = PromptMeter(api_key="pm_live_xxx")
        client = pm.wrap(OpenAI())
        # All calls are now tracked automatically.

    After initialization, the SDK never raises exceptions.
    """

    def __init__(
        self,
        api_key: Optional[str] = None,
        endpoint: str = "https://ingest.promptmeter.dev",
        tags: Optional[Dict[str, str]] = None,
        pii: bool = True,
        batch_size: int = 50,
        flush_interval: float = 5.0,
        flush_timeout: float = 5.0,
        max_queue_size: int = 10000,
        max_retries: int = 3,
        debug: bool = False,
        enabled: bool = True,
    ) -> None:
        # Resolve config from env with explicit params taking priority
        self._api_key = api_key or os.environ.get("PROMPTMETER_API_KEY", "")
        self._endpoint = endpoint or os.environ.get(
            "PROMPTMETER_ENDPOINT", "https://ingest.promptmeter.dev"
        )
        self._pii = pii if pii is not None else os.environ.get("PROMPTMETER_PII", "true").lower() == "true"
        self._debug = debug or os.environ.get("PROMPTMETER_DEBUG", "false").lower() == "true"
        self._enabled = enabled if enabled is not None else os.environ.get("PROMPTMETER_ENABLED", "true").lower() == "true"
        self._default_tags = tags or {}
        self._flush_timeout = flush_timeout

        # Validation -- these are the ONLY cases where SDK raises
        if not self._enabled:
            return

        if not self._api_key:
            raise ValueError(
                "api_key is required. Set PROMPTMETER_API_KEY or pass api_key parameter"
            )

        if not self._api_key.startswith("pm_live_") and not self._api_key.startswith("pm_test_"):
            raise ValueError(
                "Invalid api_key format. Expected pm_live_* or pm_test_*"
            )

        if batch_size < 1 or batch_size > 1000:
            raise ValueError("batch_size must be between 1 and 1000")

        if flush_interval < 0.1 or flush_interval > 60.0:
            raise ValueError("flush_interval must be between 0.1 and 60.0")

        if max_queue_size < 100 or max_queue_size > 100000:
            raise ValueError("max_queue_size must be between 100 and 100000")

        # Initialize internal components
        self._queue = _EventQueue(max_size=max_queue_size)
        self._flusher = _FlushWorker(
            queue=self._queue,
            api_key=self._api_key,
            endpoint=self._endpoint,
            batch_size=batch_size,
            flush_interval=flush_interval,
            flush_timeout=flush_timeout,
            max_retries=max_retries,
            debug=self._debug,
        )
        self._flusher.start()

        if self._debug:
            logging.basicConfig(level=logging.DEBUG)
            logger.setLevel(logging.DEBUG)

    def track(
        self,
        *,
        model: str,
        provider: str = "unknown",
        prompt_tokens: int = 0,
        completion_tokens: int = 0,
        total_tokens: Optional[int] = None,
        latency_ms: int = 0,
        status_code: int = 200,
        tags: Optional[Dict[str, str]] = None,
        prompt: Optional[str] = None,
        response: Optional[str] = None,
    ) -> None:
        """Track a single LLM API call.

        Never raises exceptions after SDK initialization.
        If the event is invalid or the queue is full, a warning is logged.
        """
        if not self._enabled:
            return

        try:
            if not model:
                logger.warning("promptmeter: model is required, dropping event")
                return

            # Merge tags: default < trace context < per-event
            merged_tags = dict(self._default_tags)
            merged_tags.update(get_trace_tags())
            if tags:
                merged_tags.update(tags)

            # Strip prompt/response if pii=False
            if not self._pii:
                prompt = None
                response = None

            event = Event(
                model=model,
                provider=provider,
                prompt_tokens=prompt_tokens,
                completion_tokens=completion_tokens,
                total_tokens=total_tokens,
                latency_ms=latency_ms,
                status_code=status_code,
                tags=merged_tags,
                prompt=prompt,
                response=response,
            )

            self._queue.put(event.to_dict())

        except Exception as exc:
            logger.warning("promptmeter: failed to track event: %s", exc)

    def wrap(self, client: Any) -> Any:
        """Wrap an LLM client for automatic tracking.

        Supports OpenAI and Anthropic clients. Returns the original
        client unwrapped if the type is not recognized.
        """
        if not self._enabled:
            return client

        try:
            return wrap_client(self, client)
        except Exception as exc:
            logger.warning("promptmeter: failed to wrap client: %s", exc)
            return client

    def trace(self, **decorator_tags: str) -> Callable[[F], F]:
        """Decorator that adds tags to all events tracked within the function.

        Tags from nested @trace decorators are merged (inner overrides outer).

        Usage::

            @pm.trace(feature="chatbot", team="support")
            def handle_chat(message: str):
                ...
        """

        def decorator(func: F) -> F:
            if asyncio.iscoroutinefunction(func):

                @functools.wraps(func)
                async def async_wrapper(*args: Any, **kwargs: Any) -> Any:
                    merged = merge_trace_tags(decorator_tags)
                    old_tags = get_trace_tags()
                    set_trace_tags(merged)
                    try:
                        return await func(*args, **kwargs)
                    finally:
                        set_trace_tags(old_tags)

                return async_wrapper  # type: ignore[return-value]
            else:

                @functools.wraps(func)
                def sync_wrapper(*args: Any, **kwargs: Any) -> Any:
                    merged = merge_trace_tags(decorator_tags)
                    old_tags = get_trace_tags()
                    set_trace_tags(merged)
                    try:
                        return func(*args, **kwargs)
                    finally:
                        set_trace_tags(old_tags)

                return sync_wrapper  # type: ignore[return-value]

        return decorator

    def shutdown(self) -> None:
        """Flush remaining events and stop the background thread."""
        if not self._enabled:
            return

        try:
            self._flusher.stop(timeout=self._flush_timeout)
        except Exception as exc:
            logger.warning("promptmeter: shutdown error: %s", exc)
