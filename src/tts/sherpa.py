"""Sherpa-ONNX local offline TTS provider (VITS / Kokoro)."""

from pathlib import Path
import asyncio
import numpy as np
from typing import Optional
from src.tts.base import BaseTTS


def _find_default_vits() -> tuple[Optional[str], Optional[str], Optional[str], Optional[str]]:
    candidate_roots = [
        Path("models"),
        Path(__file__).resolve().parent.parent.parent / "models",
    ]
    for root in candidate_roots:
        if not root.exists():
            continue
        for candidate in [
            root / "vits-zh-aishell3",
            root / "vits",
        ]:
            if candidate.is_dir():
                model = candidate / "vits-aishell3.onnx"
                tokens = candidate / "tokens.txt"
                lexicon = candidate / "lexicon.txt"
                data_dir = candidate / "espeak-ng-data"
                if model.exists() and tokens.exists():
                    return (
                        str(model),
                        str(lexicon) if lexicon.exists() else None,
                        str(tokens),
                        str(data_dir) if data_dir.exists() else None,
                    )
    return None, None, None, None


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
        if not self.vits_model:
            model, lexicon, tokens, data_dir = _find_default_vits()
            if model:
                self.vits_model = model
                self.vits_lexicon = self.vits_lexicon or lexicon
                self.vits_tokens = self.vits_tokens or tokens
                self.vits_data_dir = self.vits_data_dir or data_dir

        if self.vits_model:
            import sherpa_onnx
            vits_config = sherpa_onnx.OfflineTtsVitsModelConfig(
                model=self.vits_model,
                lexicon=self.vits_lexicon or "",
                tokens=self.vits_tokens or "",
                data_dir=self.vits_data_dir or "",
            )
            model_config = sherpa_onnx.OfflineTtsModelConfig(
                vits=vits_config,
                num_threads=2,
            )
            tts_config = sherpa_onnx.OfflineTtsConfig(model=model_config)
            self._tts = sherpa_onnx.OfflineTts(tts_config)
    async def synthesize(self, text: str) -> bytes:
        self._ensure_tts()
        if self._tts is None:
            return b""
        audio = await asyncio.to_thread(self._tts.generate, text, sid=self.speaker_id, speed=self.speed)
        # Convert float samples (-1..1) to int16 PCM bytes
        samples = (np.array(audio.samples) * 32767).clip(-32768, 32767).astype(np.int16)
        return samples.tobytes()
