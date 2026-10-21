"""TTS (Text to Speech) interface."""

from abc import ABC, abstractmethod
from typing import AsyncIterator


class BaseTTS(ABC):
    """Abstract interface for speech synthesis engines."""

    @abstractmethod
    async def synthesize(self, text: str) -> bytes:
        """Synthesize text into complete audio bytes (PCM or MP3/WAV)."""
        pass

    async def synthesize_stream(self, text: str) -> AsyncIterator[bytes]:
        """Stream synthesized audio chunks. Default implementation yields single chunk."""
        data = await self.synthesize(text)
        if data:
            yield data


class FallbackTTS(BaseTTS):
    """Tries primary TTS engine, falling back to secondary if primary fails or raises."""

    def __init__(self, primary: BaseTTS, secondary: BaseTTS):
        self.primary = primary
        self.secondary = secondary

    async def synthesize(self, text: str) -> bytes:
        try:
            audio = await self.primary.synthesize(text)
            if audio:
                return audio
        except Exception:
            pass
        return await self.secondary.synthesize(text)
