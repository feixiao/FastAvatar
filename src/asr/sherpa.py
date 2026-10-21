"""Sherpa-ONNX local offline ASR implementation."""

import io
import wave
from pathlib import Path
from typing import Optional
from src.asr.base import BaseASR


def _find_default_sense_voice() -> tuple[Optional[str], Optional[str]]:
    candidate_roots = [
        Path("models"),
        Path(__file__).resolve().parent.parent.parent / "models",
    ]
    for root in candidate_roots:
        if not root.exists():
            continue
        for candidate in [
            root / "sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17",
            root / "sense-voice",
        ]:
            if candidate.is_dir():
                model = candidate / "model.int8.onnx"
                if not model.exists():
                    model = candidate / "model.onnx"
                tokens = candidate / "tokens.txt"
                if model.exists() and tokens.exists():
                    return str(model), str(tokens)
    return None, None


class SherpaOnnxASR(BaseASR):
    """Local offline speech recognition using Sherpa-ONNX (SenseVoice / Paraformer / Whisper)."""

    def __init__(
        self,
        tokens: Optional[str] = None,
        model: Optional[str] = None,
        num_threads: int = 2,
    ):
        self.tokens = tokens
        self.model = model
        self.num_threads = num_threads
        self._recognizer = None

    def _ensure_recognizer(self):
        if self._recognizer is not None:
            return
        if not (self.tokens and self.model):
            auto_model, auto_tokens = _find_default_sense_voice()
            if auto_model and auto_tokens:
                self.model = auto_model
                self.tokens = auto_tokens

        if self.tokens and self.model:
            import sherpa_onnx
            self._recognizer = sherpa_onnx.OfflineRecognizer.from_sense_voice(
                model=self.model,
                tokens=self.tokens,
                num_threads=self.num_threads,
                use_itn=True,
            )
        else:
            self._recognizer = None

    async def transcribe(self, audio_data: bytes, sample_rate: int = 16000) -> str:
        self._ensure_recognizer()
        if self._recognizer is None:
            # Safe fallback if model weights are not yet downloaded locally
            return "本地Sherpa-ONNX模型未加载，请配置模型权重路径。"

        import io
        import soundfile as sf
        import numpy as np

        try:
            audio, sr = sf.read(io.BytesIO(audio_data), dtype="float32")
            if audio.ndim > 1:
                audio = audio.mean(axis=1)
        except Exception:
            audio = np.frombuffer(audio_data, dtype=np.int16).astype(np.float32) / 32768.0
            sr = sample_rate

        stream = self._recognizer.create_stream()
        stream.accept_waveform(sr, audio)
        return stream.result.text
