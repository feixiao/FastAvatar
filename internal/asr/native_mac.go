package asr

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// NativeMacEngine manages the native macOS avatar-asr helper process.
type NativeMacEngine struct {
	locale    string
	mode      string
	binPath   string
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	mu        sync.Mutex
	running   bool
	onPartial func(string)
	onFinal   func(string)
	onVAD     func(bool, float64)
	onError   func(string)
	onState   func(string)
}

// NewNativeMacEngine creates a new macOS native ASR engine instance.
func NewNativeMacEngine(locale string, mode string) *NativeMacEngine {
	if locale == "" {
		locale = "zh-CN"
	}
	if mode == "" {
		mode = "hands_free"
	}
	return &NativeMacEngine{
		locale: locale,
		mode:   mode,
	}
}

// EnsureBinary locates or builds the avatar-asr binary.
func (e *NativeMacEngine) EnsureBinary() (string, error) {
	// 1. Check relative to current working directory and parent paths
	candidates := []string{
		"bin/avatar-asr",
		"../bin/avatar-asr",
		"../../bin/avatar-asr",
		filepath.Join(".", "avatar-asr"),
	}

	// 2. Check next to running executable
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "avatar-asr"))
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "..", "bin", "avatar-asr"))
	}

	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			e.binPath = c
			return c, nil
		}
	}

	// 3. Try to build from cmd/avatar-asr/main.swift if swiftc is present on darwin
	if runtime.GOOS == "darwin" {
		swiftCandidates := []string{
			"cmd/avatar-asr/main.swift",
			"../cmd/avatar-asr/main.swift",
			"../../cmd/avatar-asr/main.swift",
		}
		for _, src := range swiftCandidates {
			if _, err := os.Stat(src); err == nil {
				targetDir := filepath.Join(filepath.Dir(filepath.Dir(src)), "bin")
				_ = os.MkdirAll(targetDir, 0755)
				target := filepath.Join(targetDir, "avatar-asr")
				cmd := exec.Command("swiftc", "-O", "-framework", "Speech", "-framework", "AVFoundation", src, "-o", target)
				if out, err := cmd.CombinedOutput(); err != nil {
					return "", fmt.Errorf("compile avatar-asr error: %v, output: %s", err, string(out))
				}
				e.binPath = target
				return target, nil
			}
		}
	}

	return "", fmt.Errorf("avatar-asr binary not found and cannot be built")
}

func (e *NativeMacEngine) Start(ctx context.Context) error {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return nil
	}

	bin, err := e.EnsureBinary()
	if err != nil {
		e.mu.Unlock()
		return err
	}

	cmd := exec.CommandContext(ctx, bin, "--locale", e.locale, "--mode", e.mode)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("failed to open stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		e.mu.Unlock()
		return fmt.Errorf("failed to open stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		e.mu.Unlock()
		return fmt.Errorf("failed to start avatar-asr process: %w", err)
	}

	e.cmd = cmd
	e.stdin = stdin
	e.running = true
	e.mu.Unlock()

	// Background reader for stdout JSON stream
	go e.readLoop(stdout)

	return nil
}

func (e *NativeMacEngine) readLoop(stdout io.Reader) {
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var payload map[string]interface{}
		if err := json.Unmarshal(line, &payload); err != nil {
			continue
		}

		event, _ := payload["event"].(string)
		switch event {
		case "partial", "barge_in":
			if text, ok := payload["text"].(string); ok && text != "" {
				e.mu.Lock()
				cb := e.onPartial
				e.mu.Unlock()
				if cb != nil {
					cb(text)
				}
			}
		case "final":
			if text, ok := payload["text"].(string); ok && text != "" {
				e.mu.Lock()
				cb := e.onFinal
				e.mu.Unlock()
				if cb != nil {
					cb(text)
				}
			}
		case "vad":
			speaking, _ := payload["speaking"].(bool)
			energy, _ := payload["energy"].(float64)
			e.mu.Lock()
			cb := e.onVAD
			e.mu.Unlock()
			if cb != nil {
				cb(speaking, energy)
			}
		case "state":
			st, _ := payload["state"].(string)
			e.mu.Lock()
			cb := e.onState
			e.mu.Unlock()
			if cb != nil && st != "" {
				cb(st)
			}
		case "error":
			msg, _ := payload["message"].(string)
			e.mu.Lock()
			cb := e.onError
			e.mu.Unlock()
			if cb != nil {
				cb(msg)
			}
		}
	}

	e.mu.Lock()
	e.running = false
	e.mu.Unlock()
}

func (e *NativeMacEngine) sendCommand(cmd map[string]interface{}) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running || e.stdin == nil {
		return fmt.Errorf("asr engine is not running")
	}

	data, err := json.Marshal(cmd)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = e.stdin.Write(data)
	return err
}

func (e *NativeMacEngine) Stop() error {
	e.mu.Lock()
	if !e.running {
		e.mu.Unlock()
		return nil
	}
	e.running = false
	stdin := e.stdin
	cmd := e.cmd
	e.mu.Unlock()

	if stdin != nil {
		_ = json.NewEncoder(stdin).Encode(map[string]string{"cmd": "stop"})
		_ = stdin.Close()
	}

	if cmd != nil && cmd.Process != nil {
		done := make(chan error, 1)
		go func() {
			done <- cmd.Wait()
		}()

		select {
		case <-done:
		case <-time.After(1 * time.Second):
			_ = cmd.Process.Kill()
		}
	}

	return nil
}

func (e *NativeMacEngine) SetMode(mode string) error {
	e.mode = mode
	return e.sendCommand(map[string]interface{}{
		"cmd":  "set_mode",
		"mode": mode,
	})
}

func (e *NativeMacEngine) SetSpeaking(speaking bool) error {
	return e.sendCommand(map[string]interface{}{
		"cmd":      "set_speaking",
		"speaking": speaking,
	})
}

func (e *NativeMacEngine) PTTStart() error {
	return e.sendCommand(map[string]interface{}{
		"cmd": "ptt_start",
	})
}

func (e *NativeMacEngine) PTTStop() error {
	return e.sendCommand(map[string]interface{}{
		"cmd": "ptt_stop",
	})
}

func (e *NativeMacEngine) SetCallbacks(
	onPartial func(text string),
	onFinal func(text string),
	onVAD func(speaking bool, energy float64),
	onError func(err string),
) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onPartial = onPartial
	e.onFinal = onFinal
	e.onVAD = onVAD
	e.onError = onError
}

func (e *NativeMacEngine) SetStateCallback(onState func(string)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.onState = onState
}
