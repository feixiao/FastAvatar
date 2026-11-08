package engine

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
	"avatar/internal/persona"
	"avatar/internal/tts"
	"avatar/server"
)

func TestPipelineChatAndInterrupt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		tokens := []string{"你", "好", "呀", "！", "我", "是", "小", "助", "手", "。"}
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

	cfg := config.LoadConfig()
	cfg.LLM.BaseURL = ts.URL
	cfg.LLM.Model = "test-model"
	cfg.LLM.Edge.BaseURL = ts.URL
	cfg.LLM.Edge.Model = "test-model"
	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "p_mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "p_per.json"))
	bondMgr, _ := persona.NewBondManager(filepath.Join(tmpDir, "p_bnd.json"))

	mockTTS := tts.NewMockSynthesizer()
	pipeline := NewPipeline(cfg, mockTTS, vault, personaMgr, bondMgr)

	var states []State
	pipeline.SetCallbacks(func(s State) {
		states = append(states, s)
	}, nil, nil, nil)

	ctx := context.Background()
	err := pipeline.Chat(ctx, "你好")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if len(mockTTS.Spoken) == 0 {
		t.Fatal("expected TTS to speak chunks")
	}

	if pipeline.State() != StateIdle {
		t.Fatalf("expected state Idle, got %s", pipeline.State())
	}

	// 测试口语指令拦截（切换人格）
	mockTTS.Spoken = nil
	err = pipeline.Chat(ctx, "切换成高效幕僚模式")
	if err != nil {
		t.Fatalf("switch persona chat error: %v", err)
	}
	if len(mockTTS.Spoken) == 0 || personaMgr.GetActiveSlot().ID != "jarvis" {
		t.Fatalf("expected persona switch spoken confirmation and active jarvis, got %v", personaMgr.GetActiveSlot())
	}

	// 测试端侧快环（本地记忆指令）
	mockTTS.Spoken = nil
	err = pipeline.Chat(ctx, "帮我记一下美式咖啡")
	if err != nil {
		t.Fatalf("memory fast-path chat error: %v", err)
	}
	if len(mockTTS.Spoken) == 0 {
		t.Fatal("expected memory fast-path spoken confirmation")
	}
	if pipeline.State() != StateIdle {
		t.Fatalf("expected state Idle after memory command, got %s", pipeline.State())
	}
}

func TestPipelineUnifiedEdgeTTSAndTimbreConsistency(t *testing.T) {
	// 1. Mock upstream LLM SSE streaming service
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		tokens := []string{"海", "水", "呈", "现", "蓝", "色", "，", "是", "因", "为", "光", "散", "射", "。"}
		for _, tok := range tokens {
			chunk := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", tok)
			_, _ = fmt.Fprint(w, chunk)
			flusher.Flush()
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	cfg := config.LoadConfig()
	cfg.LLM.BaseURL = ts.URL
	cfg.LLM.Model = "test-model"
	cfg.LLM.Edge.BaseURL = ts.URL
	cfg.LLM.Edge.Model = "test-model"
	tmpDir := t.TempDir()
	vault, _ := memory.NewVault(filepath.Join(tmpDir, "p_mem.json"))
	personaMgr, _ := persona.NewManager(filepath.Join(tmpDir, "p_per.json"))
	bondMgr, _ := persona.NewBondManager(filepath.Join(tmpDir, "p_bnd.json"))

	// 2. 模拟云端实时网关 (SSE 流式文本)
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	cloudSrv.SetLLM(llm.NewClient(ts.URL, "key"), "test-model")
	gwServer := httptest.NewServer(cloudSrv.Routes())
	defer gwServer.Close()

	mockTTS := tts.NewMockSynthesizer()
	pipeline := NewPipeline(cfg, mockTTS, vault, personaMgr, bondMgr)
	cloudCli := cloud.NewClient(gwServer.URL, "user_01", "mac_01", "mac")
	pipeline.SetCloudClient(cloudCli)
	pipeline.SetCloudConsent(true)
	_, _ = cloudCli.AcquireSessionToken(context.Background(), true)

	if pipeline.Synthesizer() != mockTTS {
		t.Fatalf("expected pipeline synthesizer to match the injected instance")
	}

	pipeline.SetCallbacks(nil, nil, nil, nil)
	ctx := context.Background()

	// --- 阶段 1: 快环 A (本地记忆指令) ---
	err := pipeline.Chat(ctx, "帮我记一下美式咖啡")
	if err != nil {
		t.Fatalf("fast-path chat failed: %v", err)
	}
	recordsA := mockTTS.GetRecords()
	if len(recordsA) == 0 {
		t.Fatal("expected speech output for fast-path")
	}
	voiceA := recordsA[0].Voice
	rateA := recordsA[0].Rate

	// --- 阶段 2: 慢环 B (LLM 流式对话) ---
	err = pipeline.Chat(ctx, "为什么海水是蓝色的")
	if err != nil {
		t.Fatalf("stream chat failed: %v", err)
	}
	recordsB := mockTTS.GetRecords()
	if len(recordsB) <= len(recordsA) {
		t.Fatal("expected additional speech output for stream chat")
	}
	for i := len(recordsA); i < len(recordsB); i++ {
		if recordsB[i].Voice != voiceA {
			t.Fatalf("timbre inconsistency: expected %s, got %s in record %d", voiceA, recordsB[i].Voice, i)
		}
		if recordsB[i].Rate != rateA {
			t.Fatalf("rate inconsistency: expected %d, got %d in record %d", rateA, recordsB[i].Rate, i)
		}
	}

	// --- 阶段 3: 切换人设为高效幕僚 (Jarvis, rate 195) 并验证两环音色同步流转 ---
	err = pipeline.Chat(ctx, "切换成高效幕僚模式")
	if err != nil {
		t.Fatalf("switch persona failed: %v", err)
	}
	recordsSwitch := mockTTS.GetRecords()
	lastRecord := recordsSwitch[len(recordsSwitch)-1]
	if lastRecord.Rate != 195 {
		t.Fatalf("expected jarvis speech rate 195, got %d", lastRecord.Rate)
	}

	// 再次发起慢环 B 对话，验证慢环播报也自动同步使用 Jarvis 的语速和音色
	err = pipeline.Chat(ctx, "请分析一下系统架构")
	if err != nil {
		t.Fatalf("subsequent chat failed: %v", err)
	}
	recordsAfter := mockTTS.GetRecords()
	for i := len(recordsSwitch); i < len(recordsAfter); i++ {
		if recordsAfter[i].Rate != 195 {
			t.Fatalf("expected subsequent stream chunk to use rate 195, got %d in record %d", recordsAfter[i].Rate, i)
		}
	}
}
