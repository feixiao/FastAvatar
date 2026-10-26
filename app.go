package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"avatar/internal/config"
	"avatar/internal/engine"
	"avatar/internal/llm"
	"avatar/internal/sidecar"
	"avatar/internal/tts"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx      context.Context
	cfg      config.AppConfig
	pipeline *engine.Pipeline
	sidecar  *sidecar.LlamaManager
	mu       sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.LoadConfig()
	sidecarMgr := sidecar.NewLlamaManager(cfg.LlamaServer, cfg.LocalGGUF, cfg.SidecarPort)

	// 初始化 TTS 引擎
	var synth tts.Synthesizer
	if cfg.TTS.Engine == "mac_say" {
		synth = tts.NewMacSaySynthesizer(cfg.TTS.Voice, cfg.TTS.Rate)
	} else {
		synth = tts.NewMockSynthesizer()
	}

	pipe := engine.NewPipeline(cfg, synth)

	return &App{
		cfg:      cfg,
		pipeline: pipe,
		sidecar:  sidecarMgr,
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 注册 Pipeline 事件广播
	a.pipeline.SetCallbacks(func(s engine.State) {
		runtime.EventsEmit(a.ctx, "avatar:state", string(s))
	}, func(speech string) {
		runtime.EventsEmit(a.ctx, "avatar:speech", speech)
	})

	// 异步尝试启动本地伴生服务（如果存在模型）
	go func() {
		_ = a.sidecar.Start(context.Background())
	}()
}

// shutdown is called when the app terminates
func (a *App) shutdown(ctx context.Context) {
	a.sidecar.Stop()
	a.pipeline.Interrupt()
}

// GetConfig 获取当前配置
func (a *App) GetConfig() config.AppConfig {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// SaveConfig 保存配置
func (a *App) SaveConfig(newCfg config.AppConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if err := config.SaveConfig(newCfg); err != nil {
		return err
	}
	a.cfg = newCfg
	return nil
}

// SendTextMessage 处理来自前端的文本提问
func (a *App) SendTextMessage(text string) error {
	if text == "" {
		return nil
	}
	go func() {
		err := a.pipeline.Chat(context.Background(), text)
		if err != nil {
			runtime.EventsEmit(a.ctx, "avatar:error", fmt.Sprintf("Chat error: %v", err))
		}
	}()
	return nil
}

// Interrupt 打断当前说话
func (a *App) Interrupt() {
	a.pipeline.Interrupt()
}

// GetState 获取当前状态
func (a *App) GetState() string {
	return string(a.pipeline.State())
}

// ToggleMode 切换连续畅聊与快捷键触发模式
func (a *App) ToggleMode() string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.cfg.Mode == config.ModeHandsFree {
		a.cfg.Mode = config.ModePTT
	} else {
		a.cfg.Mode = config.ModeHandsFree
	}
	_ = config.SaveConfig(a.cfg)
	runtime.EventsEmit(a.ctx, "avatar:mode", string(a.cfg.Mode))
	return string(a.cfg.Mode)
}

// CheckEngineStatus 检测本地端侧与云端可用状态
func (a *App) CheckEngineStatus() map[string]bool {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	localOk := llm.ProbeEndpoint(ctx, a.cfg.LLM.BaseURL, "")
	ollamaOk := llm.ProbeEndpoint(ctx, "http://127.0.0.1:11434/v1", "")

	return map[string]bool{
		"local_sidecar": localOk,
		"ollama":        ollamaOk,
	}
}
