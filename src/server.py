"""FastAPI WebSocket and HTTP Server for FastAvatar real-time digital avatar interaction."""

import asyncio
import base64
import json
import os
from pathlib import Path
from typing import Optional
from fastapi import FastAPI, WebSocket, WebSocketDisconnect
from fastapi.responses import HTMLResponse, JSONResponse, Response, FileResponse
from fastapi.middleware.cors import CORSMiddleware

from src.config import AppConfig, get_default_config
from src.pipeline import VoiceAvatarPipeline, PipelineEvent

app = FastAPI(title="FastAvatar", version="0.1.0")

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

config = get_default_config()
pipeline = VoiceAvatarPipeline(config=config.pipeline)

WEB_DIR = Path(__file__).parent / "web"


@app.get("/", response_class=HTMLResponse)
async def serve_index():
    index_file = WEB_DIR / "index.html"
    if index_file.exists():
        return index_file.read_text(encoding="utf-8")
    return "<h1>FastAvatar Engine is running.</h1>"


@app.get("/favicon.ico", include_in_schema=False)
async def favicon():
    return Response(status_code=204)

@app.get("/avatar.png")
async def get_avatar():
    web_avatar = WEB_DIR / "avatar.png"
    if web_avatar.exists():
        return FileResponse(web_avatar, media_type="image/png")
    root_avatar = Path(__file__).parent.parent / "2_03.png"
    if root_avatar.exists():
        return FileResponse(root_avatar, media_type="image/png")
    return Response(status_code=404)

@app.get("/api/health")
async def health_check():
    return {
        "status": "healthy",
        "llm_provider": config.pipeline.llm.provider.value,
        "asr_provider": config.pipeline.asr.provider.value,
        "tts_provider": config.pipeline.tts.provider.value,
    }


@app.post("/api/chat")
async def chat_http(req: dict):
    user_text = req.get("text", "")
    if not user_text:
        return JSONResponse(status_code=400, content={"error": "Text is required"})

    events = []
    async for event in pipeline.process_user_turn(text_input=user_text):
        if event.event_type == "audio_chunk":
            # For JSON serialization, base64-encode raw audio bytes
            payload = dict(event.payload)
            payload["audio_base64"] = base64.b64encode(payload.pop("audio_data")).decode("ascii")
            events.append({"event_type": event.event_type, "payload": payload})
        else:
            events.append(event.model_dump())

    return {"events": events}


@app.get("/api/companion")
async def get_companion_profile():
    return pipeline.companion.get_summary()


@app.post("/api/companion/fact")
async def add_companion_fact(req: dict):
    content = req.get("content", "").strip()
    category = req.get("category", "preference")
    if not content:
        return JSONResponse(status_code=400, content={"error": "Content cannot be empty"})
    fact = pipeline.companion.add_fact(content=content, category=category)
    return {"status": "ok", "fact": fact.model_dump()}


@app.delete("/api/companion/fact/{fact_id}")
async def delete_companion_fact(fact_id: str):
    success = pipeline.companion.remove_fact(fact_id)
    return {"status": "ok" if success else "not_found"}

@app.websocket("/ws/chat")
async def websocket_chat(websocket: WebSocket):
    await websocket.accept()
    session_pipeline = VoiceAvatarPipeline(config=config.pipeline)
    current_turn_task: Optional[asyncio.Task] = None

    async def run_turn(audio_bytes: Optional[bytes], user_text: Optional[str]):
        try:
            async for event in session_pipeline.process_user_turn(
                audio_input=audio_bytes,
                text_input=user_text,
            ):
                if event.event_type == "audio_chunk":
                    payload = dict(event.payload)
                    audio_b64 = base64.b64encode(payload.pop("audio_data")).decode("ascii")
                    payload["audio_base64"] = audio_b64
                    await websocket.send_json({"type": "audio_chunk", "data": payload})
                elif event.event_type == "text_chunk":
                    await websocket.send_json({"type": "text_chunk", "data": event.payload})
                elif event.event_type == "metrics":
                    await websocket.send_json({"type": "metrics", "data": event.payload})
                elif event.event_type == "user_transcription":
                    await websocket.send_json({"type": "user_transcription", "data": event.payload})
                elif event.event_type == "companion_update":
                    await websocket.send_json({"type": "companion_update", "data": event.payload})
                elif event.event_type == "interrupted":
                    await websocket.send_json({"type": "interrupted", "data": event.payload})
            await websocket.send_json({"type": "turn_complete"})
        except asyncio.CancelledError:
            pass
        except Exception as e:
            print(f"[Server] run_turn unhandled error: {e}")
            try:
                await websocket.send_json({
                    "type": "user_transcription",
                    "data": {"text": f"（语音处理异常: {e}）"}
                })
                await websocket.send_json({"type": "turn_complete"})
            except Exception:
                pass
    try:
        while True:
            raw_msg = await websocket.receive_text()
            data = json.loads(raw_msg)
            msg_type = data.get("type", "")

            if msg_type == "interrupt":
                session_pipeline.trigger_interrupt()
                if current_turn_task and not current_turn_task.done():
                    current_turn_task.cancel()
                await websocket.send_json({"type": "interrupted"})
                continue

            elif msg_type == "clear_history":
                session_pipeline.clear_history()
                if current_turn_task and not current_turn_task.done():
                    session_pipeline.trigger_interrupt()
                    current_turn_task.cancel()
                await websocket.send_json({"type": "history_cleared"})
                continue
            elif msg_type in ("text", "audio"):
                if current_turn_task and not current_turn_task.done():
                    session_pipeline.trigger_interrupt()
                    current_turn_task.cancel()

                user_text = data.get("text")
                audio_bytes = None
                if msg_type == "audio" and "audio_base64" in data:
                    audio_bytes = base64.b64decode(data["audio_base64"])

                current_turn_task = asyncio.create_task(run_turn(audio_bytes, user_text))

    except WebSocketDisconnect:
        if current_turn_task and not current_turn_task.done():
            current_turn_task.cancel()


def run():
    import uvicorn
    uvicorn.run("src.server:app", host=config.host, port=config.port, reload=True)


if __name__ == "__main__":
    run()
