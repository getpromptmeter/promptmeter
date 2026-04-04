"""Tests for PromptMeter client constructor, track(), and shutdown()."""

import pytest
from promptmeter import PromptMeter


def test_constructor_with_valid_key(api_key):
    pm = PromptMeter(api_key=api_key, enabled=True)
    pm.shutdown()


def test_constructor_missing_key():
    with pytest.raises(ValueError, match="api_key is required"):
        PromptMeter(api_key="", enabled=True)


def test_constructor_invalid_format():
    with pytest.raises(ValueError, match="Invalid api_key format"):
        PromptMeter(api_key="sk_live_invalid", enabled=True)


def test_constructor_invalid_batch_size(api_key):
    with pytest.raises(ValueError, match="batch_size"):
        PromptMeter(api_key=api_key, batch_size=0)


def test_constructor_invalid_flush_interval(api_key):
    with pytest.raises(ValueError, match="flush_interval"):
        PromptMeter(api_key=api_key, flush_interval=0.01)


def test_constructor_invalid_queue_size(api_key):
    with pytest.raises(ValueError, match="max_queue_size"):
        PromptMeter(api_key=api_key, max_queue_size=10)


def test_disabled_mode():
    """When enabled=False, SDK is a no-op. No ValueError even without api_key."""
    pm = PromptMeter(enabled=False)
    pm.track(model="gpt-4o")  # should not raise
    pm.shutdown()


def test_track_does_not_raise(api_key):
    pm = PromptMeter(api_key=api_key)
    # Should not raise even with missing fields
    pm.track(model="gpt-4o", provider="openai", prompt_tokens=100, completion_tokens=50)
    pm.shutdown()


def test_track_empty_model(api_key):
    """Empty model should log a warning, not raise."""
    pm = PromptMeter(api_key=api_key)
    pm.track(model="")  # should not raise
    pm.shutdown()


def test_track_with_tags(api_key):
    pm = PromptMeter(api_key=api_key, tags={"env": "test"})
    pm.track(
        model="gpt-4o",
        provider="openai",
        prompt_tokens=100,
        completion_tokens=50,
        tags={"feature": "chat"},
    )
    pm.shutdown()


def test_track_pii_false(api_key):
    """When pii=False, prompt/response should be stripped."""
    pm = PromptMeter(api_key=api_key, pii=False)
    pm.track(
        model="gpt-4o",
        provider="openai",
        prompt_tokens=100,
        completion_tokens=50,
        prompt="secret prompt",
        response="secret response",
    )
    # We can't easily verify the event was stripped without mocking the queue,
    # but this ensures the code path doesn't raise.
    pm.shutdown()


def test_wrap_unknown_returns_original(api_key):
    """wrap() with unsupported type returns the original object."""
    pm = PromptMeter(api_key=api_key)

    class UnknownClient:
        pass

    original = UnknownClient()
    result = pm.wrap(original)
    assert result is original
    pm.shutdown()


def test_shutdown_idempotent(api_key):
    pm = PromptMeter(api_key=api_key)
    pm.shutdown()
    pm.shutdown()  # Should not raise
