"""Tests for the bounded event queue."""

from promptmeter._queue import _EventQueue


def test_put_and_drain():
    q = _EventQueue(max_size=100)
    for i in range(10):
        assert q.put({"id": i}) is True

    items = q.drain(max_items=5)
    assert len(items) == 5
    assert items[0]["id"] == 0

    items = q.drain(max_items=100)
    assert len(items) == 5


def test_queue_overflow():
    q = _EventQueue(max_size=3)
    assert q.put({"id": 1}) is True
    assert q.put({"id": 2}) is True
    assert q.put({"id": 3}) is True
    assert q.put({"id": 4}) is False  # queue full, dropped

    items = q.drain(max_items=10)
    assert len(items) == 3


def test_drain_empty():
    q = _EventQueue(max_size=100)
    items = q.drain(max_items=10)
    assert len(items) == 0


def test_size_property():
    q = _EventQueue(max_size=100)
    assert q.size == 0
    q.put({"id": 1})
    assert q.size == 1


def test_empty_property():
    q = _EventQueue(max_size=100)
    assert q.empty is True
    q.put({"id": 1})
    assert q.empty is False
