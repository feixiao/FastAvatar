"""Benchmark runner for FastAvatar Voice Pipeline.

Measures Time to First Audio (TTFA / turn latency), TTFT, and TTS synthesis latency
deterministically under a fixed workload.
"""

import asyncio
import statistics
import time
import numpy as np

from src.config import (
    PipelineConfig,
    LLMConfig,
    ASRConfig,
    TTSConfig,
    LLMProviderType,
    ASRProviderType,
    TTSProviderType,
)
from src.pipeline import VoiceAvatarPipeline


def create_deterministic_audio_input(duration_sec: float = 1.0, sample_rate: int = 16000) -> bytes:
    """Generate fixed deterministic PCM waveform as simulated user speech."""
    np.random.seed(42)
    t = np.linspace(0, duration_sec, int(sample_rate * duration_sec), endpoint=False)
    # Synthetic speech frequency components
    signal = 0.3 * np.sin(2 * np.pi * 220 * t) + 0.2 * np.sin(2 * np.pi * 440 * t)
    pcm = (signal * 32767).astype(np.int16).tobytes()
    return pcm


async def run_benchmark():
    config = PipelineConfig(
        min_chunk_length=5,
        max_chunk_length=35,
        llm=LLMConfig(
            provider=LLMProviderType.MOCK,
            system_prompt="你是一个可爱的语音数字人助手，回答亲切简洁。",
        ),
        asr=ASRConfig(provider=ASRProviderType.MOCK),
        tts=TTSConfig(provider=TTSProviderType.MOCK),
    )

    pipeline = VoiceAvatarPipeline(config=config)
    audio_fixture = create_deterministic_audio_input()

    # 1. Warm-up run
    async for _ in pipeline.process_user_turn(audio_input=audio_fixture):
        pass

    # 2. Benchmark runs
    num_runs = 5
    turn_latencies = []
    ttft_latencies = []
    tts_latencies = []
    total_durations = []

    for run_idx in range(num_runs):
        start_t = time.perf_counter()
        turn_metrics = None

        async for event in pipeline.process_user_turn(audio_input=audio_fixture):
            if event.event_type == "metrics":
                turn_metrics = event.payload

        total_t = (time.perf_counter() - start_t) * 1000.0

        if turn_metrics:
            turn_latencies.append(turn_metrics["turn_latency_ms"])
            ttft_latencies.append(turn_metrics["ttft_ms"])
            tts_latencies.append(turn_metrics["tts_first_chunk_ms"])
            total_durations.append(total_t)

    # Use median for stable benchmarking
    median_turn_latency = statistics.median(turn_latencies)
    median_ttft = statistics.median(ttft_latencies)
    median_tts = statistics.median(tts_latencies)
    median_total = statistics.median(total_durations)

    print(f"--- Benchmark Results ({num_runs} runs) ---")
    print(f"Turn Latency (TTFA): {median_turn_latency:.2f} ms")
    print(f"Time to First Token (TTFT): {median_ttft:.2f} ms")
    print(f"TTS First Chunk: {median_tts:.2f} ms")
    print(f"Total Duration: {median_total:.2f} ms")

    # Autoresearch metric outputs (lower is better for latency)
    print(f"METRIC turn_latency_ms={median_turn_latency:.2f}")
    print(f"METRIC ttft_ms={median_ttft:.2f}")
    print(f"METRIC tts_first_chunk_ms={median_tts:.2f}")
    print(f"METRIC total_duration_ms={median_total:.2f}")


if __name__ == "__main__":
    asyncio.run(run_benchmark())
