"""Sherpa-ONNX local offline ASR implementation."""

import io
import wave
from typing import Optional
from src.asr.base import BaseASR


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
        import sherpa_onnx
        if self.tokens and self.model:
            self._recognizer = sherpa_onnx.OfflineRecognizer.from_sense_voice(
                model=self.model,
                tokens=self.tokens,
                num_threads=self.num_threads,
                use_itn=True,
            )
        else:
            # Fallback or placeholder when paths not configured
            self._recognizer = None

    async def transcribe(self, audio_data: bytes, sample_rate: int = 16000) -> str:
        self._ensure_recognizer()
        if self._recognizer is None:
            # Safe fallback if model weights are not yet downloaded locally
            return "本地Sherpa-ONNX模型未加载，请配置模型权重路径。"

        import numpy as np
        audio = np.frombuffer(audio_data, dtype=np.int16).astype(np.float32) / 32768.0
        stream = self._recognizer.create_stream()
        stream.accept_waveform(sample_rate, audio)
        self._recognizer.decode_stream(stream)
        return stream.result.text
