package interaction

import (
	"fmt"
	"sync"

	"avatar/internal/dispatcher"
	"avatar/internal/memory"
	"avatar/internal/persona"
)

type PetActionPayload struct {
	Action  string `json:"action"`
	Cue     string `json:"cue"`
	Move    string `json:"move,omitempty"`
	Message string `json:"message"`
	XP      int    `json:"xp"`
}

// FastPathHandler checks for deterministic, low-latency oral commands.
type FastPathHandler struct {
	vault            *memory.Vault
	personaMgr       *persona.Manager
	dispatcher       *dispatcher.FastDispatcher
	mu               sync.RWMutex
	isCloudConsented bool
	isOnline         bool
}

// NewFastPathHandler creates a new fast path handler.
func NewFastPathHandler(vault *memory.Vault, personaMgr *persona.Manager) *FastPathHandler {
	return &FastPathHandler{
		vault:            vault,
		personaMgr:       personaMgr,
		dispatcher:       dispatcher.NewFastDispatcher(),
		isCloudConsented: true, // Default enabled for developer desktop, controllable via SetCloudConsent
		isOnline:         true,
	}
}

// SetCloudConsent sets whether user has consented to cloud processing.
func (f *FastPathHandler) SetCloudConsent(consented bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isCloudConsented = consented
}

// SetOnlineStatus sets whether network is currently online.
func (f *FastPathHandler) SetOnlineStatus(online bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.isOnline = online
}

// CheckSchedule checks if persona auto schedule triggered a slot rotation.
func (f *FastPathHandler) CheckSchedule() (bool, persona.PersonaSlot) {
	if f.personaMgr == nil {
		return false, persona.PersonaSlot{}
	}
	return f.personaMgr.CheckSchedule()
}

// Evaluate checks if the user input matches a deterministic fast-path command or needs prompt.
func (f *FastPathHandler) Evaluate(userInput string) (handled bool, confirmation string, newSlot *persona.PersonaSlot, petAction *PetActionPayload) {
	f.mu.RLock()
	consented := f.isCloudConsented
	online := f.isOnline
	f.mu.RUnlock()

	// 1. Fast Intent Sniffing & Dispatching (< 5us)
	if f.dispatcher != nil {
		route := f.dispatcher.Evaluate(userInput, consented, online)
		switch route.Target {
		case dispatcher.RoutePetAction:
			act, _ := route.Slots["action"].(string)
			cue, _ := route.Slots["cue"].(string)
			move, _ := route.Slots["move"].(string)
			payload, confirm := buildPetActionResponse(act, cue, move)
			fmt.Printf("[端云分流] 决策: 【本地快环】%s | %s -> 100%% 端侧本地响应，未走云端\n", route.Target, route.Reason)
			return true, confirm, nil, &payload

		case dispatcher.RouteLocalPersona:
			if f.personaMgr != nil {
				if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
					fmt.Printf("[端云分流] 决策: 【本地快环】%s | %s -> 100%% 端侧本地处理，未走云端\n", route.Target, route.Reason)
					return true, confirm, &slot, nil
				}
			}

		case dispatcher.RouteLocalMemory:
			if f.vault != nil {
				res := f.vault.TryInterceptMemoryCommand(userInput)
				if res.Intercepted {
					fmt.Printf("[端云分流] 决策: 【本地快环】%s | %s -> 100%% 端侧本地处理，零云端网络开销 (Zero-Retention)\n", route.Target, route.Reason)
					return true, res.Confirmation, nil, nil
				}
			}

		case dispatcher.RouteLocalOnly:
			fmt.Printf("[端云分流] 决策: 【隐私拦截】%s | %s -> 未走云端，本地直接回复\n", route.Target, route.Reason)
			return true, "云端实时服务尚未开启授权。为保护您的隐私，当前仅可在本地为您记录偏好与回忆。", nil, nil

		case dispatcher.RouteOffline:
			fmt.Printf("[端云分流] 决策: 【离线拦截】%s | %s -> 未走云端，本地直接回复\n", route.Target, route.Reason)
			return true, "当前处于离线模式，云端对话暂时不可用。但您的本地记忆仍可正常响应。", nil, nil

		case dispatcher.RouteCloudChat:
			fmt.Printf("[端云分流] 决策: 【走云端】%s | %s -> 穿透至云端实时网关 (:8088)\n", route.Target, route.Reason)
			return false, "", nil, nil

		case dispatcher.RouteEdgeChat:
			fmt.Printf("[端云分流] 决策: 【本地端侧】%s | %s -> 走端侧小模型 (EDGE_SLM)，未走云端\n", route.Target, route.Reason)
			return false, "", nil, nil
		}
	}

	// Fallback to direct checks if dispatcher is nil
	if f.personaMgr != nil {
		if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
			fmt.Printf("[端云分流] 决策: 【本地快环】LOCAL_PERSONA -> 100%% 端侧本地处理，未走云端\n")
			return true, confirm, &slot, nil
		}
	}

	if f.vault != nil {
		res := f.vault.TryInterceptMemoryCommand(userInput)
		if res.Intercepted {
			fmt.Printf("[端云分流] 决策: 【本地快环】LOCAL_MEMORY -> 100%% 端侧本地处理，零云端网络开销 (Zero-Retention)\n")
			return true, res.Confirmation, nil, nil
		}
	}
	return false, "", nil, nil
}

