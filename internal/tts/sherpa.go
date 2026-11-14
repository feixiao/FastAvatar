package tts

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// SherpaOnnxSynthesizer implements Synthesizer and VoiceConfigurable
// using local offline neural VITS and Kokoro synthesis via sherpa-onnx.
type SherpaOnnxSynthesizer struct {
	binPath   string
	modelDir  string
	isKokoro  bool
	speakerID int
	speed     float64
	threads   int
	mu        sync.Mutex
	currCmd   *exec.Cmd
}

// DefaultSpeakerMap maps persona IDs or voice aliases to AISHELL-3 speaker IDs.
var DefaultSpeakerMap = map[string]int{
	"whale":       66, // 大肥鱼: 灵动少女音
	"companion":   10, // 知心伴侣: 温柔女声
	"jarvis":      0,  // 高效幕僚: 沉稳干练男声
	"roaster":     3,  // 毒舌损友: 活力男声
	"Ting-Ting":   66, // 兼容旧配置默认值
	"Tingting":    66,
	"Flo":         66,
	"Eddy":        3,
	"Reed":        0,
	"Shelley":     10,
}

// KokoroSpeakerMap maps persona IDs or voice aliases to Kokoro multi-lang speaker IDs.
var KokoroSpeakerMap = map[string]int{
	"whale":       45, // 大肥鱼: zf_xiaobei 灵动少女音
	"companion":   47, // 知心伴侣: zf_xiaoxiao 温柔知心女声
	"jarvis":      50, // 高效幕僚: zm_yunxi 沉稳干练男声
	"roaster":     49, // 毒舌损友: zm_yunjian 活力男声
	"sweet":       46, // zf_xiaoni: 甜美女声
	"gentle":      48, // zf_xiaoyi: 柔美女声
	"warm":        52, // zm_yunyang: 阳光暖男
	"zf_xiaobei":  45,
	"zf_xiaoxiao": 47,
	"zm_yunxi":    50,
	"zm_yunjian":  49,
	"zf_xiaoni":   46,
	"zf_xiaoyi":   48,
	"zm_yunyang":  52,
	// 兼容 AISHELL-3 旧配置默认映射
	"66":          45,
	"10":          47,
	"0":           50,
	"3":           49,
	"Ting-Ting":   45,
	"Tingting":    45,
}

// detectKokoro checks if the target directory contains a Kokoro model.
func detectKokoro(modelDir string) bool {
	if strings.Contains(strings.ToLower(modelDir), "kokoro") {
		return true
	}
	voicesPath := filepath.Join(modelDir, "voices.bin")
	if _, err := os.Stat(voicesPath); err == nil {
		return true
	}
	return false
}

// NewSherpaOnnxSynthesizer creates an offline neural TTS synthesizer.
func NewSherpaOnnxSynthesizer(binPath, modelDir string, defaultSpeakerID int, defaultRate int) *SherpaOnnxSynthesizer {
	if binPath == "" {
		binPath = "bin/sherpa-onnx-offline-tts-play"
	}
	if modelDir == "" {
		if _, err := os.Stat("models/kokoro-int8-multi-lang-v1_0"); err == nil {
			modelDir = "models/kokoro-int8-multi-lang-v1_0"
		} else {
			modelDir = "models/vits-zh-aishell3"
		}
	}
	isKokoro := detectKokoro(modelDir)
	speed := mapRateToSpeed(defaultRate)

	// If Kokoro is used and default speaker is from legacy AISHELL-3 (e.g. 66), map to Kokoro default (45)
	spkID := defaultSpeakerID
	if isKokoro {
		if spkID == 66 || spkID > 53 {
			spkID = 45
		}
	}

	return &SherpaOnnxSynthesizer{
		binPath:   binPath,
		modelDir:  modelDir,
		isKokoro:  isKokoro,
		speakerID: spkID,
		speed:     speed,
		threads:   4,
	}
}

func mapRateToSpeed(rate int) float64 {
	if rate <= 0 {
		return 1.0
	}
	// Say rate ~180 is normal speed 1.0. Range 100-250 -> 0.7 - 1.4
	if rate >= 50 {
		spd := 1.0 + float64(rate-175)*0.003
		if spd < 0.6 {
			spd = 0.6
		} else if spd > 1.6 {
			spd = 1.6
		}
		return spd
	}
	return float64(rate)
}

// IsKokoro returns true if the synthesizer is running Kokoro.
func (s *SherpaOnnxSynthesizer) IsKokoro() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isKokoro
}

