package cloud

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

var (
	ErrNotConnected     = errors.New("cloud client is not connected")
	ErrConsentRequired  = errors.New("cloud processing requires explicit user consent")
)

type ConnectionState string

const (
	StateDisconnected ConnectionState = "DISCONNECTED"
	StateConnecting   ConnectionState = "CONNECTING"
	StateConnected    ConnectionState = "CONNECTED"
)

type TokenResponse struct {
	LiveKitURL string `json:"livekit_url"`
	RoomName   string `json:"room_name"`
	Token      string `json:"token"`
	ExpiresIn  int64  `json:"expires_in_seconds"`
}

type ChatResponse struct {
	TurnID    string `json:"turn_id"`
	Response  string `json:"response"`
	LatencyMS int64  `json:"latency_ms"`
}

type streamChunkPayload struct {
	TurnID    string `json:"turn_id"`
	Chunk     string `json:"chunk,omitempty"`
	Done      bool   `json:"done,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Client manages cloud session lifecycle, token negotiation, and turn-level interruptions.
type Client struct {
	serverURL       string
	userID          string
	deviceID        string
	clientType      string
	httpClient      *http.Client
	state           ConnectionState
	currentRoom     string
	currentToken    string
	activeTurnID    string
	isAudioMuted    bool
	mu              sync.RWMutex
}

// NewClient creates a new cloud realtime client.
func NewClient(serverURL, userID, deviceID, clientType string) *Client {
	if clientType == "" {
		clientType = "mac"
	}
	return &Client{
		serverURL:    serverURL,
		userID:       userID,
		deviceID:     deviceID,
		clientType:   clientType,
		httpClient:   &http.Client{Timeout: 5 * time.Second},
		state:        StateDisconnected,
		isAudioMuted: true, // PRD Guardrail: Audio track is muted by default (Gating)
	}
}

// AcquireSessionToken requests token from server gateway.
func (c *Client) AcquireSessionToken(ctx context.Context, cloudConsent bool) (*TokenResponse, error) {
	if !cloudConsent {
		return nil, ErrConsentRequired
	}

	payload, err := json.Marshal(map[string]interface{}{
		"user_id":       c.userID,
		"device_id":     c.deviceID,
		"client_type":   c.clientType,
		"cloud_consent": cloudConsent,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/auth/token", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to reach auth gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("auth gateway rejected with status %d", resp.StatusCode)
	}

	var res TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	c.mu.Lock()
	c.currentRoom = res.RoomName
	c.currentToken = res.Token
	c.state = StateConnected
	c.mu.Unlock()

	return &res, nil
}

// StartTurn opens the audio gate for open-ended dialog.
func (c *Client) StartTurn(turnID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.state != StateConnected {
		return ErrNotConnected
	}

	c.activeTurnID = turnID
	c.isAudioMuted = false // Open media gate for open-ended turn
	return nil
}

// FinishTurn closes the audio gate after turn completes.
func (c *Client) FinishTurn(turnID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTurnID == turnID {
		c.activeTurnID = ""
		c.isAudioMuted = true // Re-mute media gate
	}
}

// InterruptActiveTurn immediately notifies cloud to cancel generation and mutes audio.
func (c *Client) InterruptActiveTurn(ctx context.Context, turnID, reason string) error {
	c.mu.Lock()
	room := c.currentRoom
	c.activeTurnID = ""
	c.isAudioMuted = true // Mute audio instantly (< 50ms)
	c.mu.Unlock()

	if room == "" {
		return nil
	}

	payload, _ := json.Marshal(map[string]string{
		"session_id": room,
		"turn_id":    turnID,
		"reason":     reason,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/session/interrupt", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("interrupt signal failed: %w", err)
	}
	defer resp.Body.Close()

	return nil
}

// CloseSession closes active session on server and cleans up.
func (c *Client) CloseSession(ctx context.Context) error {
	c.mu.Lock()
	room := c.currentRoom
	c.currentRoom = ""
	c.currentToken = ""
	c.activeTurnID = ""
	c.isAudioMuted = true
	c.state = StateDisconnected
	c.mu.Unlock()

	if room == "" {
		return nil
	}

	payload, _ := json.Marshal(map[string]string{
		"session_id": room,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/session/close", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

// IsAudioMuted returns whether the media gate is currently blocking audio.
func (c *Client) IsAudioMuted() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.isAudioMuted
}

// GetState returns current connection state.
func (c *Client) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// Chat invokes the cloud server /api/v1/chat endpoint and returns full response.
func (c *Client) Chat(ctx context.Context, turnID, message, persona string) (*ChatResponse, error) {
	c.mu.RLock()
	room := c.currentRoom
	c.mu.RUnlock()

	payload, err := json.Marshal(map[string]interface{}{
		"session_id": room,
		"turn_id":    turnID,
		"user_id":    c.userID,
		"device_id":  c.deviceID,
		"message":    message,
		"persona":    persona,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal chat request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/chat", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create chat request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("chat request failed with status: %d", resp.StatusCode)
	}

	var res ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	return &res, nil
}

// ChatStream invokes the cloud server /api/v1/chat with SSE streaming and invokes onChunk for each token.
func (c *Client) ChatStream(ctx context.Context, turnID, message, persona string, onChunk func(string) error) error {
	c.mu.RLock()
	room := c.currentRoom
	c.mu.RUnlock()

	payload, err := json.Marshal(map[string]interface{}{
		"session_id": room,
		"turn_id":    turnID,
		"user_id":    c.userID,
		"device_id":  c.deviceID,
		"message":    message,
		"persona":    persona,
		"stream":     true,
	})
	if err != nil {
		return fmt.Errorf("marshal stream request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.serverURL+"/api/v1/chat", bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("create stream request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("do stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("stream request failed with status: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}

		dataStr := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if dataStr == "" {
			continue
		}

		var chunk streamChunkPayload
		if err := json.Unmarshal([]byte(dataStr), &chunk); err != nil {
			continue
		}

		if chunk.Chunk != "" && onChunk != nil {
			if err := onChunk(chunk.Chunk); err != nil {
				return err
			}
		}

		if chunk.Done {
			break
		}
	}

	return nil
}
