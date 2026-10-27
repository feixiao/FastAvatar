package asr

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestMockEngine(t *testing.T) {
	mock := NewMockEngine()

	var partialText, finalText string
	var vadSpeaking bool
	var mu sync.Mutex

	mock.SetCallbacks(
		func(p string) {
			mu.Lock()
			partialText = p
			mu.Unlock()
		},
		func(f string) {
			mu.Lock()
			finalText = f
			mu.Unlock()
		},
		func(s bool, energy float64) {
			mu.Lock()
			vadSpeaking = s
			mu.Unlock()
		},
		nil,
	)

	ctx := context.Background()
	if err := mock.Start(ctx); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	_ = mock.SetMode("hands_free")
	_ = mock.SetSpeaking(true)

	mock.SimulateSpeech("你好", "你好世界")

	mu.Lock()
	defer mu.Unlock()
	if partialText != "你好" {
		t.Errorf("expected partial '你好', got '%s'", partialText)
	}
	if finalText != "你好世界" {
		t.Errorf("expected final '你好世界', got '%s'", finalText)
	}
	if vadSpeaking != false {
		t.Errorf("expected vadSpeaking false at end, got %v", vadSpeaking)
	}

	if err := mock.Stop(); err != nil {
		t.Errorf("Stop failed: %v", err)
	}
}

func TestNativeMacEngineLifecycle(t *testing.T) {
	engine := NewNativeMacEngine("zh-CN", "hands_free")
	bin, err := engine.EnsureBinary()
	if err != nil {
		t.Skipf("Skipping NativeMacEngine test: binary not available (%v)", err)
		return
	}
	if bin == "" {
		t.Skip("binary path empty")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = engine.Start(ctx)
	if err != nil {
		t.Fatalf("Start NativeMacEngine failed: %v", err)
	}

	// Send commands
	_ = engine.SetSpeaking(true)
	_ = engine.SetMode("ptt")
	_ = engine.PTTStart()
	_ = engine.PTTStop()

	// Stop cleanly
	err = engine.Stop()
	if err != nil {
		t.Errorf("Stop NativeMacEngine failed: %v", err)
	}
}
