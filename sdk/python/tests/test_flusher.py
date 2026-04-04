"""Tests for the background flush worker."""

import time
import json
from http.server import HTTPServer, BaseHTTPRequestHandler
import threading

import pytest
from promptmeter._queue import _EventQueue
from promptmeter._flusher import _FlushWorker


class MockHandler(BaseHTTPRequestHandler):
    """Mock HTTP server that accepts or rejects batches."""

    received_batches = []
    status_to_return = 202

    def do_POST(self):
        content_length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(content_length)
        data = json.loads(body)
        MockHandler.received_batches.append(data)

        self.send_response(MockHandler.status_to_return)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"data": {"accepted": len(data.get("events", []))}}).encode())

    def log_message(self, *args):
        pass  # Suppress server logs in tests


@pytest.fixture
def mock_server():
    MockHandler.received_batches = []
    MockHandler.status_to_return = 202

    server = HTTPServer(("127.0.0.1", 0), MockHandler)
    port = server.server_address[1]
    thread = threading.Thread(target=server.serve_forever, daemon=True)
    thread.start()
    yield f"http://127.0.0.1:{port}", MockHandler
    server.shutdown()


def test_flush_sends_batch(mock_server):
    endpoint, handler = mock_server
    q = _EventQueue(max_size=100)
    flusher = _FlushWorker(
        queue=q,
        api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
        endpoint=endpoint,
        batch_size=50,
        flush_interval=0.1,
        max_retries=0,
    )

    for i in range(3):
        q.put({"model": "gpt-4o", "idempotency_key": f"key-{i}"})

    flusher.start()
    time.sleep(0.3)
    flusher.stop()

    assert len(handler.received_batches) >= 1
    total_events = sum(len(b.get("events", [])) for b in handler.received_batches)
    assert total_events == 3


def test_flush_auth_failure_disables(mock_server):
    endpoint, handler = mock_server
    handler.status_to_return = 401

    q = _EventQueue(max_size=100)
    flusher = _FlushWorker(
        queue=q,
        api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
        endpoint=endpoint,
        batch_size=50,
        flush_interval=0.1,
        max_retries=0,
    )

    q.put({"model": "gpt-4o"})
    flusher.start()
    time.sleep(0.3)

    assert flusher.auth_failed is True

    # Further events should be dropped
    q.put({"model": "gpt-4o"})
    time.sleep(0.2)

    flusher.stop()
    # Only 1 batch should have been sent (the first one before auth failure)
    assert len(handler.received_batches) == 1


def test_flush_400_drops_batch(mock_server):
    endpoint, handler = mock_server
    handler.status_to_return = 400

    q = _EventQueue(max_size=100)
    flusher = _FlushWorker(
        queue=q,
        api_key="pm_test_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdef",
        endpoint=endpoint,
        batch_size=50,
        flush_interval=0.1,
        max_retries=0,
    )

    q.put({"model": "gpt-4o"})
    flusher.start()
    time.sleep(0.3)
    flusher.stop()

    # Batch should have been sent and dropped (no retry)
    assert len(handler.received_batches) == 1
    assert flusher.auth_failed is False
