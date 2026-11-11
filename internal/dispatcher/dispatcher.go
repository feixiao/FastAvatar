package dispatcher

import (
	"fmt"
	"strings"
	"time"
)

// RouteTarget defines where the conversational turn should be dispatched.
type RouteTarget string

const (
	RouteLocalMemory  RouteTarget = "LOCAL_MEMORY"
	RouteLocalPersona RouteTarget = "LOCAL_PERSONA"
	RouteEdgeChat     RouteTarget = "EDGE_CHAT"  // 本地端侧小模型 (SLM: 极速闲聊/情绪安抚/日常问候)
	RouteCloudChat    RouteTarget = "CLOUD_CHAT" // 云端大模型 (LLM: 复杂推理/长文生成/代码/任务规划)
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
	Reason       string
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

	// P1: Check local memory oral command
	if isMemoryCommand(clean) {
		res.Target = RouteLocalMemory
		res.IntentName = "memory.command"
		res.Reason = "命中口语记忆指令前缀"
		res.Latency = time.Since(start)
		return res
	}

	// P3: Check local persona switch oral command
	if isPersonaSwitchCommand(clean) {
		res.Target = RouteLocalPersona
		res.IntentName = "persona.switch"
		res.Reason = "命中口语人设切换指令前缀"
		res.Latency = time.Since(start)
		return res
	}

	// P4: Open-ended conversational query (Cloud candidate)
	res.IntentName = "dialog.open_ended"

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

	isComplex, reason := CheckComplexity(clean)
	if isComplex {
		res.Target = RouteCloudChat
		res.Reason = reason
	} else {
		res.Target = RouteEdgeChat
		res.Reason = reason
	}
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

// CheckComplexity 判定输入是否属于需要云端大模型进行深度推理/长文本生成的复杂问题，并返回判定原因
func CheckComplexity(text string) (bool, string) {
	clean := strings.TrimSpace(text)
	complexKeywords := []string{
		"为什么", "如何", "怎么做", "分析", "原因", "解释", "比较",
		"代码", "写一段", "编写", "实现", "设计", "架构", "算法",
		"规划", "翻译", "总结", "大纲", "撰写", "论文", "方案", "步骤",
		"什么意思", "怎么回事", "凭什么", "到底", "评价", "建议", "看法",
	}
	for _, kw := range complexKeywords {
		if strings.Contains(clean, kw) {
			return true, fmt.Sprintf("命中复杂认知关键词 %q", kw)
		}
	}
	// 超过 25 字符通常包含多重上下文约束或长篇表达，转入云端大模型
	if runeCount := len([]rune(clean)); runeCount > 25 {
		return true, fmt.Sprintf("文本长度(%d字)超过25字阈值，需云端大模型深度推理", runeCount)
	}
	return false, "日常问候/轻量闲聊"
}

// IsComplexQuery 判定输入是否属于需要云端大模型进行深度推理/长文本生成的复杂问题
func IsComplexQuery(text string) bool {
	isComplex, _ := CheckComplexity(text)
	return isComplex
}
