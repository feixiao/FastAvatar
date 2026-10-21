"""Edge-TTS provider for natural neural voice streaming."""

import io
from typing import AsyncIterator
import edge_tts
import numpy as np
import soundfile as sf
from src.tts.base import BaseTTS


def _mp3_to_pcm16(mp3_bytes: bytes, target_sr: int = 16000) -> bytes:
    if not mp3_bytes:
        return b""
    try:
        audio_data, sr = sf.read(io.BytesIO(mp3_bytes), dtype="int16")
    except Exception:
        return mp3_bytes
    if audio_data.ndim > 1:
        audio_data = audio_data.mean(axis=1).astype(np.int16)
    if sr != target_sr:
        target_len = int(round(len(audio_data) * target_sr / sr))
        orig_indices = np.linspace(0, len(audio_data) - 1, target_len)
        resampled = np.interp(orig_indices, np.arange(len(audio_data)), audio_data.astype(np.float32))
        audio_data = np.clip(resampled, -32768, 32767).astype(np.int16)
    return audio_data.tobytes()


class EdgeTTS(BaseTTS):
    """Cloud TTS using Microsoft Edge neural voices."""

    def __init__(self, voice: str = "zh-CN-XiaoxiaoNeural", rate: str = "+0%", pitch: str = "+0Hz"):
        self.voice = voice
        self.rate = rate
        self.pitch = pitch

    async def synthesize(self, text: str) -> bytes:
        if not text.strip():
            return b""
        communicate = edge_tts.Communicate(text, voice=self.voice, rate=self.rate, pitch=self.pitch)
        chunks = []
        async for chunk in communicate.stream():
            if chunk["type"] == "audio":
                chunks.append(chunk["data"])
        return _mp3_to_pcm16(b"".join(chunks))

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        if not text.strip():
            return
        communicate = edge_tts.Communicate(text, voice=self.voice, rate=self.rate, pitch=self.pitch)
        async for chunk in communicate.stream():
            if chunk["type"] == "audio":
                yield chunk["data"]
