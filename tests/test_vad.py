"""Unit tests for EnergyVAD voice activity detection."""

import numpy as np
from src.vad.energy_vad import EnergyVAD


def test_vad_silence_detection():
    vad = EnergyVAD(sample_rate=16000, frame_ms=20, energy_threshold=0.02)
    silence = np.zeros(320, dtype=np.int16).tobytes()
    assert not vad.is_speech_frame(silence)
    assert not vad.is_speech_frame(b"")
    assert not vad.is_speech_frame(b"12")


def test_vad_speech_detection():
    vad = EnergyVAD(sample_rate=16000, frame_ms=20, energy_threshold=0.02)
    speech = (np.sin(np.linspace(0, 5, 320)) * 10000).astype(np.int16).tobytes()
    assert vad.is_speech_frame(speech)


def test_vad_state_transition_and_silence_window():
    # 20ms frames, silence_duration_ms=60ms -> 3 frames to end speech
    vad = EnergyVAD(sample_rate=16000, frame_ms=20, energy_threshold=0.02, silence_duration_ms=60)
    speech = (np.sin(np.linspace(0, 5, 320)) * 10000).astype(np.int16).tobytes()
    silence = np.zeros(320, dtype=np.int16).tobytes()

    # Initial state
    spk, end = vad.process_frame(silence)
    assert not spk and not end

    # User starts speaking
    spk, end = vad.process_frame(speech)
    assert spk and not end

    # User continues speaking
    spk, end = vad.process_frame(speech)
    assert spk and not end

    # Silence frame 1
    spk, end = vad.process_frame(silence)
    assert spk and not end

    # Silence frame 2
    spk, end = vad.process_frame(silence)
    assert spk and not end

    # Silence frame 3 -> reaches threshold (60ms), end of speech!
    spk, end = vad.process_frame(silence)
    assert not spk and end

    # Reset
    vad.reset()
    assert not vad.speaking
    assert vad.silence_count == 0
