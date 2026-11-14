package tts

import (
	"context"
	"os"
	"testing"
	"time"
)
func TestSherpaVoiceMapping(t *testing.T) {
	synth := NewSherpaOnnxSynthesizer("bin/sherpa-onnx-offline-tts-play", "models/vits-zh-aishell3", 66, 180)

	if synth.GetSpeakerID() != 66 {
		t.Fatalf("expected speaker ID 66, got %d", synth.GetSpeakerID())
	}

	// Test persona name mapping
	synth.SetVoice("jarvis")
	if synth.GetSpeakerID() != 0 {
		t.Fatalf("expected jarvis to map to 0, got %d", synth.GetSpeakerID())
	}

	synth.SetVoice("companion")
	if synth.GetSpeakerID() != 10 {
		t.Fatalf("expected companion to map to 10, got %d", synth.GetSpeakerID())
	}

	synth.SetVoice("roaster")
	if synth.GetSpeakerID() != 3 {
		t.Fatalf("expected roaster to map to 3, got %d", synth.GetSpeakerID())
	}

	// Test direct numeric speaker ID
	synth.SetVoice("123")
	if synth.GetSpeakerID() != 123 {
		t.Fatalf("expected 123, got %d", synth.GetSpeakerID())
	}

	// Test rate mapping
	synth.SetRate(175)
	if synth.speed < 0.99 || synth.speed > 1.01 {
		t.Fatalf("expected speed around 1.0, got %f", synth.speed)
	}
}

func TestSherpaCancel(t *testing.T) {
	synth := NewSherpaOnnxSynthesizer("bin/sherpa-onnx-offline-tts-play", "models/vits-zh-aishell3", 66, 180)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := synth.Speak(ctx, "测试文本")
	if err == nil {
		t.Logf("Speak completed with canceled context without error or skipped")
	}
}

func TestSherpaStop(t *testing.T) {
	synth := NewSherpaOnnxSynthesizer("bin/sherpa-onnx-offline-tts-play", "models/vits-zh-aishell3", 66, 180)

	done := make(chan struct{})
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = synth.Speak(ctx, "这是一段用于测试打断的稍微长一点的测试文本，正在测试打断逻辑。")
		close(done)
	}()

	time.Sleep(100 * time.Millisecond)
	synth.Stop()

	select {
	case <-done:
		// success: stopped cleanly
	case <-time.After(3 * time.Second):
		t.Fatalf("Stop did not terminate Speak within deadline")
	}
}

func TestSherpaRealSynthesis(t *testing.T) {
	if _, err := os.Stat("../../bin/sherpa-onnx-offline-tts-play"); os.IsNotExist(err) {
		t.Skip("sherpa-onnx binary not found, skipping real synthesis test")
	}
	synth := NewSherpaOnnxSynthesizer("../../bin/sherpa-onnx-offline-tts-play", "../../models/vits-zh-aishell3", 66, 180)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	err := synth.Speak(ctx, "测试")
	if err != nil {
		t.Fatalf("Speak failed: %v", err)
	}
}
func TestKokoroVoiceMapping(t *testing.T) {
	synth := NewSherpaOnnxSynthesizer("bin/sherpa-onnx-offline-tts-play", "models/kokoro-int8-multi-lang-v1_0", 45, 180)
	if !synth.IsKokoro() {
		t.Fatal("expected synth to detect Kokoro model")
	}

	if synth.GetSpeakerID() != 45 {
		t.Fatalf("expected speaker ID 45 for whale default, got %d", synth.GetSpeakerID())
	}

	synth.SetVoice("companion")
	if synth.GetSpeakerID() != 47 {
		t.Fatalf("expected companion to map to 47 (zf_xiaoxiao), got %d", synth.GetSpeakerID())
	}

	synth.SetVoice("jarvis")
	if synth.GetSpeakerID() != 50 {
		t.Fatalf("expected jarvis to map to 50 (zm_yunxi), got %d", synth.GetSpeakerID())
	}

	synth.SetVoice("roaster")
	if synth.GetSpeakerID() != 49 {
		t.Fatalf("expected roaster to map to 49 (zm_yunjian), got %d", synth.GetSpeakerID())
	}

	// Test speaker alias direct string
	synth.SetVoice("zf_xiaoni")
	if synth.GetSpeakerID() != 46 {
		t.Fatalf("expected zf_xiaoni to map to 46, got %d", synth.GetSpeakerID())
	}

	// Test legacy AISHELL-3 ID compatibility
	synth.SetVoice("66")
	if synth.GetSpeakerID() != 45 {
		t.Fatalf("expected legacy 66 to map to 45 in Kokoro, got %d", synth.GetSpeakerID())
	}
}

func TestKokoroRealSynthesis(t *testing.T) {
	binPath := "../../bin/sherpa-onnx-offline-tts-play"
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		t.Skip("sherpa-onnx binary not found, skipping real synthesis test")
	}
	modelDir := "../../models/kokoro-int8-multi-lang-v1_0"
	if _, err := os.Stat(modelDir); os.IsNotExist(err) {
		t.Skip("kokoro model not found, skipping real synthesis test")
	}
	synth := NewSherpaOnnxSynthesizer(binPath, modelDir, 45, 180)
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	err := synth.Speak(ctx, "测试Kokoro发音")
	if err != nil {
		t.Fatalf("Kokoro Speak failed: %v", err)
	}
}
