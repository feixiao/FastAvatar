package asr

import (
	"context"
	"sync"
)

// MockEngine is a test double implementing Engine.
type MockEngine struct {
	mu        sync.RWMutex
	running   bool
	mode      string
	speaking  bool
	onPartial func(string)
	onFinal   func(string)
	onVAD     func(bool, float64)
	onError   func(string)
}

func NewMockEngine() *MockEngine {
	return &MockEngine{
		mode: "hands_free",
	}
}

func (m *MockEngine) Start(ctx context.Context) error {
	m.mu.Lock()
	m.running = true
	m.mu.Unlock()
	return nil
}

func (m *MockEngine) Stop() error {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()
	return nil
}

func (m *MockEngine) SetMode(mode string) error {
	m.mu.Lock()
	m.mode = mode
	m.mu.Unlock()
	return nil
}

func (m *MockEngine) SetSpeaking(speaking bool) error {
	m.mu.Lock()
	m.speaking = speaking
	m.mu.Unlock()
	return nil
}

func (m *MockEngine) PTTStart() error {
	return nil
}

func (m *MockEngine) PTTStop() error {
	return nil
}

func (m *MockEngine) SetCallbacks(
	onPartial func(string),
	onFinal func(string),
	onVAD func(bool, float64),
	onError func(string),
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onPartial = onPartial
	m.onFinal = onFinal
	m.onVAD = onVAD
	m.onError = onError
}

// SimulateSpeech triggers callbacks as if the user spoke.
func (m *MockEngine) SimulateSpeech(partial, final string) {
	m.mu.RLock()
	onPart := m.onPartial
	onFin := m.onFinal
	onV := m.onVAD
	m.mu.RUnlock()

	if onV != nil {
		onV(true, 0.5)
	}
	if onPart != nil && partial != "" {
		onPart(partial)
	}
	if onFin != nil && final != "" {
		onFin(final)
	}
	if onV != nil {
		onV(false, 0.0)
	}
}
