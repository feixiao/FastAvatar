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
	SessionID string `json:"session_id"`
	RoomName  string `json:"room_name"`
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in_seconds"`
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
	streamClient    *http.Client
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
		streamClient: &http.Client{Timeout: 0},
		state:        StateDisconnected,
		isAudioMuted: true, // PRD Guardrail: Audio track is muted by default (Gating)
	}
}

// AcquireSessionToken requests token from server gateway.
func (c *Client) AcquireSessionToken(ctx context.Context, cloudConsent bool) (*TokenResponse, error) {
	if !cloudConsent {
		fmt.Printf("[云端鉴权] 拒绝上云: 未开启联网大模型授权 (cloud_consent=false)，严格禁止上云\n")
		return nil, ErrConsentRequired
	}

	fmt.Printf("[云端鉴权] 【走云端】向网关申请会话 Token -> POST %s/api/v1/auth/token | User: %s | Device: %s | Platform: %s\n", c.serverURL, c.userID, c.deviceID, c.clientType)

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
		fmt.Printf("[云端鉴权] 【走云端】网关连接失败: %v (请确认是否已运行 make server 启动网关)\n", err)
		return nil, fmt.Errorf("failed to reach auth gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[云端鉴权] 【走云端】网关拒绝握手: HTTP %d\n", resp.StatusCode)
		return nil, fmt.Errorf("auth gateway rejected with status %d", resp.StatusCode)
	}

	var res TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	sessionID := res.SessionID
	if sessionID == "" {
		sessionID = res.RoomName
	}
	c.mu.Lock()
	c.currentRoom = sessionID
	c.currentToken = res.Token
	c.state = StateConnected
	c.mu.Unlock()

	fmt.Printf("[云端鉴权] 【走云端】会话建立成功 | 会话标识: %s | 客户端状态: %s\n", sessionID, StateConnected)
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
	fmt.Printf("[云端门禁] 【走云端】开启媒体音频推流门禁 (isAudioMuted=false) | TurnID: %s | Room: %s\n", turnID, c.currentRoom)
	return nil
}

// FinishTurn closes the audio gate after turn completes.
func (c *Client) FinishTurn(turnID string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.activeTurnID == turnID {
		c.activeTurnID = ""
		c.isAudioMuted = true // Re-mute media gate
		fmt.Printf("[云端门禁] 【走云端】回合结束，闭锁媒体音频门禁 (isAudioMuted=true) | TurnID: %s\n", turnID)
	}
}

// InterruptActiveTurn immediately notifies cloud to cancel generation and mutes audio.
func (c *Client) InterruptActiveTurn(ctx context.Context, turnID, reason string) error {
	c.mu.Lock()
	room := c.currentRoom
	if turnID == "" {
		turnID = c.activeTurnID
	}
	c.activeTurnID = ""
	c.isAudioMuted = true // Mute audio instantly (< 50ms)
	c.mu.Unlock()

	if room == "" || turnID == "" {
		return nil
	}

	fmt.Printf("[云端打断] 【走云端】向网关下发级联打断信令 -> POST %s/api/v1/session/interrupt | TurnID: %s | Room: %s | 原因: %s | 媒体门禁已闭锁\n", c.serverURL, turnID, room, reason)

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
		fmt.Printf("[云端打断] 【走云端】打断信令网络发送失败: %v\n", err)
		return fmt.Errorf("interrupt signal failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("[云端打断] 【走云端】云端网关已成功接收并响应打断信令 (HTTP %d)\n", resp.StatusCode)
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

	fmt.Printf("[云端会话] 【走云端】向网关请求关闭会话 -> POST %s/api/v1/session/close | Room: %s\n", c.serverURL, room)

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
		fmt.Printf("[云端会话] 【走云端】关闭会话网络失败: %v\n", err)
		return fmt.Errorf("close signal failed: %w", err)
	}
	defer resp.Body.Close()

	fmt.Printf("[云端会话] 【走云端】会话已正常关闭 (HTTP %d)\n", resp.StatusCode)
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

	start := time.Now()
	fmt.Printf("[云端请求] 【走云端】向网关发起全量对话 -> POST %s/api/v1/chat | TurnID: %s | Room: %s | 角色: %s | 输入: %q\n", c.serverURL, turnID, room, persona, message)

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
		fmt.Printf("[云端响应] 【走云端】请求失败: %v | TurnID: %s\n", err, turnID)
		return nil, fmt.Errorf("do chat request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[云端响应] 【走云端】网关返回异常状态: HTTP %d | TurnID: %s\n", resp.StatusCode, turnID)
		return nil, fmt.Errorf("chat request failed with status: %d", resp.StatusCode)
	}

	var res ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("decode chat response: %w", err)
	}
	fmt.Printf("[云端响应] 【走云端】收到完整回复 | TurnID: %s | 耗时: %v\n", turnID, time.Since(start))
	return &res, nil
}

// ChatStream invokes the cloud server /api/v1/chat with SSE streaming and invokes onChunk for each token.
func (c *Client) ChatStream(ctx context.Context, turnID, message, persona string, onChunk func(string) error) error {
	c.mu.RLock()
	room := c.currentRoom
	c.mu.RUnlock()

	start := time.Now()
	fmt.Printf("[云端流式] 【走云端】发起 SSE 流式推理 -> POST %s/api/v1/chat | TurnID: %s | Room: %s | 角色: %s | 输入: %q\n", c.serverURL, turnID, room, persona, message)

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

	resp, err := c.streamClient.Do(req)
	if err != nil {
		fmt.Printf("[云端流式] 【走云端】流式请求发起失败: %v | TurnID: %s\n", err, turnID)
		return fmt.Errorf("do stream request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("[云端流式] 【走云端】网关返回异常状态: HTTP %d | TurnID: %s\n", resp.StatusCode, turnID)
		return fmt.Errorf("stream request failed with status: %d", resp.StatusCode)
	}

	reader := bufio.NewReader(resp.Body)
	var tokenCount int
	var firstTokenOnce sync.Once

	for {
		select {
		case <-ctx.Done():
			fmt.Printf("[云端流式] 【走云端】上下文取消 (Context Done): %v | TurnID: %s\n", ctx.Err(), turnID)
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
			firstTokenOnce.Do(func() {
				ttft := time.Since(start)
				fmt.Printf("[云端流式] 【走云端】收到首包 Token (TTFT: %v) | TurnID: %s\n", ttft, turnID)
			})
			tokenCount++
			if err := onChunk(chunk.Chunk); err != nil {
				return err
			}
		}

		if chunk.Done {
			break
		}
	}

	totalElapsed := time.Since(start)
	fmt.Printf("[云端流式] 【走云端】流式生成完成 | TurnID: %s | 总耗时: %v | 收到Token数: %d\n", turnID, totalElapsed, tokenCount)
	return nil
}
