package cloud

import (
	"context"
	"net/http/httptest"
	"testing"

	"avatar/server"
)

func TestCloudClient_EndToEndLifecycle(t *testing.T) {
	// 1. Setup local mock cloud server
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	ts := httptest.NewServer(cloudSrv.Routes())
	defer ts.Close()

	client := NewClient(ts.URL, "user_test", "mac_device_01", "mac")

	// 2. Audio gate must be muted by default
	if !client.IsAudioMuted() {
		t.Fatal("audio track must be muted by default (Media Gating guardrail)")
	}

	// 3. Acquire token without consent must fail
	ctx := context.Background()
	_, err := client.AcquireSessionToken(ctx, false)
	if err != ErrConsentRequired {
		t.Fatalf("expected ErrConsentRequired, got %v", err)
	}

	// 4. Acquire token with consent must succeed
	res, err := client.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire token: %v", err)
	}
	if res.RoomName == "" || res.Token == "" {
		t.Fatalf("invalid token response: %+v", res)
	}
	if client.GetState() != StateConnected {
		t.Fatalf("expected state connected, got %s", client.GetState())
	}

	// 5. Start turn un-mutes audio gate
	err = client.StartTurn("turn_001")
	if err != nil {
		t.Fatalf("start turn failed: %v", err)
	}
	if client.IsAudioMuted() {
		t.Fatal("audio must be unmuted during active cloud turn")
	}

	// 6. User barge-in triggers instant interrupt
	err = client.InterruptActiveTurn(ctx, "turn_001", "LOCAL_VAD_BARGE_IN")
	if err != nil {
		t.Fatalf("interrupt failed: %v", err)
	}
	if !client.IsAudioMuted() {
		t.Fatal("audio must be instantly muted after interrupt")
	}

	// 7. Close session
	err = client.CloseSession(ctx)
	if err != nil {
		t.Fatalf("close session failed: %v", err)
	}
	if client.GetState() != StateDisconnected {
		t.Fatalf("expected state disconnected, got %s", client.GetState())
	}
}

func TestCloudClient_ChatAndStreaming(t *testing.T) {
	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     "testkey",
		APISecret:  "secretsecretsecret123456789012345",
		LiveKitURL: "ws://localhost:7880",
	})
	cloudSrv := server.NewCloudServer(tokenSvc)
	ts := httptest.NewServer(cloudSrv.Routes())
	defer ts.Close()

	client := NewClient(ts.URL, "user_test", "mac_device_01", "mac")
	ctx := context.Background()

	// 1. Establish session token
	_, err := client.AcquireSessionToken(ctx, true)
	if err != nil {
		t.Fatalf("failed to acquire token: %v", err)
	}

	// 2. Test unary Chat
	chatRes, err := client.Chat(ctx, "turn_chat_1", "你好呀！", "pet")
	if err != nil {
		t.Fatalf("chat failed: %v", err)
	}
	if chatRes.TurnID != "turn_chat_1" || chatRes.Response == "" {
		t.Fatalf("invalid chat response: %+v", chatRes)
	}

	// 3. Test ChatStream
	var streamedTokens []string
	err = client.ChatStream(ctx, "turn_stream_1", "在吗？", "jarvis", func(chunk string) error {
		streamedTokens = append(streamedTokens, chunk)
		return nil
	})
	if err != nil {
		t.Fatalf("chat stream failed: %v", err)
	}
	if len(streamedTokens) == 0 {
		t.Fatal("expected at least one streamed token")
	}
}
