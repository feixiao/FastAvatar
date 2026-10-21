"""Sherpa-ONNX local offline TTS provider (VITS / Kokoro)."""

from typing import Optional
from src.tts.base import BaseTTS


class SherpaOnnxTTS(BaseTTS):
    """Local offline TTS synthesis via Sherpa-ONNX."""

    def __init__(
        self,
        vits_model: Optional[str] = None,
        vits_lexicon: Optional[str] = None,
        vits_tokens: Optional[str] = None,
        vits_data_dir: Optional[str] = None,
        speaker_id: int = 0,
        speed: float = 1.0,
    ):
        self.vits_model = vits_model
        self.vits_lexicon = vits_lexicon
        self.vits_tokens = vits_tokens
        self.vits_data_dir = vits_data_dir
        self.speaker_id = speaker_id
        self.speed = speed
        self._tts = None

    def _ensure_tts(self):
        if self._tts is not None:
            return
        if self.vits_model:
            import sherpa_onnx
            self._tts = sherpa_onnx.OfflineTts(
                vits_model=self.vits_model,
                vits_lexicon=self.vits_lexicon or "",
                vits_tokens=self.vits_tokens or "",
                vits_data_dir=self.vits_data_dir or "",
                num_threads=2,
            )

    async def synthesize(self, text: str) -> bytes:
        self._ensure_tts()
        if self._tts is None:
            return b""
        audio = self._tts.generate(text, sid=self.speaker_id, speed=self.speed)
        # Convert float samples (-1..1) to int16 PCM bytes
        import numpy as np
        samples = (np.array(audio.samples) * 32767).clip(-32768, 32767).astype(np.int16)
        return samples.tobytes()
