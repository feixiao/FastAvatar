package task

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"avatar/internal/cloud"
	"avatar/internal/config"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
	"avatar/internal/tts"
	"avatar/server"
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
func TestTaskRunner_EdgeModelRouting(t *testing.T) {
	var invokedModel string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chat/completions" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"回答完成。\"}}]}\n\n")
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	bondMgr, _ := persona.NewBondManager(filepath.Join(tmpDir, "bnd.json"))

	mockTTS := tts.NewMockSynthesizer()
	coord := orchestrator.NewCoordinator("test-edge")

	edgeCli := llm.NewClient(ts.URL, "key")
	edgeCfg := config.ModelEndpointConfig{
		BaseURL: ts.URL,
		Model:   "qwen2.5-0.5b-edge",
	}

	llmCfg := config.LLMConfig{
		Model: "qwen2.5-0.5b-edge",
		Edge:  edgeCfg,
	}

	runner := NewRunner(llmCfg, edgeCli, mockTTS, vault, personaMgr, bondMgr, coord)
	runner.SetEdgeClient(edgeCli, edgeCfg)

	var capturedRoute string
	var capturedIsEdge bool
	runner.SetModelRouteCallback(func(route string, model string, isEdge bool) {
		capturedRoute = route
		invokedModel = model
		capturedIsEdge = isEdge
	})

	emit := func(e orchestrator.TaskEvent) {}

	simpleTask := orchestrator.Task{
		TaskID:       "task-simple",
		TurnID:       "turn-1",
		SessionID:    "test-edge",
		GoalSnapshot: "早上好呀",
	}
	_ = runner.Run(context.Background(), simpleTask, emit)
	if capturedRoute != "EDGE_SLM" || !capturedIsEdge || invokedModel != "qwen2.5-0.5b-edge" {
		t.Fatalf("expected EDGE_SLM with qwen2.5-0.5b-edge, got %s / %s", capturedRoute, invokedModel)
	}
}

func TestTaskRunner_CloudGatewayRouting(t *testing.T) {
	// 1. 模拟云端网关服务 (:8088)
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "gw_key",
		APISecret:  "gw_secret_12345678901234567890",
		LiveKitURL: "ws://localhost:7880",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	gwServer := httptest.NewServer(cloudSrv.Routes())
	defer gwServer.Close()

	cloudCli := cloud.NewClient(gwServer.URL, "user_test", "mac_test", "mac")

	// 2. 本地小模型 mock 服务器
	var localInvoked bool
	localServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		localInvoked = true
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"端侧小模型回复。\"}}]}\n\n")
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer localServer.Close()

	edgeCli := llm.NewClient(localServer.URL, "")
	edgeCfg := config.ModelEndpointConfig{
		BaseURL: localServer.URL,
		Model:   "qwen2.5-0.5b-edge",
	}

	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "per.json"))
	bondMgr, _ := persona.NewBondManager(filepath.Join(tmpDir, "bnd.json"))
	mockTTS := tts.NewMockSynthesizer()
	coord := orchestrator.NewCoordinator("test-gw-sess")

	llmCfg := config.LLMConfig{
		Model: "qwen2.5-0.5b-edge",
		Edge:  edgeCfg,
	}

	runner := NewRunner(llmCfg, edgeCli, mockTTS, vault, personaMgr, bondMgr, coord)
	runner.SetCloudGateway(cloudCli)

	var capturedRoute string
	var capturedModel string
	runner.SetModelRouteCallback(func(route string, model string, isEdge bool) {
		capturedRoute = route
		capturedModel = model
	})

	emit := func(e orchestrator.TaskEvent) {}

	// 简单意图 -> 走本地小模型
	simpleTask := orchestrator.Task{
		TaskID:       "task-simple",
		TurnID:       "turn-1",
		SessionID:    "test-gw-sess",
		GoalSnapshot: "你好呀",
	}
	_ = runner.Run(context.Background(), simpleTask, emit)
	if capturedRoute != "EDGE_SLM" || !localInvoked {
		t.Fatalf("expected simple task to invoke local SLM, got %s, invoked=%v", capturedRoute, localInvoked)
	}

	// 复杂意图 -> 走云端网关 (:8088)
	complexTask := orchestrator.Task{
		TaskID:       "task-complex",
		TurnID:       "turn-2",
		SessionID:    "test-gw-sess",
		GoalSnapshot: "请深入分析并推导爱因斯坦场方程在弱场低速下的牛顿引力近似",
	}
	_ = runner.Run(context.Background(), complexTask, emit)
	if capturedRoute != "CLOUD_GATEWAY" || capturedModel != "cloud" {
		t.Fatalf("expected complex task to invoke CLOUD_GATEWAY, got %s / %s", capturedRoute, capturedModel)
	}
}
