package main

import (
	"context"
	"testing"
	"time"

	"avatar/internal/asr"
	"avatar/internal/config"
	"avatar/internal/engine"
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
	var spoken []string
	app.pipeline.SetCallbacks(func(s engine.State) {
		t.Logf("State -> %s", s)
	}, func(speech string) {
		t.Logf("Speech -> %s", speech)
		spoken = append(spoken, speech)
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	err := app.pipeline.Chat(ctx, "你好，请回答一句话：你是谁？")
	if err != nil {
		t.Fatalf("Live Chat error: %v", err)
	}
	t.Logf("Spoken count: %d, chunks: %v", len(spoken), spoken)
	if len(spoken) == 0 {
		t.Fatal("expected at least one speech chunk")
	}
}
