package interaction

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"avatar/internal/memory"
	"avatar/internal/mockvehicle"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
)

type testTrackingSynth struct {
	mu      sync.Mutex
	spoken  []string
	stopped bool
}

func (s *testTrackingSynth) Speak(ctx context.Context, text string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.spoken = append(s.spoken, text)
	return nil
}

func (s *testTrackingSynth) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped = true
}

type mockTaskRunner struct {
	called bool
	task   orchestrator.Task
}

func (m *mockTaskRunner) Run(ctx context.Context, task orchestrator.Task, emit func(orchestrator.TaskEvent)) error {
	m.called = true
	m.task = task
	emit(orchestrator.TaskEvent{
		TaskID:    task.TaskID,
		TurnID:    task.TurnID,
		SessionID: task.SessionID,
		State:     orchestrator.TaskStateCompleted,
		Result:    "mock completed",
	})
	return nil
}

func TestFastPathPersonaSwitch(t *testing.T) {
	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	mockTTS := &testTrackingSynth{}

	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(vault, personaMgr)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}

	manager := NewManager(coord, fp, player, runner)

	var capturedSpeech []string
	manager.SetSpeechChunkCallback(func(chunk string) {
		capturedSpeech = append(capturedSpeech, chunk)
	})

	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "切换成高效幕僚模式",
		Stability: orchestrator.StabilityFinal,
	}

	turnID, err := manager.Submit(context.Background(), input)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if turnID == "" {
		t.Fatal("expected non-empty turn ID")
	}

	if runner.called {
		t.Fatal("expected fast path to handle input without calling TaskRunner")
	}

	if len(mockTTS.spoken) == 0 || len(capturedSpeech) == 0 {
		t.Fatal("expected TTS and callback speech from fast-path confirmation")
	}
	if personaMgr.GetActiveSlot().ID != "jarvis" {
		t.Fatalf("expected active persona jarvis, got %s", personaMgr.GetActiveSlot().ID)
	}
	if coord.State() != orchestrator.StateIdle {
		t.Fatalf("expected coordinator state to return to Idle, got %s", coord.State())
	}
}

func TestNormalInputDelegatesToTaskRunner(t *testing.T) {
	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	mockTTS := &testTrackingSynth{}

	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(vault, personaMgr)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}

	manager := NewManager(coord, fp, player, runner)

	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "今天天气怎么样？",
		Stability: orchestrator.StabilityFinal,
	}

	turnID, err := manager.Submit(context.Background(), input)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if turnID == "" {
		t.Fatal("expected non-empty turn ID")
	}

	if !runner.called {
		t.Fatal("expected TaskRunner to be called for non-fastpath input")
	}
	if runner.task.GoalSnapshot != "今天天气怎么样？" {
		t.Fatalf("unexpected goal snapshot in task: %s", runner.task.GoalSnapshot)
	}
}

func TestInteractionInterrupt(t *testing.T) {
	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(nil, nil)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}

	manager := NewManager(coord, fp, player, runner)

	// Simulate coordinator in speaking state
	coord.SetState(orchestrator.StateSpeaking)

	err := manager.Interrupt(context.Background(), coord.SessionID(), "")
	if err != nil {
		t.Fatalf("Interrupt error: %v", err)
	}

	if coord.State() != orchestrator.StateIdle {
		t.Fatalf("expected state Idle after interrupt, got %s", coord.State())
	}
	if !mockTTS.stopped {
		t.Fatal("expected TTS to be stopped on interrupt")
	}
}

func TestFastPathMockVehicle(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "vehicle_audit.log")

	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(nil, nil)

	// Set test audit log path
	vehicleEngine, err := mockvehicle.NewEngine(logPath)
	if err != nil {
		t.Fatalf("failed to create vehicle engine: %v", err)
	}
	fp.SetVehicleEngine(vehicleEngine)

	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "把空调调到24度",
		Stability: orchestrator.StabilityFinal,
	}

	turnID, err := manager.Submit(context.Background(), input)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if turnID == "" {
		t.Fatal("expected non-empty turn ID")
	}

	// Must NOT call TaskRunner (0% cloud or background cognitive overhead)
	if runner.called {
		t.Fatal("expected mock vehicle to be handled 100% locally by fast-path")
	}

	// Verify local voice confirmation was spoken
	if len(mockTTS.spoken) == 0 {
		t.Fatal("expected spoken confirmation for vehicle mock")
	}

	// Verify audit log exists and is populated
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("failed to read audit log: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("audit log must not be empty")
	}
}

func TestFastPathLocalOnlyAndOfflineHandling(t *testing.T) {
	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(nil, nil)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	// 1. Not consented to cloud
	fp.SetCloudConsent(false)
	inputOpen := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "给我讲个哲学家的故事",
		Stability: orchestrator.StabilityFinal,
	}
	_, err := manager.Submit(context.Background(), inputOpen)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if runner.called {
		t.Fatal("expected unconsented turn to be stopped by fast-path with local-only prompt")
	}

	// 2. Consented but offline
	fp.SetCloudConsent(true)
	fp.SetOnlineStatus(false)
	runner.called = false
	_, err = manager.Submit(context.Background(), inputOpen)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if runner.called {
		t.Fatal("expected offline turn to be handled by fast-path with offline prompt")
	}
}
