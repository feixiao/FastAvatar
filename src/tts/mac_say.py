"""Native macOS offline TTS using the built-in `say` command."""

import asyncio
import os
import tempfile
from typing import Optional
from src.tts.base import BaseTTS


class MacSayTTS(BaseTTS):
    """Offline TTS engine using macOS built-in speech synthesis (e.g., Ting-Ting)."""

    def __init__(self, voice: str = "Ting-Ting", rate_wpm: int = 200):
        self.voice = voice
        self.rate_wpm = rate_wpm

    async def synthesize(self, text: str) -> bytes:
        if not text.strip():
            return b""

        with tempfile.NamedTemporaryFile(suffix=".aiff", delete=False) as tmp_aiff:
            aiff_path = tmp_aiff.name

        with tempfile.NamedTemporaryFile(suffix=".pcm", delete=False) as tmp_pcm:
            pcm_path = tmp_pcm.name

        try:
            # 1. Generate AIFF using macOS native say
            proc = await asyncio.create_subprocess_exec(
                "say",
                "-v",
                self.voice,
                "-r",
                str(self.rate_wpm),
                "-o",
                aiff_path,
                text,
            )
            await proc.communicate()

            # 2. Convert AIFF to 16kHz 16-bit raw PCM using ffmpeg
            proc_conv = await asyncio.create_subprocess_exec(
                "ffmpeg",
                "-y",
                "-i",
                aiff_path,
                "-ar",
                "16000",
                "-ac",
                "1",
                "-f",
                "s16le",
                pcm_path,
                stdout=asyncio.subprocess.DEVNULL,
                stderr=asyncio.subprocess.DEVNULL,
            )
            await proc_conv.communicate()

            if os.path.exists(pcm_path):
                with open(pcm_path, "rb") as f:
                    return f.read()
            return b""
        finally:
            if os.path.exists(aiff_path):
                os.remove(aiff_path)
            if os.path.exists(pcm_path):
                os.remove(pcm_path)
