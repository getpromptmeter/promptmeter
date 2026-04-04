"""Provider wrapping for automatic LLM call tracking."""

from __future__ import annotations

import json
import logging
import time
from functools import wraps
from typing import Any, Callable, Optional, TYPE_CHECKING

if TYPE_CHECKING:
    from .client import PromptMeter

logger = logging.getLogger("promptmeter")


def wrap_client(pm: "PromptMeter", client: Any) -> Any:
    """Wrap an LLM client for automatic tracking.

    Supports:
    - openai.OpenAI / openai.AsyncOpenAI
    - anthropic.Anthropic / anthropic.AsyncAnthropic

    Returns the original client if the type is not recognized.
    """
    client_type = type(client).__name__
    module = type(client).__module__ or ""

    if "openai" in module:
        if client_type in ("OpenAI", "AsyncOpenAI"):
            return _wrap_openai(pm, client)

    if "anthropic" in module:
        if client_type in ("Anthropic", "AsyncAnthropic"):
            return _wrap_anthropic(pm, client)

    logger.warning(
        "promptmeter: unsupported client type: %s. Returning unwrapped.",
        client_type,
    )
    return client


def _wrap_openai(pm: "PromptMeter", client: Any) -> Any:
    """Wrap OpenAI client to intercept chat.completions.create, completions.create, embeddings.create."""
    # Wrap chat.completions.create
    if hasattr(client, "chat") and hasattr(client.chat, "completions"):
        original_create = client.chat.completions.create
        client.chat.completions.create = _make_openai_chat_wrapper(pm, original_create)

    # Wrap completions.create (legacy)
    if hasattr(client, "completions") and not hasattr(client.completions, "chat"):
        try:
            original_create = client.completions.create
            client.completions.create = _make_openai_completion_wrapper(pm, original_create)
        except AttributeError:
            pass

    # Wrap embeddings.create
    if hasattr(client, "embeddings"):
        original_create = client.embeddings.create
        client.embeddings.create = _make_openai_embedding_wrapper(pm, original_create)

    return client


def _make_openai_chat_wrapper(pm: "PromptMeter", original: Callable) -> Callable:
    """Create a wrapper for OpenAI chat.completions.create."""

    @wraps(original)
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        start = time.monotonic()
        response = None
        status_code = 200
        error_occurred = False

        try:
            response = original(*args, **kwargs)
            return response
        except Exception as exc:
            error_occurred = True
            status_code = _extract_status_from_exception(exc)
            raise
        finally:
            latency_ms = int((time.monotonic() - start) * 1000)
            try:
                _track_openai_chat(pm, kwargs, response, latency_ms, status_code, error_occurred)
            except Exception:
                pass  # SDK never raises after init

    return wrapper


def _make_openai_completion_wrapper(pm: "PromptMeter", original: Callable) -> Callable:
    @wraps(original)
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        start = time.monotonic()
        response = None
        status_code = 200

        try:
            response = original(*args, **kwargs)
            return response
        except Exception as exc:
            status_code = _extract_status_from_exception(exc)
            raise
        finally:
            latency_ms = int((time.monotonic() - start) * 1000)
            try:
                model = kwargs.get("model", "unknown")
                prompt_tokens = 0
                completion_tokens = 0
                response_text = None

                if response and hasattr(response, "usage") and response.usage:
                    prompt_tokens = getattr(response.usage, "prompt_tokens", 0) or 0
                    completion_tokens = getattr(response.usage, "completion_tokens", 0) or 0

                if response and hasattr(response, "choices") and response.choices:
                    response_text = getattr(response.choices[0], "text", None)

                prompt_text = kwargs.get("prompt")
                if isinstance(prompt_text, list):
                    prompt_text = json.dumps(prompt_text)

                pm.track(
                    model=model,
                    provider="openai",
                    prompt_tokens=prompt_tokens,
                    completion_tokens=completion_tokens,
                    latency_ms=latency_ms,
                    status_code=status_code,
                    prompt=str(prompt_text) if prompt_text and pm._pii else None,
                    response=response_text if pm._pii else None,
                )
            except Exception:
                pass

    return wrapper


