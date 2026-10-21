"""Integration tests for the complete VoiceAvatarPipeline."""

import pytest
from src.config import PipelineConfig, LLMConfig, ASRConfig, TTSConfig, LLMProviderType, ASRProviderType, TTSProviderType
from src.pipeline import VoiceAvatarPipeline


@pytest.mark.asyncio
async def test_pipeline_streaming_turn():
    config = PipelineConfig(
        llm=LLMConfig(provider=LLMProviderType.MOCK),
        asr=ASRConfig(provider=ASRProviderType.MOCK),
        tts=TTSConfig(provider=TTSProviderType.MOCK),
    )
    pipeline = VoiceAvatarPipeline(config=config)

    events = []
    async for event in pipeline.process_user_turn(text_input="你好！"):
        events.append(event)

    types = [e.event_type for e in events]
    assert "text_chunk" in types
    assert "audio_chunk" in types
    assert "metrics" in types

    metrics_event = next(e for e in events if e.event_type == "metrics")
    payload = metrics_event.payload
    assert payload["turn_latency_ms"] > 0
    assert payload["ttft_ms"] > 0
    assert payload["chunks_count"] >= 1


@pytest.mark.asyncio
async def test_pipeline_interruption():
    config = PipelineConfig(
        llm=LLMConfig(provider=LLMProviderType.MOCK),
        asr=ASRConfig(provider=ASRProviderType.MOCK),
        tts=TTSConfig(provider=TTSProviderType.MOCK),
    )
    pipeline = VoiceAvatarPipeline(config=config)
    events = []
    pipeline.trigger_interrupt()
    async for event in pipeline.process_user_turn(text_input="你好！", reset_interrupt=False):
        events.append(event)
    types = [e.event_type for e in events]
    assert "interrupted" in types or len([e for e in events if e.event_type == "audio_chunk"]) == 0

@pytest.mark.asyncio
async def test_pipeline_audio_input_turn():
    import numpy as np
    config = PipelineConfig(
        llm=LLMConfig(provider=LLMProviderType.MOCK),
        asr=ASRConfig(provider=ASRProviderType.MOCK),
        tts=TTSConfig(provider=TTSProviderType.MOCK),
    )
    pipeline = VoiceAvatarPipeline(config=config)

    t = np.linspace(0, 0.2, 3200, endpoint=False)
    pcm_audio = (0.3 * np.sin(2 * np.pi * 440 * t) * 32767).astype(np.int16).tobytes()

    events = []
    async for event in pipeline.process_user_turn(audio_input=pcm_audio):
        events.append(event)

    types = [e.event_type for e in events]
    assert "user_transcription" in types
    assert "audio_chunk" in types
    assert "metrics" in types

    metrics_event = next(e for e in events if e.event_type == "metrics")
    payload = metrics_event.payload
    assert payload["asr_latency_ms"] > 0
    assert payload["turn_latency_ms"] > 0
