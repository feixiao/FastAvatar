package dispatcher

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"avatar/internal/audioagent"
)

// RouteTarget defines where the conversational turn should be dispatched.
type RouteTarget string

const (
	RouteLocalMemory  RouteTarget = "LOCAL_MEMORY"
	RouteLocalPersona RouteTarget = "LOCAL_PERSONA"
	RoutePetAction    RouteTarget = "LOCAL_PET_ACTION"
	RouteEdgeChat     RouteTarget = "EDGE_CHAT"  // 本地端侧小模型 (SLM: 极速闲聊/情绪安抚/日常问候)
	RouteCloudChat    RouteTarget = "CLOUD_CHAT" // 云端大模型 (LLM: 复杂推理/长文生成/代码/任务规划)
	RouteLocalOnly    RouteTarget = "LOCAL_ONLY_PROMPT"
	RouteOffline      RouteTarget = "OFFLINE_PROMPT"
)

// DispatchResult carries routing decision, extracted intent, and extracted slots.
type DispatchResult struct {
	Target        RouteTarget
	IntentName    string
	PrimaryDomain string
	Slots         map[string]interface{}
	Confidence    float64
	EvaluatedAt   time.Time
	Latency       time.Duration
	Reason        string
}

type cppDispatchPayload struct {
	Domain           string  `json:"domain"`
	Intent           string  `json:"intent"`
	Action           string  `json:"action"`
	Direction        string  `json:"direction"`
	Distance         float64 `json:"distance"`
	Persona          string  `json:"persona"`
	NeedCloud        bool    `json:"need_cloud"`
	CloudProbability float64 `json:"cloud_probability"`
	PrimaryDomain    string  `json:"primary_domain"`
	Reason           string  `json:"reason"`
}

// FastDispatcher delegates intent sniffing and TinyIntentClassifier complexity evaluation to the C++ core.
type FastDispatcher struct {
	mu         sync.RWMutex
	lastRoute  string
	lastDomain string
}

// NewFastDispatcher creates a new dispatcher instance.
func NewFastDispatcher() *FastDispatcher {
	return &FastDispatcher{}
}

// UpdateContext updates the multi-turn DST context (route and domain from the previous turn).
func (d *FastDispatcher) UpdateContext(lastRoute, lastDomain string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.lastRoute = lastRoute
	d.lastDomain = lastDomain
}

// Evaluate performs deterministic rule-based intent sniffing and C++ TinyIntentClassifier evaluation.
func (d *FastDispatcher) Evaluate(text string, isCloudConsented bool, isOnline bool) DispatchResult {
	d.mu.RLock()
	lr := d.lastRoute
	ld := d.lastDomain
	d.mu.RUnlock()
	return d.EvaluateWithContext(text, isCloudConsented, isOnline, lr, ld)
}

