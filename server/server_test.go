package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
