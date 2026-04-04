"""Tests for OpenAI client wrapping."""

from __future__ import annotations

import time
from typing import Any
from unittest.mock import MagicMock, patch

import pytest
from promptmeter import PromptMeter


def _make_mock_openai_client() -> Any:
    """Create a mock OpenAI client with the expected structure."""
    client = MagicMock()
    client.__class__.__name__ = "OpenAI"
    client.__class__.__module__ = "openai"

    # Mock usage response
    usage = MagicMock()
    usage.prompt_tokens = 100
    usage.completion_tokens = 50
    usage.total_tokens = 150

    message = MagicMock()
    message.content = "Hello!"

    choice = MagicMock()
    choice.message = message

    response = MagicMock()
    response.usage = usage
    response.choices = [choice]

    client.chat.completions.create.return_value = response
    return client, response


def test_wrap_openai_chat(api_key: str) -> None:
    pm = PromptMeter(api_key=api_key)
    client, expected_response = _make_mock_openai_client()

    wrapped = pm.wrap(client)

    result = wrapped.chat.completions.create(
        model="gpt-4o",
        messages=[{"role": "user", "content": "Hi"}],
    )

    # Original response is returned unchanged
    assert result is expected_response

    # The original method was called
    client.chat.completions.create.assert_called_once()

    pm.shutdown()


def test_wrap_openai_error_tracking(api_key: str) -> None:
    """LLM API errors should be tracked with error status_code."""
    pm = PromptMeter(api_key=api_key)
    client, _ = _make_mock_openai_client()

    # Make create raise an exception
    exc = Exception("API error")
    exc.status_code = 429  # type: ignore[attr-defined]
    client.chat.completions.create.side_effect = exc

    wrapped = pm.wrap(client)

    with pytest.raises(Exception, match="API error"):
        wrapped.chat.completions.create(
            model="gpt-4o",
            messages=[{"role": "user", "content": "Hi"}],
        )

    pm.shutdown()


def test_wrap_openai_embeddings(api_key: str) -> None:
    pm = PromptMeter(api_key=api_key)
    client, _ = _make_mock_openai_client()

    # Mock embedding response
    usage = MagicMock()
    usage.prompt_tokens = 50
    usage.total_tokens = 50

    response = MagicMock()
    response.usage = usage

    client.embeddings.create.return_value = response

    wrapped = pm.wrap(client)
    result = wrapped.embeddings.create(
        model="text-embedding-3-small",
        input="Hello world",
    )

    assert result is response
    pm.shutdown()


def test_wrap_preserves_original_on_failure(api_key: str) -> None:
    """If wrapping somehow fails, the original client is returned."""
    pm = PromptMeter(api_key=api_key)

    # A completely unknown object
    original = object()
    result = pm.wrap(original)
    assert result is original
    pm.shutdown()