// SetVoice sets speaker by ID or persona name.
func (s *SherpaOnnxSynthesizer) SetVoice(voice string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	voice = strings.TrimSpace(voice)

	if s.isKokoro {
		if id, ok := KokoroSpeakerMap[voice]; ok {
			s.speakerID = id
			return
		}
		if id, err := strconv.Atoi(voice); err == nil && id >= 0 && id <= 53 {
			s.speakerID = id
			return
		}
		return
	}

	if id, err := strconv.Atoi(voice); err == nil {
		s.speakerID = id
		return
	}
	if id, ok := DefaultSpeakerMap[voice]; ok {
		s.speakerID = id
		return
	}
}

// SetRate sets speed from rate.
func (s *SherpaOnnxSynthesizer) SetRate(rate int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.speed = mapRateToSpeed(rate)
}

// GetVoice returns the active speaker ID as a string.
func (s *SherpaOnnxSynthesizer) GetVoice() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return fmt.Sprintf("%d", s.speakerID)
}

// GetRate returns an estimated speech rate.
func (s *SherpaOnnxSynthesizer) GetRate() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return int(175 + (s.speed-1.0)/0.003)
}

// GetSpeakerID returns current numeric speaker ID.
func (s *SherpaOnnxSynthesizer) GetSpeakerID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.speakerID
}

// Speak synthesizes text and plays it directly to the system audio output.
func (s *SherpaOnnxSynthesizer) Speak(ctx context.Context, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	s.mu.Lock()
	var args []string

	if s.isKokoro {
		modelFile := filepath.Join(s.modelDir, "model.int8.onnx")
		if _, err := os.Stat(modelFile); os.IsNotExist(err) {
			modelFile = filepath.Join(s.modelDir, "model.onnx")
		}
		voicesFile := filepath.Join(s.modelDir, "voices.bin")
		tokensFile := filepath.Join(s.modelDir, "tokens.txt")
		lexiconFile := filepath.Join(s.modelDir, "lexicon-zh.txt")
		if _, err := os.Stat(lexiconFile); os.IsNotExist(err) {
			lexiconFile = filepath.Join(s.modelDir, "lexicon.txt")
		}

		args = []string{
			fmt.Sprintf("--kokoro-model=%s", modelFile),
			fmt.Sprintf("--kokoro-voices=%s", voicesFile),
			fmt.Sprintf("--kokoro-tokens=%s", tokensFile),
			fmt.Sprintf("--kokoro-lexicon=%s", lexiconFile),
			fmt.Sprintf("--sid=%d", s.speakerID),
			fmt.Sprintf("--speed=%.2f", s.speed),
			fmt.Sprintf("--num-threads=%d", s.threads),
		}
	} else {
		modelFile := filepath.Join(s.modelDir, "vits-aishell3.int8.onnx")
		if _, err := os.Stat(modelFile); os.IsNotExist(err) {
			modelFile = filepath.Join(s.modelDir, "vits-aishell3.onnx")
		}
		tokensFile := filepath.Join(s.modelDir, "tokens.txt")
		lexiconFile := filepath.Join(s.modelDir, "lexicon.txt")
		ruleFarFile := filepath.Join(s.modelDir, "rule.far")

		args = []string{
			fmt.Sprintf("--vits-model=%s", modelFile),
			fmt.Sprintf("--vits-tokens=%s", tokensFile),
			fmt.Sprintf("--vits-lexicon=%s", lexiconFile),
			fmt.Sprintf("--sid=%d", s.speakerID),
			fmt.Sprintf("--speed=%.2f", s.speed),
			fmt.Sprintf("--num-threads=%d", s.threads),
		}
		if _, err := os.Stat(ruleFarFile); err == nil {
			args = append(args, fmt.Sprintf("--tts-rule-fars=%s", ruleFarFile))
		}
	}

	args = append(args, text)

	cmd := exec.CommandContext(ctx, s.binPath, args...)
	s.currCmd = cmd
	s.mu.Unlock()

	err := cmd.Run()

	s.mu.Lock()
	if s.currCmd == cmd {
		s.currCmd = nil
	}
	s.mu.Unlock()

	return err
}

// Stop terminates ongoing synthesis and audio playback immediately.
func (s *SherpaOnnxSynthesizer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.currCmd != nil && s.currCmd.Process != nil {
		_ = s.currCmd.Process.Kill()
		s.currCmd = nil
	}
}
