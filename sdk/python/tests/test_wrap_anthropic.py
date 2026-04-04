"""Tests for Anthropic client wrapping."""

from __future__ import annotations

from typing import Any
from unittest.mock import MagicMock

import pytest
from promptmeter import PromptMeter


def _make_mock_anthropic_client() -> Any:
    """Create a mock Anthropic client with the expected structure."""
    client = MagicMock()
    client.__class__.__name__ = "Anthropic"
    client.__class__.__module__ = "anthropic"

    # Mock usage response
    usage = MagicMock()
    usage.input_tokens = 200
    usage.output_tokens = 100

    text_block = MagicMock()
    text_block.text = "Hello from Claude!"

    response = MagicMock()
    response.usage = usage
    response.content = [text_block]

    client.messages.create.return_value = response
    return client, response


def test_wrap_anthropic_messages(api_key: str) -> None:
    pm = PromptMeter(api_key=api_key)
    client, expected_response = _make_mock_anthropic_client()

    wrapped = pm.wrap(client)

    result = wrapped.messages.create(
        model="claude-sonnet-4-20250514",
        max_tokens=1024,
        messages=[{"role": "user", "content": "Hello!"}],
    )

    # Original response is returned unchanged
    assert result is expected_response

    # The original method was called
    client.messages.create.assert_called_once()

    pm.shutdown()


def test_wrap_anthropic_error_tracking(api_key: str) -> None:
    """LLM API errors should be tracked with error status_code."""
    pm = PromptMeter(api_key=api_key)
    client, _ = _make_mock_anthropic_client()

    exc = Exception("Rate limited")
    exc.status_code = 429  # type: ignore[attr-defined]
    client.messages.create.side_effect = exc

    wrapped = pm.wrap(client)

    with pytest.raises(Exception, match="Rate limited"):
        wrapped.messages.create(
            model="claude-sonnet-4-20250514",
            max_tokens=1024,
            messages=[{"role": "user", "content": "Hello!"}],
        )

    pm.shutdown()
