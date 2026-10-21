"""ASR (Automatic Speech Recognition) interface definition."""

from abc import ABC, abstractmethod


class BaseASR(ABC):
    """Abstract interface for speech-to-text models."""

    @abstractmethod
    async def transcribe(self, audio_data: bytes, sample_rate: int = 16000) -> str:
        """Transcribe raw PCM or audio container bytes into text."""
        pass
