package interaction

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"avatar/internal/cloud"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
	"avatar/server"
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

func TestFastPathMemoryCommand(t *testing.T) {
	tmpDir := t.TempDir()
	memPath := filepath.Join(tmpDir, "mem.json")
	vault, err := memory.NewVault(memPath)
	if err != nil {
		t.Fatalf("failed to create vault: %v", err)
	}

	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(vault, nil)

	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "帮我记一下美式咖啡",
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
		t.Fatal("expected memory command to be handled 100% locally by fast-path")
	}

	// Verify local voice confirmation was spoken
	if len(mockTTS.spoken) == 0 {
		t.Fatal("expected spoken confirmation for local memory command")
	}
}

func TestFastPathPetAction(t *testing.T) {
	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess")
	fp := NewFastPathHandler(nil, nil)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	var capturedAction PetActionPayload
	actionCalled := false
	manager.SetPetActionCallback(func(act PetActionPayload) {
		actionCalled = true
		capturedAction = act
	})

	input := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "小黑，翻个跟头跳一下",
		Stability: orchestrator.StabilityFinal,
	}

	turnID, err := manager.Submit(context.Background(), input)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if turnID == "" {
		t.Fatal("expected valid turn ID")
	}
	if runner.called {
		t.Fatal("expected pet action to be handled by fast-path without invoking task runner")
	}
	if !actionCalled {
		t.Fatal("expected onPetAction callback to be invoked")
	}
	if capturedAction.Action != "jump" || capturedAction.Cue != "jumping" {
		t.Fatalf("unexpected action payload: %+v", capturedAction)
	}
	if len(mockTTS.spoken) == 0 {
		t.Fatal("expected confirmation voice to be spoken")
	}
}

func TestFastPathPatrolRoamAction(t *testing.T) {
	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("test-sess-roam")
	fp := NewFastPathHandler(nil, nil)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	var events []string
	var capturedAction PetActionPayload
	coord.SetStateChangeCallback(func(s orchestrator.State) {
		events = append(events, "state:"+string(s))
	})
	manager.SetSpeechChunkCallback(func(s string) {
		events = append(events, "speech:"+s)
	})
	manager.SetPetActionCallback(func(act PetActionPayload) {
		capturedAction = act
		events = append(events, "action:"+act.Action)
	})

	_, err := manager.Submit(context.Background(), orchestrator.UserInput{
		InputType: orchestrator.InputTypeVoice,
		Text:      "去巡逻",
		Stability: orchestrator.StabilityFinal,
	})
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}
	if capturedAction.Action != "roam" {
		t.Fatalf("expected action 'roam', got %+v", capturedAction)
	}
	if len(events) < 4 || events[0] != "action:roam" || events[1] != "state:speaking" {
		t.Fatalf("unexpected event sequence: %v", events)
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

func TestEdgeCloudCoordination_Interruption(t *testing.T) {
	// 1. 启动轻量云端实时服务
	tokenSvc := server.NewTokenService(server.TokenConfig{
		SecretKey: "secretsecretsecret123456789012345",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	ts := httptest.NewServer(cloudSrv.Routes())
	defer ts.Close()

	// 2. 客户端完成握手进入 Connected 态
	cloudClient := cloud.NewClient(ts.URL, "user_mac", "mac_client_01", "mac")
	ctx := context.Background()
	_, err := cloudClient.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire cloud session token: %v", err)
	}

	// 3. 构造 Mac 端主管编排器
	mockTTS := &testTrackingSynth{}
	coord := orchestrator.NewCoordinator("edge-cloud-sess")
	fp := NewFastPathHandler(nil, nil)
	player := NewSpeechPlayer(mockTTS)
	runner := &mockTaskRunner{}
	manager := NewManager(coord, fp, player, runner)

	// 绑定云端实时客户端
	manager.SetCloudClient(cloudClient)

	// 4. 提交开放式对话回合（开放媒体门禁推流）
	inputOpen := orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "你能跟我聊聊量子力学吗？",
		Stability: orchestrator.StabilityFinal,
	}
	turnID, err := manager.Submit(ctx, inputOpen)
	if err != nil {
		t.Fatalf("Submit error: %v", err)
	}

	if !runner.called {
		t.Fatal("expected task runner to be called for open-ended dialog")
	}

	// 5. 模拟用户突然发声插话 (Barge-in: Swift 原生 VAD 检测到有效能量)
	coord.SetState(orchestrator.StateSpeaking)
	err = manager.Interrupt(ctx, coord.SessionID(), turnID)
	if err != nil {
		t.Fatalf("Interrupt failed: %v", err)
	}

	// 验证双重解耦：
	// 断言 1: 端侧声学层本地播放器立即物理停止 (≤ 80ms)
	if !mockTTS.stopped {
		t.Fatal("local speaker must be stopped immediately upon barge-in")
	}

	// 断言 2: 端侧推流媒体门禁立即重新关闭，停止向云端推流
	if !cloudClient.IsAudioMuted() {
		t.Fatal("cloud client audio track must be muted immediately upon barge-in")
	}

	// 断言 3: 云端实时服务成功接收 turn.interrupt 并记录 cloud_turn_cancelled 事件
	telemetry := cloudSrv.GetTelemetryEvents()
	var foundCancelled bool
	for _, ev := range telemetry {
		if ev.EventName == "cloud_turn_cancelled" && ev.TurnID == string(turnID) {
			foundCancelled = true
			break
		}
	}
	if !foundCancelled {
		t.Fatalf("cloud server must record cloud_turn_cancelled event for turn %s, events: %+v", turnID, telemetry)
	}
}
