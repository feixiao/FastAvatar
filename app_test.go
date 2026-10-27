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
