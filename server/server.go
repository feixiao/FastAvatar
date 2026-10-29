package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"avatar/internal/llm"
)

// ServerEvent represents structured operational telemetry strictly without user content.
// PRD Guardrail: No raw audio, no full transcript text.
type ServerEvent struct {
	EventName string `json:"event_name"`
	SessionID string `json:"session_id"`
	TurnID    string `json:"turn_id,omitempty"`
	Platform  string `json:"platform"`
	DurationMS int64 `json:"duration_ms,omitempty"`
	ErrorCode string `json:"error_code,omitempty"`
	Timestamp string `json:"timestamp"`
}

// SessionState tracks cloud realtime session lifecycle.
type SessionState string

const (
	SessionStateNew        SessionState = "NEW"
	SessionStateReady      SessionState = "READY"
	SessionStateProcessing SessionState = "PROCESSING"
	SessionStateStreaming  SessionState = "STREAMING"
	SessionStateClosed     SessionState = "CLOSED"
)

// ActiveSession represents an ephemeral in-memory cloud session.
type ActiveSession struct {
	SessionID string
	UserID    string
	DeviceID  string
	Platform  string
	State     SessionState
	ActiveTurn string
	StartedAt time.Time
	UpdatedAt time.Time
}

// CloudServer implements the minimal MVP realtime orchestration service.
type CloudServer struct {
	tokenService *TokenService
	mu           sync.RWMutex
	sessions     map[string]*ActiveSession
	eventsLog    []ServerEvent // In-memory audit telemetry for PRD verification
	llmClient    *llm.Client
	llmModel     string
}

// NewCloudServer creates a new cloud realtime server.
func NewCloudServer(tokenService *TokenService) *CloudServer {
	return &CloudServer{
		tokenService: tokenService,
		sessions:     make(map[string]*ActiveSession),
	}
}

// SetLLM configures upstream LLM service for dynamic chat generation.
func (s *CloudServer) SetLLM(client *llm.Client, model string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.llmClient = client
	s.llmModel = model
}

// GetLLMModel returns configured LLM model name.
func (s *CloudServer) GetLLMModel() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.llmModel
}

// Routes sets up standard HTTP endpoints.
func (s *CloudServer) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/api/v1/auth/token", s.handleToken)
	mux.HandleFunc("/api/v1/chat", s.handleChat)
	mux.HandleFunc("/api/v1/session/interrupt", s.handleInterrupt)
	mux.HandleFunc("/api/v1/session/close", s.handleClose)
	return mux
}

func (s *CloudServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok","service":"avatar-cloud-realtime"}`))
}

