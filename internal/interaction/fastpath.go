package interaction

import (
	"avatar/internal/memory"
	"avatar/internal/persona"
)

// FastPathHandler checks for deterministic, low-latency oral commands.
type FastPathHandler struct {
	vault      *memory.Vault
	personaMgr *persona.Manager
}

// NewFastPathHandler creates a new fast path handler.
func NewFastPathHandler(vault *memory.Vault, personaMgr *persona.Manager) *FastPathHandler {
	return &FastPathHandler{
		vault:      vault,
		personaMgr: personaMgr,
	}
}

// CheckSchedule checks if persona auto schedule triggered a slot rotation.
func (f *FastPathHandler) CheckSchedule() (bool, persona.PersonaSlot) {
	if f.personaMgr == nil {
		return false, persona.PersonaSlot{}
	}
	return f.personaMgr.CheckSchedule()
}

// Evaluate checks if the user input matches a deterministic fast-path command.
func (f *FastPathHandler) Evaluate(userInput string) (handled bool, confirmation string, newSlot *persona.PersonaSlot) {
	// 1. 口语切换人格 (0.1ms 快速拦截)
	if f.personaMgr != nil {
		if intercepted, confirm, slot := f.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
			return true, confirm, &slot
		}
	}

	// 2. 口语记忆指令 ("记住..." / "忘掉...") (0.1ms 快速拦截)
	if f.vault != nil {
		res := f.vault.TryInterceptMemoryCommand(userInput)
		if res.Intercepted {
			return true, res.Confirmation, nil
		}
	}

	return false, "", nil
}
