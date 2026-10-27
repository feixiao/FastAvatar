package task

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"avatar/internal/config"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
	"avatar/internal/tts"
)

func TestTaskRunnerExecution(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		tokens := []string{"你", "好", "，", "世界", "！"}
		for _, tok := range tokens {
			chunk := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", tok)
			_, _ = fmt.Fprint(w, chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	bondMgr, _ := persona.NewBondManager(filepath.Join(tmpDir, "bnd.json"))

	mockTTS := tts.NewMockSynthesizer()
	coord := orchestrator.NewCoordinator("test-sess")
	llmClient := llm.NewClient(ts.URL, "key")
	llmCfg := config.LLMConfig{
		Model: "test-model",
	}

	runner := NewRunner(llmCfg, llmClient, mockTTS, vault, personaMgr, bondMgr, coord)

	var speechChunks []string
	runner.SetSpeechChunkCallback(func(s string) {
		speechChunks = append(speechChunks, s)
	})

	var emittedEvents []orchestrator.TaskEvent
	emit := func(e orchestrator.TaskEvent) {
		emittedEvents = append(emittedEvents, e)
	}

	sampleTask := orchestrator.Task{
		TaskID:       "task-1",
		TurnID:       "turn-1",
		SessionID:    "test-sess",
		GoalSnapshot: "你好",
	}

	ctx := context.Background()
	err := runner.Run(ctx, sampleTask, emit)
	if err != nil {
		t.Fatalf("Runner.Run returned error: %v", err)
	}

	if len(mockTTS.Spoken) == 0 {
		t.Fatal("expected TTS speech chunks to be spoken")
	}

	if len(speechChunks) == 0 {
		t.Fatal("expected speech chunk callbacks")
	}

	if coord.State() != orchestrator.StateIdle {
		t.Fatalf("expected coordinator state Idle, got %s", coord.State())
	}

	if len(emittedEvents) == 0 {
		t.Fatal("expected emitted task events")
	}

	lastEvent := emittedEvents[len(emittedEvents)-1]
	if lastEvent.State != orchestrator.TaskStateCompleted {
		t.Fatalf("expected final event state Completed, got %s", lastEvent.State)
	}

	history := runner.History()
	if len(history) != 2 {
		t.Fatalf("expected 2 history messages (user, assistant), got %d", len(history))
	}
}
