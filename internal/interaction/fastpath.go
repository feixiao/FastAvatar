package interaction

import (
	"sync"

	"avatar/internal/dispatcher"
	"avatar/internal/memory"
	"avatar/internal/mockvehicle"
	"avatar/internal/persona"
)

// FastPathHandler checks for deterministic, low-latency oral commands.
type FastPathHandler struct {
	vault            *memory.Vault
	personaMgr       *persona.Manager
	dispatcher       *dispatcher.FastDispatcher
	vehicleEngine    *mockvehicle.Engine
	mu               sync.RWMutex
	isCloudConsented bool
	isOnline         bool
}

// NewFastPathHandler creates a new fast path handler.
func NewFastPathHandler(vault *memory.Vault, personaMgr *persona.Manager) *FastPathHandler {
	vehicle, _ := mockvehicle.NewEngine("")
	return &FastPathHandler{
		vault:            vault,
		personaMgr:       personaMgr,
		dispatcher:       dispatcher.NewFastDispatcher(),
		vehicleEngine:    vehicle,
		isCloudConsented: true, // Default enabled for developer desktop, controllable via SetCloudConsent
		isOnline:         true,
	}
}

// SetVehicleEngine overrides default mock vehicle engine (useful for custom logs in tests).
func (f *FastPathHandler) SetVehicleEngine(engine *mockvehicle.Engine) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vehicleEngine = engine
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
					return true, confirm, &slot
				}
			}

		case dispatcher.RouteLocalMemory:
			if f.vault != nil {
				res := f.vault.TryInterceptMemoryCommand(userInput)
				if res.Intercepted {
					return true, res.Confirmation, nil
				}
			}

		case dispatcher.RouteMockVehicle:
			if f.vehicleEngine != nil {
				res, err := f.vehicleEngine.Execute(route.IntentName, route.Slots)
				if err == nil {
					return true, res.Confirmation, nil
				}
				return true, "模拟车控处理异常：" + err.Error(), nil
			}

		case dispatcher.RouteLocalOnly:
			return true, "云端实时服务尚未开启授权。为保护您的隐私，当前仅可在本地为您记录偏好与模拟车控。", nil

		case dispatcher.RouteOffline:
			return true, "当前处于离线模式，云端对话暂时不可用。但您的本地记忆与模拟车控仍可正常响应。", nil

		case dispatcher.RouteCloudChat:
			// Let it pass to background task runner or LiveKit cloud realtime agent
			return false, "", nil
		}
	}

	// Fallback to direct checks if dispatcher is nil
	if f.personaMgr != nil {
		if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
			return true, confirm, &slot
		}
	}

	if f.vault != nil {
		res := f.vault.TryInterceptMemoryCommand(userInput)
		if res.Intercepted {
			return true, res.Confirmation, nil
		}
	}

	return false, "", nil
}
