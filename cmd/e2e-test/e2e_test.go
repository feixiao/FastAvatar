package main

import (
	"context"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"avatar/internal/cloud"
	"avatar/internal/interaction"
	"avatar/internal/mockvehicle"
	"avatar/internal/orchestrator"
	"avatar/server"
)

func TestE2E_FullPipelineVerification(t *testing.T) {
	tempDir := t.TempDir()
	auditLogPath := filepath.Join(tempDir, "mock_vehicle.log")

	// 1. 云端实时服务
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "test_e2e_key",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
		TokenTTL:   5 * time.Minute,
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	httpSrv := httptest.NewServer(cloudSrv.Routes())
	defer httpSrv.Close()

	// 2. 端侧 Mac 客户端初始化
	vehicleEngine, err := mockvehicle.NewEngine(auditLogPath)
	if err != nil {
		t.Fatalf("failed to init vehicle engine: %v", err)
	}
	fastPath := interaction.NewFastPathHandler(nil, nil)
	fastPath.SetVehicleEngine(vehicleEngine)

	cloudClient := cloud.NewClient(httpSrv.URL, "user_test", "mac_m2", "mac")
	speaker := &trackingSpeaker{}
	coord := orchestrator.NewCoordinator("sess_test")
	runner := &streamingCloudRunner{cloudClient: cloudClient, simulateTTFA: 200 * time.Millisecond}

	manager := interaction.NewManager(coord, fastPath, speaker, runner)
	manager.SetCloudClient(cloudClient)

	// ==========================================
	// 验证 1: 本地车控快环 (时延 < 150ms，且零云端网络开销)
	// ==========================================
	t0 := time.Now()
	_, err = manager.Submit(context.Background(), orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "把空调调到24度",
		Stability: orchestrator.StabilityFinal,
	})
	elapsed1 := time.Since(t0)
	if err != nil {
		t.Fatalf("climate control failed: %v", err)
	}
	if elapsed1 > 150*time.Millisecond {
		t.Fatalf("fast loop latency exceeded 150ms: %v", elapsed1)
	}
	if !cloudClient.IsAudioMuted() {
		t.Fatal("audio track must stay muted during local vehicle command")
	}

	// ==========================================
	// 验证 2: 危险指令拦截
	// ==========================================
	_, err = manager.Submit(context.Background(), orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "切换到漂移模式",
		Stability: orchestrator.StabilityFinal,
	})
	if err != nil {
		t.Fatalf("drift command failed: %v", err)
	}

	// ==========================================
	// 验证 3: 开放式对话上云与流式时延 (TTFA < 800ms)
	// ==========================================
	ctx := context.Background()
	_, err = cloudClient.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire token: %v", err)
	}

	tCloud := time.Now()
	turnCloud, err := manager.Submit(ctx, orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "量子力学是什么？",
		Stability: orchestrator.StabilityFinal,
	})
	ttfa := time.Since(tCloud)
	if err != nil {
		t.Fatalf("cloud turn failed: %v", err)
	}
	if ttfa > 800*time.Millisecond {
		t.Fatalf("TTFA exceeded 800ms: %v", ttfa)
	}

	// ==========================================
	// 验证 4: 插话毫秒级打断 (Barge-in ≤ 80ms)
	// ==========================================
	coord.SetState(orchestrator.StateSpeaking)
	tInterrupt := time.Now()
	err = manager.Interrupt(ctx, coord.SessionID(), turnCloud)
	interruptTime := time.Since(tInterrupt)
	if err != nil {
		t.Fatalf("interrupt failed: %v", err)
	}
	if interruptTime > 80*time.Millisecond {
		t.Fatalf("interrupt latency exceeded 80ms: %v", interruptTime)
	}
	if !speaker.stopped {
		t.Fatal("speaker must be physically stopped upon barge-in")
	}
	if !cloudClient.IsAudioMuted() {
		t.Fatal("audio track must be re-muted upon barge-in")
	}

	// 确认云端记录了打断事件
	events := cloudSrv.GetTelemetryEvents()
	var foundCancelled bool
	for _, ev := range events {
		if ev.EventName == "cloud_turn_cancelled" && ev.TurnID == string(turnCloud) {
			foundCancelled = true
			break
		}
	}
	if !foundCancelled {
		t.Fatalf("cloud server must record cloud_turn_cancelled event, events: %+v", events)
	}
}
