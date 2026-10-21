"""Deterministic Mock ASR for testing and reproducible benchmarking."""

import asyncio
from typing import Optional
from src.asr.base import BaseASR


class MockASR(BaseASR):
    """Returns fixed or simulated transcription without external network or heavy GPU."""

    def __init__(self, fixed_text: str = "你好呀，很高兴认识你！今天有什么可以帮你的？", simulated_delay_ms: float = 15.0):
        self.fixed_text = fixed_text
        self.simulated_delay_ms = simulated_delay_ms

    async def transcribe(self, audio_data: bytes, sample_rate: int = 16000) -> str:
        if self.simulated_delay_ms > 0:
            await asyncio.sleep(self.simulated_delay_ms / 1000.0)
        if not audio_data:
            return ""
        return self.fixed_text
