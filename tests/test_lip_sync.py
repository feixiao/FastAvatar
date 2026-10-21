"""Unit tests for lip sync and viseme extraction."""

import numpy as np
from src.lip_sync import LipSyncExtractor


def test_lip_sync_rms():
    extractor = LipSyncExtractor(sample_rate=16000, frame_size_ms=20)
    # Generate 100ms of audio
    t = np.linspace(0, 0.1, 1600, endpoint=False)
    pcm = (0.5 * np.sin(2 * np.pi * 440 * t) * 32767).astype(np.int16).tobytes()

    frames = extractor.compute_rms_envelope(pcm)
    assert len(frames) == 5  # 100ms / 20ms = 5 frames
    for val in frames:
        assert 0.0 <= val <= 1.0
        assert val > 0.05  # Sine wave has non-zero energy


def test_lip_sync_visemes():
    extractor = LipSyncExtractor(sample_rate=16000, frame_size_ms=20)
    t = np.linspace(0, 0.04, 640, endpoint=False)
    pcm = (0.4 * np.sin(2 * np.pi * 1000 * t) * 32767).astype(np.int16).tobytes()

    visemes = extractor.compute_visemes(pcm)
    assert len(visemes) >= 1
    for v in visemes:
        assert all(k in v for k in ["A", "I", "U", "E", "O"])
        assert all(0.0 <= v[k] <= 1.0 for k in ["A", "I", "U", "E", "O"])

def test_lip_sync_silent_and_empty():
    extractor = LipSyncExtractor(sample_rate=16000, frame_size_ms=20)
    assert extractor.extract_lip_sync(b"") == ([], [])
    assert extractor.compute_rms_envelope(b"") == []
    assert extractor.compute_visemes(b"") == []

    # 40ms of silence
    silent_pcm = np.zeros(640, dtype=np.int16).tobytes()
    rms, vis = extractor.extract_lip_sync(silent_pcm)
    assert len(rms) == 2
    assert all(r == 0.0 for r in rms)
    assert all(v["A"] == 0.0 and v["O"] == 0.0 for v in vis)
