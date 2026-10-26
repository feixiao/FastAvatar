package persona

import (
	"time"
)

// CheckSchedule 根据当前系统时间流转人格
func (m *Manager) CheckSchedule() (changed bool, newSlot PersonaSlot) {
	m.mu.RLock()
	if !m.data.AutoSchedule {
		m.mu.RUnlock()
		return false, PersonaSlot{}
	}
	activeID := m.data.ActiveSlotID
	m.mu.RUnlock()

	now := time.Now()
	hour := now.Hour()

	targetID := "companion"
	if hour >= 9 && hour < 18 {
		targetID = "jarvis"
	}

	if targetID != activeID {
		slot, err := m.SetActiveSlot(targetID)
		if err == nil {
			return true, slot
		}
	}
	return false, PersonaSlot{}
}
