"""Promptmeter SDK -- Track LLM API costs effortlessly.

Usage::

    from promptmeter import PromptMeter
    pm = PromptMeter(api_key="pm_live_xxx")

    # Automatic tracking:
    from openai import OpenAI
    client = pm.wrap(OpenAI())

    # Manual tracking:
    pm.track(model="gpt-4o", provider="openai", prompt_tokens=100, completion_tokens=50)

    # Global init (monkey-patching):
    import promptmeter
    promptmeter.init(api_key="pm_live_xxx")
"""

from __future__ import annotations

import logging
from typing import Any, Dict, Optional

from .client import PromptMeter
from ._version import __version__

__all__ = ["PromptMeter", "init", "shutdown", "__version__"]

logger = logging.getLogger("promptmeter")

_global_instance: Optional[PromptMeter] = None
_original_openai_init: Any = None
_original_anthropic_init: Any = None


def init(
    api_key: Optional[str] = None,
    endpoint: str = "https://ingest.promptmeter.dev",
    tags: Optional[Dict[str, str]] = None,
    pii: bool = True,
    batch_size: int = 50,
    flush_interval: float = 5.0,
    max_queue_size: int = 10000,
    max_retries: int = 3,
    debug: bool = False,
    enabled: bool = True,
) -> None:
    """Initialize Promptmeter globally and monkey-patch LLM clients.

    After calling init(), all new OpenAI() and Anthropic() instances
    will be automatically wrapped for tracking.

    Raises ValueError only for invalid configuration.
    """
    global _global_instance

    if _global_instance is not None:
        logger.warning("promptmeter: init() called more than once. Ignoring.")
        return

    _global_instance = PromptMeter(
        api_key=api_key,
        endpoint=endpoint,
        tags=tags,
        pii=pii,
        batch_size=batch_size,
        flush_interval=flush_interval,
        max_queue_size=max_queue_size,
        max_retries=max_retries,
        debug=debug,
        enabled=enabled,
    )

    if not enabled:
        return

    _monkey_patch_openai()
    _monkey_patch_anthropic()


def shutdown() -> None:
    """Flush remaining events and restore monkey-patched constructors."""
    global _global_instance

    if _global_instance is not None:
        _global_instance.shutdown()
        _global_instance = None

    _restore_openai()
    _restore_anthropic()


def _monkey_patch_openai() -> None:
    global _original_openai_init
    try:
        import openai

        _original_openai_init = openai.OpenAI.__init__

        def patched_init(self: Any, *args: Any, **kwargs: Any) -> None:
            _original_openai_init(self, *args, **kwargs)
            if _global_instance is not None:
                _global_instance.wrap(self)

        openai.OpenAI.__init__ = patched_init  # type: ignore[attr-defined]
    except ImportError:
        pass


def _monkey_patch_anthropic() -> None:
    global _original_anthropic_init
    try:
        import anthropic

        _original_anthropic_init = anthropic.Anthropic.__init__

        def patched_init(self: Any, *args: Any, **kwargs: Any) -> None:
            _original_anthropic_init(self, *args, **kwargs)
            if _global_instance is not None:
                _global_instance.wrap(self)

        anthropic.Anthropic.__init__ = patched_init  # type: ignore[attr-defined]
    except ImportError:
        pass


def _restore_openai() -> None:
    global _original_openai_init
    if _original_openai_init is not None:
        try:
            import openai

            openai.OpenAI.__init__ = _original_openai_init  # type: ignore[attr-defined]
        except ImportError:
            pass
        _original_openai_init = None


def _restore_anthropic() -> None:
    global _original_anthropic_init
    if _original_anthropic_init is not None:
        try:
            import anthropic

            anthropic.Anthropic.__init__ = _original_anthropic_init  # type: ignore[attr-defined]
        except ImportError:
            pass
        _original_anthropic_init = None
