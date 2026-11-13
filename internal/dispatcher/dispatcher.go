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
	RoutePetAction    RouteTarget = "LOCAL_PET_ACTION"
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
	// P2: Check local pet action oral command
	if act, cue, move, ok := matchPetActionCommand(clean); ok {
		res.Target = RoutePetAction
		res.IntentName = "pet.action"
		res.Slots = map[string]interface{}{
			"action": act,
			"cue":    cue,
			"move":   move,
		}
		res.Reason = fmt.Sprintf("命中桌面宠物动作指令 [%s -> %s]", act, cue)
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

func matchPetActionCommand(text string) (action, cue, move string, ok bool) {
	clean := strings.TrimSpace(text)
	// 排除询问类复杂句
	if strings.HasPrefix(clean, "为什么") || strings.HasPrefix(clean, "怎么") || strings.HasPrefix(clean, "如何") || strings.HasPrefix(clean, "什么是") {
		return "", "", "", false
	}

	// 去除常见唤醒/称呼前缀
	prefixes := []string{
		"小黑，", "小黑,", "小黑 ", "小黑",
		"罗小黑，", "罗小黑,", "罗小黑 ", "罗小黑",
		"桌宠，", "桌宠,", "桌宠 ", "桌宠",
		"小猫，", "小猫,", "小猫 ", "小猫",
		"宠物，", "宠物,", "宠物 ", "宠物",
		"大肥鱼，", "大肥鱼,", "大肥鱼 ", "大肥鱼",
	}
	cmd := clean
	for _, p := range prefixes {
		if strings.HasPrefix(cmd, p) {
			cmd = strings.TrimSpace(strings.TrimPrefix(cmd, p))
			break
		}
	}
	if cmd == "" {
		return "", "", "", false
	}

	// 1. 移动与漫游巡视
	if strings.Contains(cmd, "向右跑") || strings.Contains(cmd, "往右跑") || strings.Contains(cmd, "去右边") || strings.Contains(cmd, "向右走") || strings.Contains(cmd, "往右走") || strings.Contains(cmd, "右边巡视") {
		return "roam_right", "running-right", "right", true
	}
	if strings.Contains(cmd, "向左跑") || strings.Contains(cmd, "往左跑") || strings.Contains(cmd, "去左边") || strings.Contains(cmd, "向左走") || strings.Contains(cmd, "往左走") || strings.Contains(cmd, "左边巡视") {
		return "roam_left", "running-left", "left", true
	}
	if strings.Contains(cmd, "巡视桌面") || strings.Contains(cmd, "去巡逻") || strings.Contains(cmd, "巡视") || strings.Contains(cmd, "巡逻") || strings.Contains(cmd, "到处逛逛") || strings.Contains(cmd, "溜达溜达") || strings.Contains(cmd, "走两步") || strings.Contains(cmd, "转悠转悠") {
		return "roam", "running", "", true
	}

	// 2. 干饭与投喂
	if strings.Contains(cmd, "疯狂干饭") || strings.Contains(cmd, "暴风吸入") || strings.Contains(cmd, "狂吃") || strings.Contains(cmd, "大口吃") {
		return "turbo", "running", "", true
	}
	if strings.Contains(cmd, "吃米饭") || strings.Contains(cmd, "吃大米饭") || strings.Contains(cmd, "干饭") || strings.Contains(cmd, "开饭") || strings.Contains(cmd, "大米饭") {
		return "rice", "running", "", true
	}
	if strings.Contains(cmd, "小鱼干") || strings.Contains(cmd, "吃鱼") || strings.Contains(cmd, "投喂") || strings.Contains(cmd, "喂食") || strings.Contains(cmd, "喂你") || strings.Contains(cmd, "吃点东西") || strings.Contains(cmd, "吃零食") || strings.Contains(cmd, "给你鱼") {
		return "feed", "running", "", true
	}

	// 3. 玩草
	if strings.Contains(cmd, "逗猫草") || strings.Contains(cmd, "逗草") || strings.Contains(cmd, "逗猫棒") || strings.Contains(cmd, "玩草") {
		return "grass", "jumping", "", true
	}

	// 4. 跳跃与翻转
	if strings.Contains(cmd, "跳一下") || strings.Contains(cmd, "跳一个") || strings.Contains(cmd, "跳跳") || strings.Contains(cmd, "翻个跟头") || strings.Contains(cmd, "翻跟头") || strings.Contains(cmd, "蹦一下") || strings.Contains(cmd, "蹦一个") || strings.Contains(cmd, "空翻") || strings.Contains(cmd, "跳起来") || strings.Contains(cmd, "转圈") || strings.Contains(cmd, "跳舞") || cmd == "跳" || cmd == "蹦" {
		return "jump", "jumping", "", true
	}

	// 5. 蹲下、坐下、趴下、躺下
	if strings.Contains(cmd, "蹲下") || strings.Contains(cmd, "蹲着") || strings.Contains(cmd, "蹲一个") || cmd == "蹲" ||
		strings.Contains(cmd, "坐下") || strings.Contains(cmd, "坐着") || strings.Contains(cmd, "坐好") || cmd == "坐" ||
		strings.Contains(cmd, "趴下") || strings.Contains(cmd, "趴着") || strings.Contains(cmd, "趴好") || cmd == "趴" ||
		strings.Contains(cmd, "躺下") || strings.Contains(cmd, "躺平") || strings.Contains(cmd, "躺着") {
		return "sit", "idle", "", true
	}

	// 6. 起立、醒醒、起床
	if strings.Contains(cmd, "站起来") || strings.Contains(cmd, "起来") || strings.Contains(cmd, "醒醒") || strings.Contains(cmd, "起床") {
		return "wake", "jumping", "", true
	}

	// 7. 叫一声 / 喵一声
	if strings.Contains(cmd, "叫一声") || strings.Contains(cmd, "喵一声") || strings.Contains(cmd, "叫一个") || strings.Contains(cmd, "学猫叫") || strings.Contains(cmd, "喵喵叫") || cmd == "叫" || cmd == "喵" {
		return "meow", "waving", "", true
	}

	// 8. 抚摸与蹭蹭
	if strings.Contains(cmd, "摸摸头") || strings.Contains(cmd, "摸摸你") || strings.Contains(cmd, "摸摸") || strings.Contains(cmd, "摸一下") || strings.Contains(cmd, "揉揉头") || strings.Contains(cmd, "蹭蹭") || strings.Contains(cmd, "顺毛") || strings.Contains(cmd, "挠挠") {
		return "pet", "jumping", "", true
	}

	// 9. 打招呼与握手
	if strings.Contains(cmd, "打个招呼") || strings.Contains(cmd, "挥手") || strings.Contains(cmd, "打招呼") || strings.Contains(cmd, "挥挥爪") || strings.Contains(cmd, "挥爪") || strings.Contains(cmd, "招招手") || strings.Contains(cmd, "握手") || strings.Contains(cmd, "握个手") || strings.Contains(cmd, "伸爪") {
		return "wave", "waving", "", true
	}

	// 10. 困惑 / 装死 / 别动
	if strings.Contains(cmd, "装死") || strings.Contains(cmd, "别动") || strings.Contains(cmd, "倒下") || strings.Contains(cmd, "晕一个") || strings.Contains(cmd, "犯错") {
		return "failed", "failed", "", true
	}

	// 11. 歪头沉思
	if strings.Contains(cmd, "歪头") || strings.Contains(cmd, "想一想") || strings.Contains(cmd, "思考一下") || strings.Contains(cmd, "琢磨琢磨") {
		return "review", "review", "", true
	}

	// 12. 睡觉与休眠
	if strings.Contains(cmd, "去睡觉") || strings.Contains(cmd, "睡觉吧") || strings.Contains(cmd, "休息吧") || strings.Contains(cmd, "打个盹") || strings.Contains(cmd, "闭嘴睡觉") || strings.Contains(cmd, "安静睡觉") {
		return "sleep", "idle", "", true
	}

	return "", "", "", false
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