def _make_openai_embedding_wrapper(pm: "PromptMeter", original: Callable) -> Callable:
    @wraps(original)
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        start = time.monotonic()
        response = None
        status_code = 200

        try:
            response = original(*args, **kwargs)
            return response
        except Exception as exc:
            status_code = _extract_status_from_exception(exc)
            raise
        finally:
            latency_ms = int((time.monotonic() - start) * 1000)
            try:
                model = kwargs.get("model", "unknown")
                prompt_tokens = 0

                if response and hasattr(response, "usage") and response.usage:
                    prompt_tokens = getattr(response.usage, "prompt_tokens", 0) or 0
                    total = getattr(response.usage, "total_tokens", 0) or 0
                    if total and not prompt_tokens:
                        prompt_tokens = total

                input_text = kwargs.get("input")
                prompt_text = None
                if input_text:
                    prompt_text = json.dumps(input_text) if isinstance(input_text, list) else str(input_text)

                pm.track(
                    model=model,
                    provider="openai",
                    prompt_tokens=prompt_tokens,
                    completion_tokens=0,
                    latency_ms=latency_ms,
                    status_code=status_code,
                    prompt=prompt_text if pm._pii else None,
                )
            except Exception:
                pass

    return wrapper


def _track_openai_chat(
    pm: "PromptMeter",
    kwargs: dict,
    response: Any,
    latency_ms: int,
    status_code: int,
    error_occurred: bool,
) -> None:
    """Extract data from OpenAI chat completion and track it."""
    model = kwargs.get("model", "unknown")
    prompt_tokens = 0
    completion_tokens = 0
    response_text = None

    if response and hasattr(response, "usage") and response.usage:
        prompt_tokens = getattr(response.usage, "prompt_tokens", 0) or 0
        completion_tokens = getattr(response.usage, "completion_tokens", 0) or 0

    if response and hasattr(response, "choices") and response.choices:
        choice = response.choices[0]
        if hasattr(choice, "message") and choice.message:
            response_text = getattr(choice.message, "content", None)

    prompt_text = None
    messages = kwargs.get("messages")
    if messages and pm._pii:
        prompt_text = json.dumps(messages)

    pm.track(
        model=model,
        provider="openai",
        prompt_tokens=prompt_tokens,
        completion_tokens=completion_tokens,
        latency_ms=latency_ms,
        status_code=status_code,
        prompt=prompt_text,
        response=response_text if pm._pii else None,
    )


def _wrap_anthropic(pm: "PromptMeter", client: Any) -> Any:
    """Wrap Anthropic client to intercept messages.create."""
    if hasattr(client, "messages"):
        original_create = client.messages.create
        client.messages.create = _make_anthropic_wrapper(pm, original_create)

    return client


def _make_anthropic_wrapper(pm: "PromptMeter", original: Callable) -> Callable:
    @wraps(original)
    def wrapper(*args: Any, **kwargs: Any) -> Any:
        start = time.monotonic()
        response = None
        status_code = 200

        try:
            response = original(*args, **kwargs)
            return response
        except Exception as exc:
            status_code = _extract_status_from_exception(exc)
            raise
        finally:
            latency_ms = int((time.monotonic() - start) * 1000)
            try:
                model = kwargs.get("model", "unknown")
                prompt_tokens = 0
                completion_tokens = 0
                response_text = None

                if response and hasattr(response, "usage") and response.usage:
                    prompt_tokens = getattr(response.usage, "input_tokens", 0) or 0
                    completion_tokens = getattr(response.usage, "output_tokens", 0) or 0

                if response and hasattr(response, "content") and response.content:
                    first_block = response.content[0]
                    response_text = getattr(first_block, "text", None)

                prompt_text = None
                messages = kwargs.get("messages")
                if messages and pm._pii:
                    prompt_text = json.dumps(messages)

                pm.track(
                    model=model,
                    provider="anthropic",
                    prompt_tokens=prompt_tokens,
                    completion_tokens=completion_tokens,
                    latency_ms=latency_ms,
                    status_code=status_code,
                    prompt=prompt_text,
                    response=response_text if pm._pii else None,
                )
            except Exception:
                pass

    return wrapper


def _extract_status_from_exception(exc: Exception) -> int:
    """Try to extract HTTP status code from an LLM SDK exception."""
    # OpenAI exceptions have status_code attribute
    if hasattr(exc, "status_code"):
        return getattr(exc, "status_code", 500)
    # Anthropic exceptions have status_code attribute too
    if hasattr(exc, "status_code"):
        return getattr(exc, "status_code", 500)
    return 500
