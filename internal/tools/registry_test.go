package tools

import (
	"context"
	"path/filepath"
	"testing"

	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
)

func TestRegistryToolExecutionAndIdempotency(t *testing.T) {
	reg := NewRegistry()

	calledCount := 0
	err := reg.Register(ToolDefinition{
		ID:          "counter_inc",
		Version:     "1.0",
		Description: "increment counter",
		Handler: func(ctx context.Context, params map[string]any) (any, error) {
			calledCount++
			return calledCount, nil
		},
	})
	if err != nil {
		t.Fatalf("failed to register tool: %v", err)
	}

	call := orchestrator.ToolCall{
		ToolID:         "counter_inc",
		Version:        "1.0",
		IdempotencyKey: "key-123",
	}

	// First execution
	res1, err := reg.Execute(context.Background(), call, "key-123")
	if err != nil || !res1.Success {
		t.Fatalf("expected successful execution, got res=%+v err=%v", res1, err)
	}
	if res1.Data != 1 {
		t.Fatalf("expected result 1, got %v", res1.Data)
	}

	// Second execution with same idempotency key should return cached result without calling handler again
	res2, err := reg.Execute(context.Background(), call, "key-123")
	if err != nil || !res2.Success {
		t.Fatalf("expected successful cached execution, got res=%+v err=%v", res2, err)
	}
	if res2.Data != 1 {
		t.Fatalf("expected cached result 1, got %v", res2.Data)
	}
	if calledCount != 1 {
		t.Fatalf("handler should only be called once, called %d times", calledCount)
	}
}

func TestRegistryBuiltInTools(t *testing.T) {
	tmpDir := t.TempDir()
	vault, err := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	if err != nil {
		t.Fatalf("create vault: %v", err)
	}
	personaMgr, err := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	if err != nil {
		t.Fatalf("create persona mgr: %v", err)
	}

	reg := NewRegistry()
	RegisterMemoryTools(reg, vault)
	RegisterPersonaTools(reg, personaMgr)

	ctx := context.Background()

	// 1. Test persona switch tool
	switchCall := orchestrator.ToolCall{
		ToolID:  "persona_switch",
		Version: "1.0",
		Params: map[string]any{
			"persona_id": "jarvis",
		},
	}
	res, err := reg.Execute(ctx, switchCall, "")
	if err != nil || !res.Success {
		t.Fatalf("persona switch failed: %+v, err: %v", res, err)
	}
	if personaMgr.GetActiveSlot().ID != "jarvis" {
		t.Fatalf("expected active slot jarvis, got %s", personaMgr.GetActiveSlot().ID)
	}

	// 2. Test memory store tool
	storeCall := orchestrator.ToolCall{
		ToolID:  "memory_store",
		Version: "1.0",
		Params: map[string]any{
			"category": "preference",
			"key":      "coffee",
			"value":    "ice americano",
		},
	}
	res, err = reg.Execute(ctx, storeCall, "")
	if err != nil || !res.Success {
		t.Fatalf("memory store failed: %+v, err: %v", res, err)
	}

	data := vault.GetData()
	found := false
	for _, item := range data.Profile {
		if item.Key == "coffee" && item.Value == "ice americano" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected profile item coffee to be stored in vault")
	}
}
