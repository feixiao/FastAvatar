package sidecar

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

type LlamaManager struct {
	serverBin string
	modelPath string
	port      int
	cmd       *exec.Cmd
	mu        sync.Mutex
}

func NewLlamaManager(serverBin, modelPath string, port int) *LlamaManager {
	if port <= 0 {
		port = 50088
	}
	return &LlamaManager{
		serverBin: serverBin,
		modelPath: modelPath,
		port:      port,
	}
}

// IsPortActive 检查目标端口是否已有服务在监听响应
func (m *LlamaManager) IsPortActive(ctx context.Context) bool {
	client := &http.Client{Timeout: 800 * time.Millisecond}
	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("http://127.0.0.1:%d/health", m.port), nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Start 启动后台伴生 llama-server
func (m *LlamaManager) Start(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		return nil
	}

	// 端口如果已经有活跃服务，直接复用
	if m.IsPortActive(ctx) {
		return nil
	}

	// 检查 binary 和 model 文件是否存在
	binPath, err := filepath.Abs(m.serverBin)
	if err != nil {
		return fmt.Errorf("resolve llama-server path: %w", err)
	}
	if _, err := os.Stat(binPath); os.IsNotExist(err) {
		return fmt.Errorf("llama-server binary not found at %s", binPath)
	}

	modelPath, err := filepath.Abs(m.modelPath)
	if err != nil {
		return fmt.Errorf("resolve model path: %w", err)
	}
	if _, err := os.Stat(modelPath); os.IsNotExist(err) {
		return fmt.Errorf("model file not found at %s", modelPath)
	}

	cmd := exec.Command(binPath,
		"-m", modelPath,
		"--port", fmt.Sprintf("%d", m.port),
		"-ngl", "99",
		"-c", "2048",
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start llama-server: %w", err)
	}
	m.cmd = cmd

	// 等待端口就绪，最多等待 10 秒
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if m.IsPortActive(ctx) {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}

	return nil
}

// Stop 停止后台伴生服务
func (m *LlamaManager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.cmd != nil && m.cmd.Process != nil {
		_ = m.cmd.Process.Kill()
		_ = m.cmd.Wait()
		m.cmd = nil
	}
}
