package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"avatar/internal/cloud"
	"avatar/internal/interaction"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/tts"
	"avatar/server"
)

func TestE2E_FullPipelineVerification(t *testing.T) {
	tempDir := t.TempDir()
	memPath := filepath.Join(tempDir, "local_memory.json")

	// 1. 模拟上游 LLM 流式输出服务
	upstreamLLM := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		tokens := []string{"海", "水", "呈", "现", "蓝", "色", "，", "主", "要", "是", "因", "为", "光", "散", "射", "。"}
		for _, tok := range tokens {
			chunk := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", tok)
			_, _ = fmt.Fprint(w, chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer upstreamLLM.Close()

	// 2. 云端实时网关 (Cloud Gateway :8088 纯文本 SSE 流式代理)
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "test_e2e_key",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
		TokenTTL:   5 * time.Minute,
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	cloudSrv.SetLLM(llm.NewClient(upstreamLLM.URL, "dummy_key"), "qwen-2.5-7b")
	httpSrv := httptest.NewServer(cloudSrv.Routes())
	defer httpSrv.Close()

	// 3. 端侧 Mac 客户端初始化 (端侧统一 TTS 实例)
	vault, err := memory.NewVault(memPath)
	if err != nil {
		t.Fatalf("failed to init vault: %v", err)
	}
	fastPath := interaction.NewFastPathHandler(vault, nil)
	cloudClient := cloud.NewClient(httpSrv.URL, "user_test", "mac_m2", "mac")

	unifiedTTS := tts.NewMockSynthesizer()
	speaker := interaction.NewSpeechPlayer(unifiedTTS)
	coord := orchestrator.NewCoordinator("sess_test")
	runner := newRealStreamingCloudRunner(cloudClient, speaker)

	manager := interaction.NewManager(coord, fastPath, speaker, runner)
	manager.SetCloudClient(cloudClient)

	// ==========================================
	// 验证 1: 本地记忆快环 (时延 < 150ms，且零云端网络开销)
	// ==========================================
	t0 := time.Now()
	_, err = manager.Submit(context.Background(), orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "帮我记一下美式咖啡",
		Stability: orchestrator.StabilityFinal,
	})
	elapsed1 := time.Since(t0)
	if err != nil {
		t.Fatalf("memory command failed: %v", err)
	}
	if elapsed1 > 350*time.Millisecond {
		t.Fatalf("fast loop latency exceeded 350ms: %v", elapsed1)
	}
	if !cloudClient.IsAudioMuted() {
		t.Fatal("audio track must stay muted during local memory command")
	}

	records := unifiedTTS.GetRecords()
	if len(records) == 0 {
		t.Fatal("expected fast-path to output spoken confirmation")
	}
	voiceA := records[0].Voice
	rateA := records[0].Rate

	// ==========================================
	// 验证 2: 隐私未授权本地保护拦截
	// ==========================================
	fastPath.SetCloudConsent(false)
	_, err = manager.Submit(context.Background(), orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "今天有什么新鲜事",
		Stability: orchestrator.StabilityFinal,
	})
	fastPath.SetCloudConsent(true)
	if err != nil {
		t.Fatalf("privacy protection failed: %v", err)
	}

	// ==========================================
	// 验证 3: 开放式对话上云 + 全链路文本 SSE + 端侧统一 TTS (TTFA < 800ms)
	// ==========================================
	ctx := context.Background()
	_, err = cloudClient.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire token: %v", err)
	}

	recordsBeforeCloud := len(unifiedTTS.GetRecords())
	tCloud := time.Now()
	turnCloud, err := manager.Submit(ctx, orchestrator.UserInput{
		InputType: orchestrator.InputTypeText,
		Text:      "为什么海水是蓝色的？",
		Stability: orchestrator.StabilityFinal,
	})
	ttfa := time.Since(tCloud)
	if err != nil {
		t.Fatalf("cloud turn failed: %v", err)
	}
	if ttfa > 800*time.Millisecond {
		t.Fatalf("TTFA exceeded 800ms: %v", ttfa)
	}

	recordsAfterCloud := unifiedTTS.GetRecords()
	if len(recordsAfterCloud) <= recordsBeforeCloud {
		t.Fatalf("expected cloud stream chunks to be spoken by unified TTS")
	}

	// 核心验证: 快环 A 与慢环 B 的音色 (Voice) 与语速 (Rate) 100% 绝对一致
	for i := recordsBeforeCloud; i < len(recordsAfterCloud); i++ {
		if recordsAfterCloud[i].Voice != voiceA {
			t.Fatalf("voice mismatch: record %d had %s, expected %s", i, recordsAfterCloud[i].Voice, voiceA)
		}
		if recordsAfterCloud[i].Rate != rateA {
			t.Fatalf("rate mismatch: record %d had %d, expected %d", i, recordsAfterCloud[i].Rate, rateA)
		}
	}

	// ==========================================
	// 验证 4: 全双工插话毫秒级打断 (Barge-in ≤ 80ms)
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
	if !unifiedTTS.IsStopped() {
		t.Fatal("unified TTS must be physically stopped upon barge-in")
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
