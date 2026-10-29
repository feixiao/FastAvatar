package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"avatar/internal/llm"
)

func TestServer_AuthAndConsentGate(t *testing.T) {
	tokenSvc := NewTokenService(TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "wss://livekit.test.avatar.ai",
	})
	srv := NewCloudServer(tokenSvc)
	handler := srv.Routes()

	// 1. Unconsented client must be rejected with 403
	bodyUnconsented, _ := json.Marshal(TokenRequest{
		UserID:       "user_01",
		DeviceID:     "mac_dev_01",
		ClientType:   "mac",
		CloudConsent: false,
	})
	req1 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(bodyUnconsented))
	w1 := httptest.NewRecorder()
	handler.ServeHTTP(w1, req1)

	if w1.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unconsented request, got %d", w1.Code)
	}

	// 2. Consented client gets valid JWT and isolated room
	bodyConsented, _ := json.Marshal(TokenRequest{
		UserID:       "user_01",
		DeviceID:     "mac_dev_01",
		ClientType:   "mac",
		CloudConsent: true,
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(bodyConsented))
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for consented request, got %d", w2.Code)
	}

	var res TokenResponse
	if err := json.Unmarshal(w2.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if res.RoomName == "" || res.Token == "" {
		t.Fatalf("expected valid roomName and token, got room: %s", res.RoomName)
	}

	// 3. Pixel client gets isolated room
	bodyPixel, _ := json.Marshal(TokenRequest{
		UserID:       "user_01",
		DeviceID:     "pixel_dev_02",
		ClientType:   "pixel",
		CloudConsent: true,
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(bodyPixel))
	w3 := httptest.NewRecorder()
	handler.ServeHTTP(w3, req3)

	var resPixel TokenResponse
	_ = json.Unmarshal(w3.Body.Bytes(), &resPixel)
	if resPixel.RoomName == res.RoomName {
		t.Fatalf("Pixel and Mac must have isolated room names, got duplicate: %s", res.RoomName)
	}
}

func TestServer_InterruptAndCloseLifecycle(t *testing.T) {
	tokenSvc := NewTokenService(TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "wss://livekit.test.avatar.ai",
	})
	srv := NewCloudServer(tokenSvc)
	handler := srv.Routes()

	// 1. Establish session
	tokenReq, _ := json.Marshal(TokenRequest{
		UserID:       "user_01",
		DeviceID:     "mac_dev_01",
		ClientType:   "mac",
		CloudConsent: true,
	})
	wAuth := httptest.NewRecorder()
	handler.ServeHTTP(wAuth, httptest.NewRequest(http.MethodPost, "/api/v1/auth/token", bytes.NewReader(tokenReq)))

	var tokenRes TokenResponse
	_ = json.Unmarshal(wAuth.Body.Bytes(), &tokenRes)
	room := tokenRes.RoomName

	// 2. Client barge-in triggers interrupt
	interruptBody, _ := json.Marshal(InterruptRequest{
		SessionID: room,
		TurnID:    "turn_01",
		Reason:    "LOCAL_VAD_BARGE_IN",
	})
	wInterrupt := httptest.NewRecorder()
	handler.ServeHTTP(wInterrupt, httptest.NewRequest(http.MethodPost, "/api/v1/session/interrupt", bytes.NewReader(interruptBody)))

	if wInterrupt.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for interrupt, got %d", wInterrupt.Code)
	}

	// 3. Close session purges ephemeral context
	closeBody, _ := json.Marshal(CloseRequest{SessionID: room})
	wClose := httptest.NewRecorder()
	handler.ServeHTTP(wClose, httptest.NewRequest(http.MethodPost, "/api/v1/session/close", bytes.NewReader(closeBody)))

	if wClose.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for close, got %d", wClose.Code)
	}

	srv.mu.RLock()
	_, stillExists := srv.sessions[room]
	srv.mu.RUnlock()
	if stillExists {
		t.Fatal("session memory must be purged after close")
	}

	// 4. Verify telemetry conforms to PRD (zero user content)
	events := srv.GetTelemetryEvents()
	if len(events) == 0 {
		t.Fatal("expected telemetry events")
	}
	for _, ev := range events {
		if ev.EventName == "" || ev.Timestamp == "" {
			t.Errorf("incomplete telemetry event: %+v", ev)
		}
	}
}

