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

// VoiceConfigurable allows runtime adjustment of voice timbre and speaking rate.
type VoiceConfigurable interface {
	SetVoice(voice string)
	SetRate(rate int)
	GetVoice() string
	GetRate() int
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

func (m *MacSaySynthesizer) SetVoice(voice string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Voice = voice
}

func (m *MacSaySynthesizer) SetRate(rate int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Rate = rate
}

func (m *MacSaySynthesizer) GetVoice() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Voice
}

func (m *MacSaySynthesizer) GetRate() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Rate
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

// SpokenRecord tracks synthesized text along with the active voice and rate.
type SpokenRecord struct {
	Text  string
	Voice string
	Rate  int
}

// MockSynthesizer 用于测试与通用环境兜底
type MockSynthesizer struct {
	mu      sync.Mutex
	Spoken  []string
	Records []SpokenRecord
	Voice   string
	Rate    int
	stopped bool
}

func NewMockSynthesizer() *MockSynthesizer {
	return &MockSynthesizer{
		Voice: "Ting-Ting",
		Rate:  180,
	}
}

func (m *MockSynthesizer) SetVoice(voice string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Voice = voice
}

func (m *MockSynthesizer) SetRate(rate int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.Rate = rate
}

func (m *MockSynthesizer) GetVoice() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Voice
}

func (m *MockSynthesizer) GetRate() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.Rate
}

func (m *MockSynthesizer) Speak(ctx context.Context, text string) error {
	m.mu.Lock()
	m.Spoken = append(m.Spoken, text)
	m.Records = append(m.Records, SpokenRecord{
		Text:  text,
		Voice: m.Voice,
		Rate:  m.Rate,
	})
	m.stopped = false
	m.mu.Unlock()
	return nil
}

func (m *MockSynthesizer) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

func (m *MockSynthesizer) IsStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

func (m *MockSynthesizer) GetRecords() []SpokenRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]SpokenRecord, len(m.Records))
	copy(cp, m.Records)
	return cp
}

// SpeakerClient represents an in-process native speech synthesizer interface.
type SpeakerClient interface {
	Speak(ctx context.Context, text string) error
	StopSpeaking() error
}

// NativeDaemonSynthesizer coordinates speech through the native audio daemon, avoiding fork/exec overhead.
type NativeDaemonSynthesizer struct {
	speaker  SpeakerClient
	fallback Synthesizer
}

func NewNativeDaemonSynthesizer(speaker SpeakerClient, fallback Synthesizer) *NativeDaemonSynthesizer {
	return &NativeDaemonSynthesizer{
		speaker:  speaker,
		fallback: fallback,
	}
}

func (s *NativeDaemonSynthesizer) SetVoice(v string) {
	if vc, ok := s.fallback.(VoiceConfigurable); ok {
		vc.SetVoice(v)
	}
}

func (s *NativeDaemonSynthesizer) SetRate(r int) {
	if vc, ok := s.fallback.(VoiceConfigurable); ok {
		vc.SetRate(r)
	}
}

func (s *NativeDaemonSynthesizer) GetVoice() string {
	if vc, ok := s.fallback.(VoiceConfigurable); ok {
		return vc.GetVoice()
	}
	return ""
}

func (s *NativeDaemonSynthesizer) GetRate() int {
	if vc, ok := s.fallback.(VoiceConfigurable); ok {
		return vc.GetRate()
	}
	return 0
}

func (s *NativeDaemonSynthesizer) Speak(ctx context.Context, text string) error {
	if s.speaker != nil {
		if err := s.speaker.Speak(ctx, text); err == nil {
			return nil
		}
	}
	if s.fallback != nil {
		return s.fallback.Speak(ctx, text)
	}
	return nil
}

func (s *NativeDaemonSynthesizer) Stop() {
	if s.speaker != nil {
		_ = s.speaker.StopSpeaking()
	}
	if s.fallback != nil {
		s.fallback.Stop()
	}
}
