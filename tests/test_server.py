"""Tests for FastAPI server endpoints and WebSocket barge-in."""

import pytest
from fastapi.testclient import TestClient
from src.server import app


def test_health_check():
    client = TestClient(app)
    resp = client.get("/api/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "healthy"


def test_favicon():
    client = TestClient(app)
    resp = client.get("/favicon.ico")
    assert resp.status_code == 204

def test_avatar_endpoint():
    client = TestClient(app)
    resp = client.get("/avatar.png")
    assert resp.status_code == 200
    assert resp.headers["content-type"] == "image/png"

def test_chat_http_endpoint():
    client = TestClient(app)
    resp = client.post("/api/chat", json={"text": "你好！"})
    assert resp.status_code == 200
    data = resp.json()
    assert "events" in data
    assert len(data["events"]) > 0


def test_websocket_barge_in():
    client = TestClient(app)
    with client.websocket_connect("/ws/chat") as ws:
        # Start a turn
        ws.send_json({"type": "text", "text": "你好！"})
        # Immediately send interrupt signal
        ws.send_json({"type": "interrupt"})

        received_types = []
        try:
            for _ in range(5):
                msg = ws.receive_json()
                received_types.append(msg.get("type"))
                if msg.get("type") in ("interrupted", "turn_complete"):
                    break
        except Exception:
            pass

        assert "interrupted" in received_types or "text_chunk" in received_types