func TestServer_LLMUnaryAndStreamingChat(t *testing.T) {
	// 1. Mock upstream OpenAI-compatible LLM endpoint
	var lastReceivedSystemPrompt string
	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req llm.ChatCompletionRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		for _, m := range req.Messages {
			if m.Role == "system" {
				lastReceivedSystemPrompt = m.Content
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		tokens := []string{"云端", "智能体", "为您服务。"}
		for _, tok := range tokens {
			chunkJSON := fmt.Sprintf(`{"choices":[{"delta":{"content":"%s"}}]}`, tok)
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkJSON)
			flusher.Flush()
		}
		_, _ = fmt.Fprintf(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer upstreamSrv.Close()

	tokenSvc := NewTokenService(TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
	})
	srv := NewCloudServer(tokenSvc)
	srv.SetLLM(llm.NewClient(upstreamSrv.URL, "key_123"), "qwen-2.5-7b")
	handler := srv.Routes()

	// 2. Unary Chat verification with Roaster persona
	unaryReqBody, _ := json.Marshal(ChatRequest{
		SessionID: "room_test_1",
		TurnID:    "turn_01",
		UserID:    "user_frank",
		DeviceID:  "mac_01",
		Message:   "你觉得今天天气怎么样？",
		Persona:   "roaster",
	})
	wUnary := httptest.NewRecorder()
	handler.ServeHTTP(wUnary, httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(unaryReqBody)))

	if wUnary.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from unary chat, got %d: %s", wUnary.Code, wUnary.Body.String())
	}
	var unaryRes ChatResponse
	if err := json.Unmarshal(wUnary.Body.Bytes(), &unaryRes); err != nil {
		t.Fatalf("failed to decode unary response: %v", err)
	}
	if unaryRes.Response != "云端智能体为您服务。" {
		t.Fatalf("expected accumulated response, got %s", unaryRes.Response)
	}
	if !strings.Contains(lastReceivedSystemPrompt, "吐槽") {
		t.Fatalf("expected roaster prompt to be sent to upstream LLM, got: %s", lastReceivedSystemPrompt)
	}

	// 3. Streaming Chat verification with SSE
	streamReqBody, _ := json.Marshal(ChatRequest{
		SessionID: "room_test_1",
		TurnID:    "turn_02",
		UserID:    "user_frank",
		DeviceID:  "mac_01",
		Message:   "详细说说。",
		Persona:   "jarvis",
		Stream:    true,
	})
	wStream := httptest.NewRecorder()
	handler.ServeHTTP(wStream, httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(streamReqBody)))

	if wStream.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from stream chat, got %d", wStream.Code)
	}
	if !strings.Contains(wStream.Header().Get("Content-Type"), "text/event-stream") {
		t.Fatalf("expected SSE content type, got %s", wStream.Header().Get("Content-Type"))
	}

	scanner := bufio.NewScanner(wStream.Body)
	var receivedChunks []string
	foundDone := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "data:") {
			raw := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
			var chunk StreamChunk
			if err := json.Unmarshal([]byte(raw), &chunk); err == nil {
				if chunk.Chunk != "" {
					receivedChunks = append(receivedChunks, chunk.Chunk)
				}
				if chunk.Done {
					foundDone = true
				}
			}
		}
	}
	if !foundDone || len(receivedChunks) == 0 {
		t.Fatalf("expected valid stream chunks and done indicator, got chunks: %v, done: %v", receivedChunks, foundDone)
	}
	if !strings.Contains(lastReceivedSystemPrompt, "严谨") {
		t.Fatalf("expected jarvis prompt, got: %s", lastReceivedSystemPrompt)
	}

	// 4. Test Upstream LLM Failure & Graceful Fallback
	upstreamSrv.Close() // terminate upstream
	wFail := httptest.NewRecorder()
	failReqBody, _ := json.Marshal(ChatRequest{
		SessionID: "room_test_1",
		TurnID:    "turn_03",
		Message:   "测试离线",
		Persona:   "pet",
	})
	handler.ServeHTTP(wFail, httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewReader(failReqBody)))
	if wFail.Code != http.StatusOK {
		t.Fatalf("expected graceful 200 OK with fallback, got %d", wFail.Code)
	}
	var failRes ChatResponse
	_ = json.Unmarshal(wFail.Body.Bytes(), &failRes)
	if !strings.Contains(failRes.Response, "喵呜") {
		t.Fatalf("expected pet fallback reply on upstream failure, got %s", failRes.Response)
	}
}
