package server

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
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
}

// NewCloudServer creates a new cloud realtime server.
func NewCloudServer(tokenService *TokenService) *CloudServer {
	return &CloudServer{
		tokenService: tokenService,
		sessions:     make(map[string]*ActiveSession),
	}
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
}

type ChatResponse struct {
	TurnID    string `json:"turn_id"`
	Response  string `json:"response"`
	LatencyMS int64  `json:"latency_ms"`
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
	reply := s.generateReply(req.Message, req.Persona)
	latency := time.Since(start).Milliseconds()

	s.recordEvent(ServerEvent{
		EventName:  "cloud_turn_completed",
		SessionID:  req.SessionID,
		TurnID:     req.TurnID,
		Platform:   "gateway",
		DurationMS: latency,
	})

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(ChatResponse{
		TurnID:    req.TurnID,
		Response:  reply,
		LatencyMS: latency,
	})
}

func (s *CloudServer) generateReply(msg, persona string) string {
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
