package dispatcher

import (
	"strings"
	"time"

	"avatar/internal/mockvehicle"
)

// RouteTarget defines where the conversational turn should be dispatched.
type RouteTarget string

const (
	RouteLocalMemory  RouteTarget = "LOCAL_MEMORY"
	RouteLocalPersona RouteTarget = "LOCAL_PERSONA"
	RouteMockVehicle  RouteTarget = "MOCK_VEHICLE"
	RouteCloudChat    RouteTarget = "CLOUD_CHAT"
	RouteLocalOnly    RouteTarget = "LOCAL_ONLY_PROMPT"
	RouteOffline      RouteTarget = "OFFLINE_PROMPT"
)

// DispatchResult carries routing decision, extracted intent, and extracted slots.
type DispatchResult struct {
	Target       RouteTarget
	IntentName   string
	Slots        map[string]interface{}
	Confidence   float64
	EvaluatedAt  time.Time
	Latency      time.Duration
}

// FastDispatcher evaluates user input in < 5ms to route between local fast loop and cloud loop.
type FastDispatcher struct{}

// NewFastDispatcher creates a new dispatcher instance.
func NewFastDispatcher() *FastDispatcher {
	return &FastDispatcher{}
}

// Evaluate performs deterministic rule-based intent sniffing and slot extraction.
// Timing guarantee: < 1ms on desktop/mobile CPUs.
func (d *FastDispatcher) Evaluate(text string, isCloudConsented bool, isOnline bool) DispatchResult {
	start := time.Now()
	clean := strings.TrimSpace(text)

	res := DispatchResult{
		EvaluatedAt: start,
		Confidence:  1.0,
	}

	// P0/P1: Check local persona switch oral command
	if isPersonaSwitchCommand(clean) {
		res.Target = RouteLocalPersona
		res.IntentName = "persona.switch"
		res.Latency = time.Since(start)
		return res
	}

	// P1: Check local memory oral command
	if isMemoryCommand(clean) {
		res.Target = RouteLocalMemory
		res.IntentName = "memory.command"
		res.Latency = time.Since(start)
		return res
	}

	// P2: Check mock vehicle intent
	if intent, slots, matched := mockvehicle.ParseIntentFromText(clean); matched {
		res.Target = RouteMockVehicle
		res.IntentName = intent
		res.Slots = slots
		res.Latency = time.Since(start)
		return res
	}

	// P3: Open-ended conversational query (Cloud candidate)
	res.IntentName = "dialog.open_ended"

	if !isCloudConsented {
		res.Target = RouteLocalOnly
		res.Latency = time.Since(start)
		return res
	}

	if !isOnline {
		res.Target = RouteOffline
		res.Latency = time.Since(start)
		return res
	}

	res.Target = RouteCloudChat
	res.Latency = time.Since(start)
	return res
}

func isPersonaSwitchCommand(text string) bool {
	return strings.HasPrefix(text, "切换成") ||
		strings.HasPrefix(text, "切换为") ||
		strings.HasPrefix(text, "变成") ||
		strings.HasPrefix(text, "换成") ||
		strings.HasPrefix(text, "切换到")
}

func isMemoryCommand(text string) bool {
	return strings.HasPrefix(text, "记住") ||
		strings.HasPrefix(text, "帮我记一下") ||
		strings.HasPrefix(text, "记一下") ||
		strings.HasPrefix(text, "忘掉") ||
		strings.HasPrefix(text, "忘了") ||
		strings.HasPrefix(text, "清除记忆")
}