// EvaluateWithContext evaluates user input with explicit multi-turn DST context via C++ core.
func (d *FastDispatcher) EvaluateWithContext(
	text string,
	isCloudConsented bool,
	isOnline bool,
	lastRoute string,
	lastDomain string,
) DispatchResult {
	start := time.Now()
	clean := strings.TrimSpace(text)

	res := DispatchResult{
		EvaluatedAt: start,
		Confidence:  1.0,
	}

	rawJSON := audioagent.DispatchIntentContext(clean, lastRoute, lastDomain)
	var payload cppDispatchPayload
	if rawJSON != "" {
		_ = json.Unmarshal([]byte(rawJSON), &payload)
	}

	switch payload.Domain {
	case "MEMORY_COMMAND":
		res.Target = RouteLocalMemory
		res.IntentName = "memory.command"
		res.Reason = "C++ IntentArbiter 命中本地记忆指令"
		res.Latency = time.Since(start)
		return res

	case "ACTION_COMMAND":
		cue, petAction := mapCoreActionToPresentation(payload.Action, payload.Direction)
		res.Target = RoutePetAction
		res.IntentName = "pet.action"
		res.Slots = map[string]interface{}{
			"action":    petAction,
			"cue":       cue,
			"move":      payload.Direction,
			"distance":  payload.Distance,
		}
		res.Reason = fmt.Sprintf("C++ IntentArbiter 命中动作元语 [%s -> %s]", payload.Action, cue)
		res.Latency = time.Since(start)
		return res

	case "PERSONA_SWITCH":
		res.Target = RouteLocalPersona
		res.IntentName = "persona.switch"
		res.Slots = map[string]interface{}{
			"persona": payload.Persona,
		}
		res.Reason = "C++ IntentArbiter 命中口语人设切换指令"
		res.Latency = time.Since(start)
		return res
	}

	// Open-ended conversational query (evaluated by C++ TinyIntentClassifier)
	res.IntentName = "dialog.open_ended"
	res.PrimaryDomain = payload.PrimaryDomain

	if !isCloudConsented {
		res.Target = RouteLocalOnly
		res.Reason = "用户未开启联网大模型授权 (cloud_consent=false)，纯离线隐私保护"
		res.Latency = time.Since(start)
		return res
	}

	if !isOnline {
		res.Target = RouteOffline
		res.Reason = "客户端处于离线状态 (isOnline=false)"
		res.Latency = time.Since(start)
		return res
	}

	if payload.NeedCloud {
		res.Target = RouteCloudChat
		res.Confidence = payload.CloudProbability
		res.Reason = fmt.Sprintf("[TinyClassifier P=%.2f | %s] %s", payload.CloudProbability, payload.PrimaryDomain, payload.Reason)
	} else {
		res.Target = RouteEdgeChat
		res.Confidence = 1.0 - payload.CloudProbability
		res.Reason = fmt.Sprintf("[TinyClassifier P=%.2f | %s] %s", payload.CloudProbability, payload.PrimaryDomain, payload.Reason)
	}
	res.Latency = time.Since(start)
	return res
}

// mapCoreActionToPresentation 将 C++ 核心库输出的抽象动作元语映射为桌面端表现层动画轨与事件
func mapCoreActionToPresentation(coreAction, direction string) (cue string, petAction string) {
	switch coreAction {
	case "sit":
		return "waiting", "sit"
	case "stand":
		return "jumping", "wake"
	case "jump":
		return "jumping", "jump"
	case "roll":
		return "jumping", "roll"
	case "rest":
		return "idle", "sleep"
	case "move_left":
		return "running-left", "roam_left"
	case "move_right":
		return "running-right", "roam_right"
	case "move_forward":
		return "running", "roam"
	case "stop":
		return "failed", "failed"
	case "greet":
		return "waving", "wave"
	case "celebrate":
		return "jumping", "pet"
	case "confused":
		return "failed", "failed"
	case "ponder":
		return "review", "review"
	case "vocalize":
		return "waving", "meow"
	case "consume":
		return "running", "feed"
	default:
		if direction == "left" {
			return "running-left", "roam_left"
		}
		if direction == "right" {
			return "running-right", "roam_right"
		}
		return "jumping", coreAction
	}
}

// CheckComplexityWithContext uses the C++ TinyIntentClassifier with multi-turn DST context.
func CheckComplexityWithContext(text, lastRoute, lastDomain string) (bool, string, string) {
	clean := strings.TrimSpace(text)
	rawJSON := audioagent.DispatchIntentContext(clean, lastRoute, lastDomain)
	if rawJSON == "" {
		return false, "chat", "日常问候/轻量闲聊"
	}
	var payload cppDispatchPayload
	if err := json.Unmarshal([]byte(rawJSON), &payload); err != nil {
		return false, "chat", "日常问候/轻量闲聊"
	}
	domain := payload.PrimaryDomain
	if domain == "" {
		domain = "chat"
	}
	reason := payload.Reason
	if reason == "" {
		reason = "TinyIntentClassifier 分类决策"
	}
	return payload.NeedCloud, fmt.Sprintf("[P=%.2f|%s] %s", payload.CloudProbability, domain, reason), domain
}

// CheckComplexity delegates to the C++ TinyIntentClassifier (single-turn compatibility wrapper).
func CheckComplexity(text string) (bool, string) {
	isComplex, reason, _ := CheckComplexityWithContext(text, "", "")
	return isComplex, reason
}

// IsComplexQuery 判定输入是否属于需要云端大模型进行深度推理/长文本生成的复杂问题
func IsComplexQuery(text string) bool {
	isComplex, _ := CheckComplexity(text)
	return isComplex
}
