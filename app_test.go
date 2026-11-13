package main

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"avatar/internal/asr"
	"avatar/internal/config"
	"avatar/internal/engine"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/persona"
	"avatar/internal/tts"
	"avatar/server"
)

func TestAppVoiceIntegration(t *testing.T) {
	t.Setenv("AVATAR_CONFIG_PATH", t.TempDir()+"/config.json")
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
	// Restore original mode to prevent test from mutating persistent user config
	app.ToggleMode()

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

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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

func TestLiveLLMConversationWithASR(t *testing.T) {
	time.Sleep(1500 * time.Millisecond)
	app := NewApp()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	if !llm.ProbeEndpoint(ctx, app.cfg.LLM.BaseURL, app.cfg.LLM.APIKey) {
		t.Skipf("skipping live test, no active LLM endpoint found at %s", app.cfg.LLM.BaseURL)
	}
	if err := app.asrEngine.Start(ctx); err != nil {
		t.Fatalf("ASR start: %v", err)
	}
	defer app.asrEngine.Stop()

	var spoken []string
	app.pipeline.SetCallbacks(func(s engine.State) {
		t.Logf("State -> %s", s)
	}, func(speech string) {
		t.Logf("Speech -> %s", speech)
		spoken = append(spoken, speech)
	}, nil, nil)

	err := app.pipeline.Chat(ctx, "你好")
	if err != nil {
		t.Fatalf("Live Chat error: %v", err)
	}
}
func TestApp_FastPathMemoryWorksWithoutLLM(t *testing.T) {
	app := NewApp()
	var spoken []string
	app.pipeline.SetCallbacks(nil, func(speech string) {
		spoken = append(spoken, speech)
	}, nil, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	// 本地记忆快环不需要任何运行中的 LLM 服务，100% 本地完成
	err := app.pipeline.Chat(ctx, "帮我记一下美式咖啡")
	if err != nil {
		t.Fatalf("fast-path memory command failed: %v", err)
	}

	if len(spoken) == 0 {
		t.Fatal("expected spoken confirmation for memory command")
	}
	if app.pipeline.State() != engine.StateIdle {
		t.Fatalf("expected state idle, got %s", app.pipeline.State())
	}
}

func TestApp_CloudConsentAndEngineStatus(t *testing.T) {
	app := NewApp()

	// 默认状态：纯离线模式
	origConsent := app.GetCloudConsent()

	// 切换联网授权状态
	newConsent := app.SetCloudConsent(!origConsent)
	if newConsent == origConsent {
		t.Fatalf("expected consent to change from %v to %v", origConsent, !origConsent)
	}
	if app.GetCloudConsent() != newConsent {
		t.Fatalf("expected GetCloudConsent to return %v", newConsent)
	}

	// 测试 Toggle
	toggled := app.ToggleCloudConsent()
	if toggled != origConsent {
		t.Fatalf("expected ToggleCloudConsent to flip back to %v, got %v", origConsent, toggled)
	}

	// 还原状态
	_ = app.SetCloudConsent(origConsent)

	// 检查网关与小模型引擎状态
	status := app.CheckEngineStatus()
	if _, ok := status["edge_slm"]; !ok {
		t.Fatalf("expected status to report edge_slm")
	}
	if _, ok := status["cloud_gateway"]; !ok {
		t.Fatalf("expected status to report cloud_gateway")
	}
	if status["cloud_consent"] != origConsent {
		t.Fatalf("expected status cloud_consent to match %v", origConsent)
	}

	// 检查 DualModelInfo
	info := app.GetDualModelInfo()
	if info["cloud_gateway"] == "" {
		t.Fatalf("expected cloud_gateway in dual model info")
	}
}

func TestApp_CloudRoutingAndLogging(t *testing.T) {
	tokenSvc := server.NewTokenService(server.TokenConfig{
		SecretKey: "secretsecretsecret123456789012345",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	ts := httptest.NewServer(cloudSrv.Routes())
	defer ts.Close()

	app := NewApp()
	origURL := app.GetCloudServerURL()
	origConsent := app.GetCloudConsent()
	defer func() {
		app.SetCloudServerURL(origURL)
		app.SetCloudConsent(origConsent)
	}()

	app.SetCloudServerURL(ts.URL)
	app.SetCloudConsent(true)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	res, err := app.cloudClient.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire session token: %v", err)
	}
	if res.RoomName == "" {
		t.Fatalf("expected non-empty room name")
	}

	var routeRecorded string
	app.pipeline.SetModelRouteCallback(func(route string, model string, isEdge bool) {
		routeRecorded = route
	})

	err = app.pipeline.Chat(context.Background(), "帮我记一下明天的会议在下午三点")
	if err != nil {
		t.Fatalf("memory command failed: %v", err)
	}
	if routeRecorded != "" {
		t.Fatalf("expected fast-path memory command not to invoke cognitive model, got %s", routeRecorded)
	}

	err = app.pipeline.Chat(context.Background(), "为什么天空是蓝色的？请详细分析物理原理")
	if err != nil {
		t.Fatalf("complex query chat failed: %v", err)
	}
	if routeRecorded != "CLOUD_GATEWAY" {
		t.Fatalf("expected complex query to route to CLOUD_GATEWAY, got %s", routeRecorded)
	}

	routeRecorded = ""
	err = app.SendTextMessage("编写一段代码实现快速排序算法")
	if err != nil {
		t.Fatalf("SendTextMessage failed: %v", err)
	}
	time.Sleep(300 * time.Millisecond)
	if routeRecorded != "CLOUD_GATEWAY" {
		t.Fatalf("expected text message to route to CLOUD_GATEWAY, got %s", routeRecorded)
	}
}

func TestApp_LuoxiaoheiAndRoaming(t *testing.T) {
	t.Setenv("AVATAR_CONFIG_PATH", t.TempDir()+"/config.json")
	app := NewApp()

	// 1. Verify default skin is luoxiaohei2d
	skinID := app.GetPetSkin()
	if skinID != "luoxiaohei2d" {
		t.Fatalf("expected default skin 'luoxiaohei2d', got '%s'", skinID)
	}

	// 2. Verify luoxiaohei2d interactions
	resPet := app.InteractPet("pet")
	if msg, ok := resPet["message"].(string); !ok || msg == "" {
		t.Fatalf("expected pet message for luoxiaohei2d, got %v", resPet)
	}

	resFeed := app.InteractPet("feed")
	if cue, ok := resFeed["cue"].(string); !ok || cue != "running" {
		t.Fatalf("expected cue 'running' (Row 7) for feed action, got %v", resFeed)
	}

	resPoke := app.InteractPet("poke")
	if msg, ok := resPoke["message"].(string); !ok || msg == "" {
		t.Fatalf("expected poke message for luoxiaohei2d, got %v", resPoke)
	}

	// 3. Verify roaming toggle
	initialRoaming := app.GetAutoRoaming()
	toggled := app.ToggleAutoRoaming()
	if toggled == initialRoaming {
		t.Fatalf("expected roaming toggle to change from %v", initialRoaming)
	}
	_ = app.SetAutoRoaming(true)
	if !app.GetAutoRoaming() {
		t.Fatalf("expected AutoRoaming to be true after SetAutoRoaming(true)")
	}

	// 4. Verify DialKit params
	dial := app.GetDialParams()
	if dial["scale"] == 0 || dial["speed"] == 0 {
		t.Fatalf("expected valid default scale and speed, got %v", dial)
	}
	app.SetDialParam("scale", 1.25)
	app.SetDialParam("speed", 1.5)
	dialUpdated := app.GetDialParams()
	if dialUpdated["scale"] != 1.25 || dialUpdated["speed"] != 1.5 {
		t.Fatalf("expected updated dial params, got %v", dialUpdated)
	}
}
