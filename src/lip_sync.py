"""Lip Sync and Audio Envelope processor for Digital Avatar video/animation bridge.

Computes mouth opening (visemes / blendshapes) from audio waveform chunks,
enabling Live2D, 3D VRM, or 2D Talking Head video rendering without refactoring
the core audio pipeline.
"""

from typing import Dict, List, Tuple
import numpy as np


class LipSyncExtractor:
    """Extracts mouth movement parameters (amplitude, viseme classification) from PCM audio."""

    def __init__(self, sample_rate: int = 16000, frame_size_ms: int = 20):
        self.sample_rate = sample_rate
        self.frame_length = int(sample_rate * (frame_size_ms / 1000.0))
        self.hanning = np.hanning(self.frame_length)
        freqs = np.fft.rfftfreq(self.frame_length, 1.0 / self.sample_rate)
        self.low_mask = (freqs >= 200) & (freqs < 800)
        self.mid_mask = (freqs >= 800) & (freqs < 2200)
        self.high_mask = (freqs >= 2200) & (freqs < 4000)
    def extract_lip_sync(self, pcm_bytes: bytes) -> Tuple[List[float], List[Dict[str, float]]]:
        """Extract mouth openness envelope and vowel visemes in a single efficient pass."""
        if not pcm_bytes:
            return [], []

        audio = np.frombuffer(pcm_bytes, dtype=np.int16).astype(np.float32) / 32768.0
        if len(audio) == 0:
            return [], []

        rms_frames: List[float] = []
        viseme_frames: List[Dict[str, float]] = []

        n_usable = len(audio) - self.frame_length + 1
        for i in range(0, n_usable, self.frame_length):
            frame = audio[i : i + self.frame_length]
            rms = float(np.sqrt(np.mean(frame**2)))
            normalized = min(1.0, rms * 4.0)
            rms_frames.append(round(normalized, 3))

            if rms < 0.01:
                viseme_frames.append({"A": 0.0, "I": 0.0, "U": 0.0, "E": 0.0, "O": 0.0})
            else:
                spectrum = np.abs(np.fft.rfft(frame * self.hanning))
                low_energy = float(np.sum(spectrum[self.low_mask]))
                mid_energy = float(np.sum(spectrum[self.mid_mask]))
                high_energy = float(np.sum(spectrum[self.high_mask]))
                total = low_energy + mid_energy + high_energy + 1e-6

                openness = min(1.0, rms * 4.5)
                viseme_frames.append({
                    "A": round(min(1.0, openness * (mid_energy / total * 1.5)), 3),
                    "I": round(min(1.0, openness * (high_energy / total * 1.8)), 3),
                    "U": round(min(1.0, openness * (low_energy / total * 1.6)), 3),
                    "E": round(min(1.0, openness * ((mid_energy + high_energy) / total * 0.9)), 3),
                    "O": round(min(1.0, openness * ((low_energy + mid_energy) / total * 0.9)), 3),
                })

        rem = len(audio) % self.frame_length
        if rem > 0:
            rem_frame = audio[len(audio) - rem :]
            rms_rem = float(np.sqrt(np.mean(rem_frame**2)))
            rms_frames.append(round(min(1.0, rms_rem * 4.0), 3))

        return rms_frames, viseme_frames

    def compute_rms_envelope(self, pcm_bytes: bytes) -> List[float]:
        """Compute normalized RMS energy per frame (0.0 to 1.0) for mouth opening amplitude."""
        rms, _ = self.extract_lip_sync(pcm_bytes)
        return rms

    def compute_visemes(self, pcm_bytes: bytes) -> List[Dict[str, float]]:
        """Compute estimated 5 basic vowel visemes (A, I, U, E, O) per frame using spectral bands."""
        _, vis = self.extract_lip_sync(pcm_bytes)
        return vis
