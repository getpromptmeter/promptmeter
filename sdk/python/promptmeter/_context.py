"""Context variable support for the @trace decorator."""

from __future__ import annotations

from contextvars import ContextVar
from typing import Dict, Optional

# Stores the current trace tags set by @pm.trace() decorators.
# Tags from nested decorators are merged (inner overrides outer).
_trace_tags: ContextVar[Optional[Dict[str, str]]] = ContextVar(
    "promptmeter_trace_tags", default=None
)


def get_trace_tags() -> Dict[str, str]:
    """Get the current trace tags from context, or empty dict."""
    tags = _trace_tags.get()
    return dict(tags) if tags else {}


def set_trace_tags(tags: Dict[str, str]) -> None:
    """Set trace tags in the current context."""
    _trace_tags.set(tags)


def merge_trace_tags(new_tags: Dict[str, str]) -> Dict[str, str]:
    """Merge new tags with existing trace tags (new overrides existing)."""
    current = get_trace_tags()
    current.update(new_tags)
    return current
