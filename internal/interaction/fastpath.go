package interaction

import (
	"fmt"
	"sync"

	"avatar/internal/dispatcher"
	"avatar/internal/memory"
	"avatar/internal/persona"
)

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
func (f *FastPathHandler) Evaluate(userInput string) (handled bool, confirmation string, newSlot *persona.PersonaSlot) {
	f.mu.RLock()
	consented := f.isCloudConsented
	online := f.isOnline
	f.mu.RUnlock()

	// 1. Fast Intent Sniffing & Dispatching (< 5us)
	if f.dispatcher != nil {
		route := f.dispatcher.Evaluate(userInput, consented, online)
		switch route.Target {
		case dispatcher.RouteLocalPersona:
			if f.personaMgr != nil {
				if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
					fmt.Printf("[端云分流] 决策: 【本地快环】%s | %s -> 100%% 端侧本地处理，未走云端\n", route.Target, route.Reason)
					return true, confirm, &slot
				}
			}

		case dispatcher.RouteLocalMemory:
			if f.vault != nil {
				res := f.vault.TryInterceptMemoryCommand(userInput)
				if res.Intercepted {
					fmt.Printf("[端云分流] 决策: 【本地快环】%s | %s -> 100%% 端侧本地处理，零云端网络开销 (Zero-Retention)\n", route.Target, route.Reason)
					return true, res.Confirmation, nil
				}
			}

		case dispatcher.RouteLocalOnly:
			fmt.Printf("[端云分流] 决策: 【隐私拦截】%s | %s -> 未走云端，本地直接回复\n", route.Target, route.Reason)
			return true, "云端实时服务尚未开启授权。为保护您的隐私，当前仅可在本地为您记录偏好与回忆。", nil

		case dispatcher.RouteOffline:
			fmt.Printf("[端云分流] 决策: 【离线拦截】%s | %s -> 未走云端，本地直接回复\n", route.Target, route.Reason)
			return true, "当前处于离线模式，云端对话暂时不可用。但您的本地记忆仍可正常响应。", nil

		case dispatcher.RouteCloudChat:
			fmt.Printf("[端云分流] 决策: 【走云端】%s | %s -> 穿透至云端实时网关 (:8088)\n", route.Target, route.Reason)
			return false, "", nil

		case dispatcher.RouteEdgeChat:
			fmt.Printf("[端云分流] 决策: 【本地端侧】%s | %s -> 走端侧小模型 (EDGE_SLM)，未走云端\n", route.Target, route.Reason)
			return false, "", nil
		}
	}

	// Fallback to direct checks if dispatcher is nil
	if f.personaMgr != nil {
		if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
			fmt.Printf("[端云分流] 决策: 【本地快环】LOCAL_PERSONA -> 100%% 端侧本地处理，未走云端\n")
			return true, confirm, &slot
		}
	}

	if f.vault != nil {
		res := f.vault.TryInterceptMemoryCommand(userInput)
		if res.Intercepted {
			fmt.Printf("[端云分流] 决策: 【本地快环】LOCAL_MEMORY -> 100%% 端侧本地处理，零云端网络开销 (Zero-Retention)\n")
			return true, res.Confirmation, nil
		}
	}
	return false, "", nil
}
