"""Tests for SDK config validation edge cases."""

import os
import pytest
from promptmeter import PromptMeter


def test_api_key_from_env(monkeypatch):
    monkeypatch.setenv("PROMPTMETER_API_KEY", "pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
    pm = PromptMeter()  # should pick up from env
    pm.shutdown()


def test_explicit_key_overrides_env(monkeypatch):
    monkeypatch.setenv("PROMPTMETER_API_KEY", "pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
    pm = PromptMeter(api_key="pm_live_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef")
    pm.shutdown()


def test_batch_size_too_large():
    with pytest.raises(ValueError, match="batch_size"):
        PromptMeter(api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef", batch_size=1001)


def test_flush_interval_too_large():
    with pytest.raises(ValueError, match="flush_interval"):
        PromptMeter(api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef", flush_interval=61.0)


def test_max_retries_zero_is_valid():
    """max_retries=0 means no retries, should be valid."""
    pm = PromptMeter(api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef", max_retries=0)
    pm.shutdown()
