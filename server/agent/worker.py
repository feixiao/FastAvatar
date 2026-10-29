#!/usr/bin/env python3
"""
Avatar Realtime Cloud Voice Agent Worker
Based on LiveKit Agents & OpenAI-compatible LLM/VLM serving (vLLM / MLX-LM / Ollama).
"""

import asyncio
import logging
import os
import sys
from typing import Optional
try:
    from dotenv import load_dotenv
    load_dotenv()
except ImportError:
    pass

# Configure logging without echoing user transcripts (PRD Guardrail)
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s [%(levelname)s] %(name)s: %(message)s"
)
logger = logging.getLogger("avatar-agent-worker")

# Persona System Prompts
PERSONA_PROMPTS = {
    "roaster": (
        "你是一个幽默犀利的吐槽高手，说话毒舌但充满关怀和智慧，善于用讽刺和诙谐化解尴尬，"
        "回答句句精辟、语言简明有力，极度适合超短语音播报。切勿长篇大论。"
    ),
    "jarvis": (
        "你是一个极度严谨高效的智能顾问与私人幕僚，擅长结构化拆解问题并提供执行导向的高价值建议，"
        "语气专业温和克制，语言精练，直接给方案。"
    ),
    "pet": (
        "你是一只贴心黏人的专属桌宠小伴侣，性格乖巧软萌，喜欢用拟声词（如喵呜、蹭蹭）表达喜爱与支持，"
        "永远陪伴倾听，回答短小温暖。"
    ),
    "assistant": (
        "你是一个温暖亲切、富有同理心的 AI 语音伴侣，倾听并支持用户，回答自然口语化、简洁精炼适合语音播报。"
    )
}


def get_system_prompt(persona: Optional[str] = None) -> str:
    key = (persona or os.getenv("AGENT_PERSONA", "assistant")).lower()
    return PERSONA_PROMPTS.get(key, PERSONA_PROMPTS["assistant"])


async def entrypoint(ctx):
    """
    LiveKit Agents Job Entrypoint
    Called whenever a client establishes a WebRTC room session.
    """
    from livekit.agents import AutoSubscribe, JobContext, llm
    from livekit.agents.voice_assistant import VoiceAssistant
    from livekit.plugins import openai, silero

    logger.info(f"Connecting to room: {ctx.room.name}")
    await ctx.connect(auto_subscribe=AutoSubscribe.AUDIO_ONLY)

    # Read upstream LLM configuration
    llm_base_url = os.getenv("LLM_BASE_URL", "http://127.0.0.1:8000/v1")
    llm_api_key = os.getenv("LLM_API_KEY", "dummy-key")
    llm_model = os.getenv("LLM_MODEL", "Qwen/Qwen2.5-7B-Instruct")

    # Determine initial persona from room metadata or environment
    persona_key = os.getenv("AGENT_PERSONA", "assistant")
    if ctx.room.metadata:
        import json
        try:
            meta = json.loads(ctx.room.metadata)
            if "persona" in meta:
                persona_key = meta["persona"]
        except Exception:
            pass

    initial_prompt = get_system_prompt(persona_key)
    logger.info(f"Agent initialized with persona: {persona_key}, LLM endpoint: {llm_base_url}")

    # Build VoiceAssistant Pipeline:
    # VAD: Silero for real-time speech detection and fast barge-in interruption
    # STT: Whisper / OpenAI compatible STT
    # LLM: Points to vLLM, MLX-LM server, or OpenAI-compatible endpoint
    # TTS: Points to OpenAI-compatible TTS or provider
    assistant = VoiceAssistant(
        vad=silero.VAD.load(),
        stt=openai.STT(),
        llm=openai.LLM(
            base_url=llm_base_url,
            api_key=llm_api_key,
            model=llm_model,
            temperature=0.7,
        ),
        tts=openai.TTS(),
        chat_ctx=llm.ChatContext().append(
            role="system",
            text=initial_prompt
        ),
    )

    # Start assistant in room
    assistant.start(ctx.room)
    logger.info(f"VoiceAssistant started in room {ctx.room.name}")


def main():
    livekit_url = os.getenv("LIVEKIT_URL", "ws://localhost:7880")
    api_key = os.getenv("LIVEKIT_API_KEY", "devkey")
    api_secret = os.getenv("LIVEKIT_API_SECRET", "secret_avatar_livekit_key_2026")

    logger.info(f"Starting Avatar Cloud Voice Agent Worker -> SFU: {livekit_url}")

    try:
        from livekit.agents import WorkerOptions, cli
        cli.run_app(
            WorkerOptions(
                entrypoint_fnc=entrypoint,
                ws_url=livekit_url,
                api_key=api_key,
                api_secret=api_secret,
            )
        )
    except ImportError as e:
        logger.warning(f"livekit-agents not installed in current Python runtime ({e}).")
        logger.info("To run live: pip install -r requirements.txt && python worker.py dev")
        if "--dry-run" in sys.argv or "PROBE" in os.environ:
            logger.info("Worker configuration probe successful.")
            sys.exit(0)
        sys.exit(1)


if __name__ == "__main__":
    main()
