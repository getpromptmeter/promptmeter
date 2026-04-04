"""Shared test fixtures for the Promptmeter SDK tests."""

import pytest


@pytest.fixture
def api_key() -> str:
    """A valid test API key."""
    return "pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef"
