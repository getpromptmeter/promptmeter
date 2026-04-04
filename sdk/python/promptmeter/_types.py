"""Event type definitions for the Promptmeter SDK."""

from __future__ import annotations

import json
import uuid
from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import Any, Dict, Optional


def _generate_uuid_v7() -> str:
    """Generate a UUID v7 (time-ordered) as a string."""
    # Use uuid7 if available (Python 3.12+), otherwise approximate with uuid4
    try:
        return str(uuid.uuid7())
    except AttributeError:
        # Fallback: create a time-ordered UUID by embedding timestamp in uuid4
        now = int(datetime.now(timezone.utc).timestamp() * 1000)
        u = uuid.uuid4()
        # Replace first 48 bits with timestamp, set version to 7
        hex_str = f"{now:012x}" + u.hex[12:]
        # Set version nibble to 7
        hex_str = hex_str[:12] + "7" + hex_str[13:]
        # Set variant bits
        variant_nibble = int(hex_str[16], 16) & 0x3 | 0x8
        hex_str = hex_str[:16] + format(variant_nibble, "x") + hex_str[17:]
        return str(uuid.UUID(hex_str))


@dataclass
class Event:
    """Represents a single LLM API call event."""

    model: str
    provider: str = "unknown"
    prompt_tokens: int = 0
    completion_tokens: int = 0
    total_tokens: Optional[int] = None
    latency_ms: int = 0
    status_code: int = 200
    tags: Dict[str, str] = field(default_factory=dict)
    prompt: Optional[str] = None
    response: Optional[str] = None
    idempotency_key: str = field(default_factory=_generate_uuid_v7)
    timestamp: str = field(
        default_factory=lambda: datetime.now(timezone.utc).isoformat()
    )
    schema_version: int = 1

    def to_dict(self) -> Dict[str, Any]:
        """Serialize event to a dictionary for JSON transmission."""
        total = (
            self.total_tokens
            if self.total_tokens is not None
            else self.prompt_tokens + self.completion_tokens
        )

        d: Dict[str, Any] = {
            "idempotency_key": self.idempotency_key,
            "timestamp": self.timestamp,
            "model": self.model,
            "provider": self.provider,
            "prompt_tokens": self.prompt_tokens,
            "completion_tokens": self.completion_tokens,
            "total_tokens": total,
            "latency_ms": self.latency_ms,
            "status_code": self.status_code,
            "tags": self.tags or {},
            "schema_version": self.schema_version,
        }

        if self.prompt is not None:
            d["prompt"] = self.prompt
        if self.response is not None:
            d["response"] = self.response

        return d
