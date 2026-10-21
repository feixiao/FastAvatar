"""Deterministic Mock TTS for unit tests and reproducible latency benchmarks."""

import asyncio
from typing import AsyncIterator
import numpy as np
from src.tts.base import BaseTTS


class MockTTS(BaseTTS):
    """Generates synthetic PCM audio bytes with deterministic timing."""

    def __init__(
        self,
        sample_rate: int = 16000,
        synthesis_delay_ms: float = 20.0,
        ms_per_char: float = 80.0,
    ):
        self.sample_rate = sample_rate
        self.synthesis_delay_ms = synthesis_delay_ms
        self.ms_per_char = ms_per_char

    async def synthesize(self, text: str) -> bytes:
        if self.synthesis_delay_ms > 0:
            await asyncio.sleep(self.synthesis_delay_ms / 1000.0)

        # Generate smooth synthetic speech-like tone burst
        duration_sec = max(0.1, len(text) * (self.ms_per_char / 1000.0))
        t = np.linspace(0, duration_sec, int(self.sample_rate * duration_sec), endpoint=False)
        # 440Hz fundamental with harmonic modulation
        waveform = 0.5 * np.sin(2 * np.pi * 440 * t) * (1.0 + 0.2 * np.sin(2 * np.pi * 5 * t))
        pcm16 = (waveform * 32767).astype(np.int16).tobytes()
        return pcm16

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        pcm16 = await self.synthesize(text)
        # Yield in 40ms packets (640 samples = 1280 bytes at 16kHz 16-bit)
        packet_size = int(self.sample_rate * 0.04) * 2
        for i in range(0, len(pcm16), packet_size):
            yield pcm16[i : i + packet_size]