func buildPetActionResponse(act, cue, move string) (PetActionPayload, string) {
	msg := "高兴地跳了起来！"
	confirm := "" // 物理动作与肢体互动默认静默执行，只动不说，杜绝模态冗余与听觉打扰
	xp := 5
	switch act {
	case "jump":
		msg = "欢快地纵身一跃，在空中翻了个利落的跟头！(亲密度 +5!)"
		xp = 5
	case "roll":
		msg = "罗小黑欢快地在桌面上打了个滚，骨碌碌翻了一圈！(亲密度 +5!)"
		xp = 5
	case "feed":
		msg = "罗小黑敏捷暴风干饭，一口吞下小鱼干！(亲密度 +5!)"
		xp = 5
	case "rice":
		msg = "狼吞虎咽狂吃大米饭，满足地打了个饱嗝~ (亲密度 +5!)"
		xp = 5
	case "turbo":
		msg = "罗小黑飞身扑向食物，两只爪子疯狂刨饭！(亲密度 +5!)"
		xp = 5
	case "grass":
		msg = "罗小黑敏捷纵跳扑住逗猫草，耳朵开心地抖了抖~ (亲密度 +5!)"
		xp = 5
	case "wave":
		msg = "开心地挥了挥爪爪打招呼~ (亲密度 +5!)"
		xp = 5
	case "pet":
		msg = "舒服地眯起黄绿大眼贴了贴你，尾巴轻轻摇晃~ (亲密度 +5!)"
		xp = 5
	case "roam_right":
		msg = "迈着轻快猫步向右边巡视小跑~"
		xp = 2
	case "roam_left":
		msg = "迈着轻快猫步向左边巡视小跑~"
		xp = 2
	case "roam":
		msg = "罗小黑开始在桌面上欢快漫游巡视~"
		xp = 2
	case "failed":
		msg = "两眼一翻倒在桌面上装死，一动不动~"
		xp = 2
	case "review":
		msg = "歪过小脑瓜，黄绿大眼睛凝神审视着屏幕~"
		xp = 2
	case "sleep":
		msg = "罗小黑收起小爪子趴下打盹，进入香甜梦乡~"
		xp = 2
	case "sit":
		msg = "罗小黑乖巧蹲坐下来，大眼睛安静地望着你~"
		xp = 3
	case "wake":
		msg = "罗小黑敏捷纵跳起身，精神抖擞！"
		xp = 3
	case "meow":
		confirm = "喵呜~"
		msg = "罗小黑仰起小脑袋软软地叫了一声：喵呜~"
		xp = 3
	}
	return PetActionPayload{
		Action:  act,
		Cue:     cue,
		Move:    move,
		Message: msg,
		XP:      xp,
	}, confirm
}
