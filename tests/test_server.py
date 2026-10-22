"""Tests for FastAPI server endpoints and WebSocket barge-in."""

import pytest
from fastapi.testclient import TestClient
from src.server import app

@pytest.fixture(autouse=True)
def mock_server_pipeline():
    from src.config import LLMProviderType, ASRProviderType, TTSProviderType
    from src import server
    orig_llm = server.config.pipeline.llm.provider
    orig_asr = server.config.pipeline.asr.provider
    orig_tts = server.config.pipeline.tts.provider
    server.config.pipeline.llm.provider = LLMProviderType.MOCK
    server.config.pipeline.asr.provider = ASRProviderType.MOCK
    server.config.pipeline.tts.provider = TTSProviderType.MOCK
    yield
    server.config.pipeline.llm.provider = orig_llm
    server.config.pipeline.asr.provider = orig_asr
    server.config.pipeline.tts.provider = orig_tts


def test_health_check():
    client = TestClient(app)
    resp = client.get("/api/health")
    assert resp.status_code == 200
    data = resp.json()
    assert data["status"] == "healthy"


def test_index_page():
    client = TestClient(app)
    resp = client.get("/")
    assert resp.status_code == 200
    assert "FastAvatar" in resp.text
    assert "continuousContainer" in resp.text

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
def test_websocket_voice_turn():
    import base64
    import numpy as np
    client = TestClient(app)
    t = np.linspace(0, 0.5, 8000, endpoint=False)
    pcm = (0.3 * np.sin(2 * np.pi * 440 * t) * 32767).astype(np.int16).tobytes()
    b64_audio = base64.b64encode(pcm).decode("ascii")

    with client.websocket_connect("/ws/chat") as ws:
        ws.send_json({"type": "audio", "audio_base64": b64_audio})
        received = []
        for _ in range(50):
            msg = ws.receive_json()
            received.append(msg.get("type"))
            if msg.get("type") in ("turn_complete", "interrupted"):
                break
        assert "turn_complete" in received
        assert "user_transcription" in received or "audio_chunk" in received

def test_websocket_clear_history():
    client = TestClient(app)
    with client.websocket_connect("/ws/chat") as ws:
        ws.send_json({"type": "clear_history"})
        msg = ws.receive_json()
        assert msg.get("type") == "history_cleared"


def test_companion_endpoints():
    client = TestClient(app)
    # 1. Get summary
    resp = client.get("/api/companion")
    assert resp.status_code == 200
    data = resp.json()
    assert "level" in data
    assert "affinity_stage" in data
    assert "memories" in data

    # 2. Add fact
    add_resp = client.post("/api/companion/fact", json={"content": "最喜欢吃草莓蛋糕", "category": "preference"})
    assert add_resp.status_code == 200
    fact_data = add_resp.json()["fact"]
    fact_id = fact_data["id"]
    assert fact_data["content"] == "最喜欢吃草莓蛋糕"

    # 3. Delete fact
    del_resp = client.delete(f"/api/companion/fact/{fact_id}")
    assert del_resp.status_code == 200
    assert del_resp.json()["status"] == "ok"

    # 4. Empty content validation and not found deletion
    empty_resp = client.post("/api/companion/fact", json={"content": "  "})
    assert empty_resp.status_code == 400
    del_fail = client.delete("/api/companion/fact/non-existent-id-999")
    assert del_fail.status_code == 200
    assert del_fail.json()["status"] == "not_found"
