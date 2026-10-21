"""Energy-based Voice Activity Detection (VAD).

Detects voice presence, start-of-speech, and end-of-speech silence windows.
"""

from typing import List, Tuple
import numpy as np


class EnergyVAD:
    """Fast, lightweight energy-based VAD for raw 16-bit 16kHz PCM audio."""

    def __init__(
        self,
        sample_rate: int = 16000,
        frame_ms: int = 20,
        energy_threshold: float = 0.02,
        silence_duration_ms: int = 600,
    ):
        self.sample_rate = sample_rate
        self.frame_length = int(sample_rate * (frame_ms / 1000.0))
        self.energy_threshold = energy_threshold
        self._energy_threshold_sq = float((energy_threshold * 32768.0) ** 2)
        self.silence_frames_threshold = int(silence_duration_ms / frame_ms)
        self.silence_count = 0
        self.speaking = False

    def reset(self):
        self.silence_count = 0
        self.speaking = False

    def is_speech_frame(self, pcm_frame: bytes) -> bool:
        """Evaluate whether a single PCM frame contains speech."""
        if len(pcm_frame) < 4:
            return False
        audio = np.frombuffer(pcm_frame, dtype=np.int16)
        mean_sq = float(np.mean(audio.astype(np.float64) ** 2))
        return mean_sq >= self._energy_threshold_sq

    def process_frame(self, pcm_frame: bytes) -> Tuple[bool, bool]:
        """Process a frame.
        
        Returns:
            (is_speaking: bool, speech_ended: bool)
        """
        has_voice = self.is_speech_frame(pcm_frame)

        if has_voice:
            self.silence_count = 0
            self.speaking = True
            return True, False
        else:
            if self.speaking:
                self.silence_count += 1
                if self.silence_count >= self.silence_frames_threshold:
                    self.speaking = False
                    self.silence_count = 0
                    return False, True  # Speech ended event!
                return True, False
            return False, False