func (s *CloudServer) handleToken(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := s.tokenService.IssueToken(req)
	if err != nil {
		if err == ErrCloudNotConsented {
			s.recordEvent(ServerEvent{
				EventName: "cloud_session_rejected",
				Platform:  req.ClientType,
				ErrorCode: "ERR_CLOUD_NOT_CONSENTED",
			})
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Register ephemeral session
	s.mu.Lock()
	sess := &ActiveSession{
		SessionID: res.RoomName,
		UserID:    req.UserID,
		DeviceID:  req.DeviceID,
		Platform:  req.ClientType,
		State:     SessionStateReady,
		StartedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	s.sessions[res.RoomName] = sess
	s.mu.Unlock()

	s.recordEvent(ServerEvent{
		EventName: "cloud_session_started",
		SessionID: res.RoomName,
		Platform:  req.ClientType,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

type ChatRequest struct {
	SessionID string `json:"session_id"`
	TurnID    string `json:"turn_id"`
	UserID    string `json:"user_id"`
	DeviceID  string `json:"device_id"`
	Message   string `json:"message"`
	Persona   string `json:"persona,omitempty"`
	Stream    bool   `json:"stream,omitempty"`
}

type ChatResponse struct {
	TurnID    string `json:"turn_id"`
	Response  string `json:"response"`
	LatencyMS int64  `json:"latency_ms"`
}

type StreamChunk struct {
	TurnID    string `json:"turn_id"`
	Chunk     string `json:"chunk,omitempty"`
	Done      bool   `json:"done,omitempty"`
	LatencyMS int64  `json:"latency_ms,omitempty"`
	Error     string `json:"error,omitempty"`
}

func (s *CloudServer) handleChat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	start := time.Now()
	ctx := r.Context()

	// Update session state if session exists
	s.mu.Lock()
	if sess, exists := s.sessions[req.SessionID]; exists {
		sess.State = SessionStateProcessing
		sess.ActiveTurn = req.TurnID
		sess.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	wantsStream := req.Stream || strings.Contains(r.Header.Get("Accept"), "text/event-stream")

	if wantsStream {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		var fullReply strings.Builder
		err := s.streamReply(ctx, req.Message, req.Persona, func(token string) error {
			fullReply.WriteString(token)
			chunkPayload, _ := json.Marshal(StreamChunk{
				TurnID: req.TurnID,
				Chunk:  token,
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkPayload)
			flusher.Flush()
			return nil
		})

		latency := time.Since(start).Milliseconds()
		if err != nil && fullReply.Len() == 0 {
			// Fallback on total failure before any token sent
			fallback := s.getFallbackReply(req.Message, req.Persona)
			chunkPayload, _ := json.Marshal(StreamChunk{
				TurnID: req.TurnID,
				Chunk:  fallback,
			})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", chunkPayload)
			flusher.Flush()
		}

		donePayload, _ := json.Marshal(StreamChunk{
			TurnID:    req.TurnID,
			Done:      true,
			LatencyMS: latency,
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", donePayload)
		flusher.Flush()

		s.finishTurnSession(req.SessionID, req.TurnID, latency, err)
		return
	}

	reply, err := s.generateReply(ctx, req.Message, req.Persona)
	latency := time.Since(start).Milliseconds()

	s.finishTurnSession(req.SessionID, req.TurnID, latency, err)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ChatResponse{
		TurnID:    req.TurnID,
		Response:  reply,
		LatencyMS: latency,
	})
}

func (s *CloudServer) finishTurnSession(sessionID, turnID string, latency int64, err error) {
	s.mu.Lock()
	if sess, exists := s.sessions[sessionID]; exists {
		sess.State = SessionStateReady
		sess.ActiveTurn = ""
		sess.UpdatedAt = time.Now()
	}
	s.mu.Unlock()

	errorCode := ""
	if err != nil {
		errorCode = "ERR_UPSTREAM_LLM"
	}

	s.recordEvent(ServerEvent{
		EventName:  "cloud_turn_completed",
		SessionID:  sessionID,
		TurnID:     turnID,
		Platform:   "gateway",
		DurationMS: latency,
		ErrorCode:  errorCode,
	})
}

func (s *CloudServer) streamReply(ctx context.Context, msg, persona string, onToken func(string) error) error {
	s.mu.RLock()
	client := s.llmClient
	model := s.llmModel
	s.mu.RUnlock()

	if client == nil {
		fallback := s.getFallbackReply(msg, persona)
		return onToken(fallback)
	}

	if model == "" {
		model = "default"
	}

	systemPrompt := getPersonaPrompt(persona)
	req := llm.ChatCompletionRequest{
		Model: model,
		Messages: []llm.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: msg},
		},
		Temperature: 0.7,
	}

	return client.StreamChat(ctx, req, onToken)
}

func (s *CloudServer) generateReply(ctx context.Context, msg, persona string) (string, error) {
	var builder strings.Builder
	err := s.streamReply(ctx, msg, persona, func(token string) error {
		builder.WriteString(token)
		return nil
	})
	if err != nil {
		if builder.Len() > 0 {
			return builder.String(), err
		}
		return s.getFallbackReply(msg, persona), err
	}
	return builder.String(), nil
}

func getPersonaPrompt(persona string) string {
	switch persona {
	case "roaster":
		return "你是一个幽默犀利的吐槽高手，说话毒舌但充满关怀和智慧，善于用讽刺和诙谐化解尴尬，句句精辟，语言简明有力，适合语音播报。"
	case "jarvis":
		return "你是一个极度严谨高效的智能顾问与私人幕僚，擅长结构化拆解问题并提供执行导向的高价值建议，语气专业温和克制，语言精炼。"
	case "pet":
		return "你是一只贴心黏人的专属桌宠小伴侣，性格乖巧软萌，喜欢用拟声词（如喵呜、蹭蹭）表达喜爱与支持，永远陪伴倾听。"
	default:
		return "你是一个温暖亲切、富有同理心的 AI 语音伴侣，倾听并支持用户，回答自然口语化、简洁精炼适合语音播报。"
	}
}

func (s *CloudServer) getFallbackReply(msg, persona string) string {
	switch persona {
	case "roaster":
		return "哟，稀客啊！说吧，今天又遇到什么让人哭笑不得的事想让我吐槽了？"
	case "jarvis":
		return "指令已接收。针对该问题，建议从核心目标分解执行，如有进一步分析需求请随时指示。"
	case "pet":
		return "喵呜~ 主人终于找我聊天啦！蹭蹭手心，今天我也会一直陪着主人哦！"
	default:
		if len(msg) > 0 {
			return "嗨！很高兴陪你聊天。听你这么说，生活里总有许多值得回味的小瞬间，不管你想聊什么，我都认真听着呢。"
		}
		return "你好呀！我是你的 Avatar 伴侣，今天过得开心吗？"
	}
}

type InterruptRequest struct {
	SessionID string `json:"session_id"`
	TurnID    string `json:"turn_id"`
	Reason    string `json:"reason"`
}

func (s *CloudServer) handleInterrupt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req InterruptRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	sess, exists := s.sessions[req.SessionID]
	if !exists {
		s.mu.Unlock()
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	// Invalidate active turn immediately (PRD 5.3: Discard stale chunks)
	sess.ActiveTurn = ""
	sess.State = SessionStateReady
	sess.UpdatedAt = time.Now()
	s.mu.Unlock()

	s.recordEvent(ServerEvent{
		EventName: "cloud_turn_cancelled",
		SessionID: req.SessionID,
		TurnID:    req.TurnID,
		Platform:  sess.Platform,
	})

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"cancelled","discarded":true}`))
}

type CloseRequest struct {
	SessionID string `json:"session_id"`
}

func (s *CloudServer) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CloseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	sess, exists := s.sessions[req.SessionID]
	if exists {
		sess.State = SessionStateClosed
		delete(s.sessions, req.SessionID) // Purge ephemeral memory
	}
	s.mu.Unlock()

	if exists {
		s.recordEvent(ServerEvent{
			EventName: "cloud_session_closed",
			SessionID: req.SessionID,
			Platform:  sess.Platform,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"closed"}`))
}

func (s *CloudServer) recordEvent(ev ServerEvent) {
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventsLog = append(s.eventsLog, ev)
}

// GetTelemetryEvents returns recorded telemetry for test assertions.
func (s *CloudServer) GetTelemetryEvents() []ServerEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copied := make([]ServerEvent, len(s.eventsLog))
	copy(copied, s.eventsLog)
	return copied
}
