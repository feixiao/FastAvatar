package orchestrator

import (
	"context"
	"time"
)

type SessionID string
type TurnID string
type TaskID string

type State string

const (
	StateIdle      State = "idle"
	StateListening State = "listening"
	StateThinking  State = "thinking"
	StateSpeaking  State = "speaking"
)

type TaskState string

const (
	TaskStateCreated             TaskState = "created"
	TaskStateUnderstanding       TaskState = "understanding"
	TaskStatePlanning            TaskState = "planning"
	TaskStateWaitingConfirmation TaskState = "waiting_confirmation"
	TaskStateExecuting           TaskState = "executing"
	TaskStateVerifying           TaskState = "verifying"
	TaskStateCompleted           TaskState = "completed"
	TaskStateFailed              TaskState = "failed"
	TaskStateCancelled           TaskState = "cancelled"
)

type InputType string

const (
	InputTypeText  InputType = "text"
	InputTypeVoice InputType = "voice"
)

type Stability string

const (
	StabilityInterim Stability = "interim"
	StabilityFinal   Stability = "final"
)

// UserInput represents input received from user (voice or text).
type UserInput struct {
	InputType   InputType     `json:"input_type"`
	Text        string        `json:"text"`
	AudioHandle string        `json:"audio_handle,omitempty"`
	TimeRange   time.Duration `json:"time_range,omitempty"`
	SpeakerHint string        `json:"speaker_hint,omitempty"`
	Stability   Stability     `json:"stability"`
}

// Turn tracks a single interaction turn.
type Turn struct {
	TurnID    TurnID    `json:"turn_id"`
	SessionID SessionID `json:"session_id"`
	UserGoal  string    `json:"user_goal"`
	Revision  int       `json:"revision"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Task tracks a background cognitive execution task.
type Task struct {
	TaskID       TaskID             `json:"task_id"`
	TurnID       TurnID             `json:"turn_id"`
	SessionID    SessionID          `json:"session_id"`
	GoalSnapshot string             `json:"goal_snapshot"`
	ContextRef   string             `json:"context_ref,omitempty"`
	Deadline     time.Time          `json:"deadline,omitempty"`
	RiskLevel    string             `json:"risk_level,omitempty"`
	CancelFunc   context.CancelFunc `json:"-"`
}

// TaskEvent reports execution progress or state changes.
type TaskEvent struct {
	Seq                int64     `json:"seq"`
	TaskID             TaskID    `json:"task_id"`
	TurnID             TurnID    `json:"turn_id"`
	SessionID          SessionID `json:"session_id"`
	State              TaskState `json:"state"`
	Progress           float64   `json:"progress,omitempty"`
	ConfirmationPrompt string    `json:"confirmation_prompt,omitempty"`
	Result             string    `json:"result,omitempty"`
	Error              error     `json:"error,omitempty"`
	Timestamp          time.Time `json:"timestamp"`
}

// ToolCall describes a tool execution request.
type ToolCall struct {
	ToolID         string         `json:"tool_id"`
	Version        string         `json:"version"`
	Params         map[string]any `json:"params"`
	IdempotencyKey string         `json:"idempotency_key"`
	Permission     string         `json:"permission"`
}

// ToolResult contains the output of a tool execution.
type ToolResult struct {
	ToolID         string `json:"tool_id"`
	Success        bool   `json:"success"`
	Data           any    `json:"data,omitempty"`
	Error          string `json:"error,omitempty"`
	IdempotencyKey string `json:"idempotency_key"`
}

// Playback tracks speech synthesis playback state.
type Playback struct {
	PlaybackID  string        `json:"playback_id"`
	Text        string        `json:"text"`
	StartTime   time.Time     `json:"start_time"`
	EndTime     time.Time     `json:"end_time"`
	Duration    time.Duration `json:"duration"`
	PlayedText  string        `json:"played_text"`
	Completed   bool          `json:"completed"`
	Interrupted bool          `json:"interrupted"`
}

// ModelEvent represents a chunk or event from model streaming.
type ModelEvent struct {
	Delta string
	Done  bool
	Error error
}

// ModelRequest represents a request to the model provider.
type ModelRequest struct {
	Messages []ModelMessage
}

type ModelMessage struct {
	Role    string
	Content string
}

// Core interfaces defined in fastavatar_modular_design.md
type Interaction interface {
	Submit(ctx context.Context, input UserInput) (TurnID, error)
	Interrupt(ctx context.Context, sessionID SessionID, turnID TurnID) error
}

type TaskRunner interface {
	Run(ctx context.Context, task Task, emit func(TaskEvent)) error
}

type Model interface {
	Stream(ctx context.Context, request ModelRequest) (<-chan ModelEvent, error)
}

type ToolExecutor interface {
	Execute(ctx context.Context, call ToolCall, idempotencyKey string) (ToolResult, error)
}

type SpeechOutput interface {
	Speak(ctx context.Context, text string) (Playback, error)
	Stop(ctx context.Context) error
}
