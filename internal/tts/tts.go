package tts

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
)

type Synthesizer interface {
	Speak(ctx context.Context, text string) error
	Stop()
}

type MacSaySynthesizer struct {
	Voice   string
	Rate    int
	mu      sync.Mutex
	currCmd *exec.Cmd
}

func NewMacSaySynthesizer(voice string, rate int) *MacSaySynthesizer {
	if voice == "" {
		voice = "Ting-Ting"
	}
	return &MacSaySynthesizer{
		Voice: voice,
		Rate:  rate,
	}
}

func (m *MacSaySynthesizer) Speak(ctx context.Context, text string) error {
	m.mu.Lock()
	if runtime.GOOS != "darwin" {
		m.mu.Unlock()
		return fmt.Errorf("mac_say only available on macOS")
	}

	args := []string{}
	if m.Voice != "" {
		args = append(args, "-v", m.Voice)
	}
	if m.Rate > 0 {
		args = append(args, "-r", fmt.Sprintf("%d", m.Rate))
	}
	args = append(args, text)

	cmd := exec.CommandContext(ctx, "say", args...)
	m.currCmd = cmd
	m.mu.Unlock()

	err := cmd.Run()

	m.mu.Lock()
	if m.currCmd == cmd {
		m.currCmd = nil
	}
	m.mu.Unlock()

	return err
}

func (m *MacSaySynthesizer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.currCmd != nil && m.currCmd.Process != nil {
		_ = m.currCmd.Process.Kill()
		m.currCmd = nil
	}
}

// MockSynthesizer 用于测试与通用环境兜底
type MockSynthesizer struct {
	mu     sync.Mutex
	Spoken []string
}

func NewMockSynthesizer() *MockSynthesizer {
	return &MockSynthesizer{}
}

func (m *MockSynthesizer) Speak(ctx context.Context, text string) error {
	m.mu.Lock()
	m.Spoken = append(m.Spoken, text)
	m.mu.Unlock()
	return nil
}

func (m *MockSynthesizer) Stop() {
	// no-op
}
