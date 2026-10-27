package main

import (
	"context"
	"testing"
	"time"

	"avatar/internal/asr"
	"avatar/internal/config"
	"avatar/internal/engine"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/persona"
	"avatar/internal/tts"
)

func TestAppVoiceIntegration(t *testing.T) {
	mockASR := asr.NewMockEngine()
	cfg := config.LoadConfig()

	vault, err := memory.NewVault(t.TempDir() + "/mem.json")
	if err != nil {
		t.Fatalf("NewVault failed: %v", err)
	}
	personaMgr, err := persona.NewManager(t.TempDir() + "/personas.json")
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}
	bondMgr, err := persona.NewBondManager(t.TempDir() + "/bond.json")
	if err != nil {
		t.Fatalf("NewBondManager failed: %v", err)
	}

	synth := tts.NewMockSynthesizer()
	pipe := engine.NewPipeline(cfg, synth, vault, personaMgr, bondMgr)

	app := &App{
		cfg:        cfg,
		pipeline:   pipe,
		vault:      vault,
		personaMgr: personaMgr,
		bondMgr:    bondMgr,
		asrEngine:  mockASR,
	}

	// Verify PTT calls
	if err := app.StartPTT(); err != nil {
		t.Errorf("StartPTT failed: %v", err)
	}
	if err := app.StopPTT(); err != nil {
		t.Errorf("StopPTT failed: %v", err)
	}

	// Verify mode toggle
	origMode := app.cfg.Mode
	newMode := app.ToggleMode()
	if newMode == string(origMode) {
		t.Errorf("expected mode toggle, got unchanged %s", newMode)
	}

	// Verify state and interrupts
	app.Interrupt()
	if s := app.GetState(); s != "idle" {
		t.Errorf("expected idle state, got %s", s)
	}

	// Clean shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	app.shutdown(ctx)
}

func TestLiveLLMConversation(t *testing.T) {
	app := NewApp()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if !llm.ProbeEndpoint(ctx, app.cfg.LLM.BaseURL, app.cfg.LLM.APIKey) {
		t.Skipf("skipping live test, no active LLM endpoint found at %s", app.cfg.LLM.BaseURL)
	}

	var spoken []string
	app.pipeline.SetCallbacks(func(s engine.State) {
		t.Logf("State -> %s", s)
	}, func(speech string) {
		t.Logf("Speech -> %s", speech)
		spoken = append(spoken, speech)
	}, nil, nil)

	err := app.pipeline.Chat(ctx, "你好，请回答一句话：你是谁？")
	if err != nil {
		t.Fatalf("Live Chat error: %v", err)
	}
	t.Logf("Spoken count: %d, chunks: %v", len(spoken), spoken)
	if len(spoken) == 0 {
		t.Fatal("expected at least one speech chunk")
	}
}

func TestApp_FastPathMockVehicleWorksWithoutLLM(t *testing.T) {
	app := NewApp()
	var spoken []string
	app.pipeline.SetCallbacks(nil, func(speech string) {
		spoken = append(spoken, speech)
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 车控快环不需要任何运行中的 LLM 服务，100% 本地完成
	err := app.pipeline.Chat(ctx, "把空调调到24度")
	if err != nil {
		t.Fatalf("fast-path vehicle command failed: %v", err)
	}

	if len(spoken) == 0 {
		t.Fatal("expected spoken confirmation for mock vehicle")
	}
	if app.pipeline.State() != engine.StateIdle {
		t.Fatalf("expected state idle, got %s", app.pipeline.State())
	}
}
