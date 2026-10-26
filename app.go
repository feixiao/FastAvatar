package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"avatar/internal/config"
	"avatar/internal/engine"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/persona"
	"avatar/internal/sidecar"
	"avatar/internal/tts"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App struct
type App struct {
	ctx        context.Context
	cfg        config.AppConfig
	pipeline   *engine.Pipeline
	sidecar    *sidecar.LlamaManager
	vault      *memory.Vault
	personaMgr *persona.Manager
	bondMgr    *persona.BondManager
	mu         sync.Mutex
}

// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.LoadConfig()
	sidecarMgr := sidecar.NewLlamaManager(cfg.LlamaServer, cfg.LocalGGUF, cfg.SidecarPort)

	// 初始化记忆、人格与羁绊管理器
	vault, err := memory.NewVault("")
	if err != nil {
		fmt.Printf("Init memory vault error: %v\n", err)
	}
	personaMgr, err := persona.NewManager("")
	if err != nil {
		fmt.Printf("Init persona manager error: %v\n", err)
	}
	bondMgr, err := persona.NewBondManager("")
	if err != nil {
		fmt.Printf("Init bond manager error: %v\n", err)
	}

	// 初始化 TTS 引擎
	var synth tts.Synthesizer
	if cfg.TTS.Engine == "mac_say" {
		activeVoice := cfg.TTS.Voice
		activeRate := cfg.TTS.Rate
		if personaMgr != nil {
			slot := personaMgr.GetActiveSlot()
			if slot.VoiceOverride != "" {
				activeVoice = slot.VoiceOverride
			}
			if slot.RateOverride > 0 {
				activeRate = slot.RateOverride
			}
		}
		synth = tts.NewMacSaySynthesizer(activeVoice, activeRate)
	} else {
		synth = tts.NewMockSynthesizer()
	}

	pipe := engine.NewPipeline(cfg, synth, vault, personaMgr, bondMgr)

	return &App{
		cfg:        cfg,
		pipeline:   pipe,
		sidecar:    sidecarMgr,
		vault:      vault,
		personaMgr: personaMgr,
		bondMgr:    bondMgr,
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 注册 Pipeline 事件广播
	a.pipeline.SetCallbacks(func(s engine.State) {
		runtime.EventsEmit(a.ctx, "avatar:state", string(s))
	}, func(speech string) {
		runtime.EventsEmit(a.ctx, "avatar:speech", speech)
	}, func(slot persona.PersonaSlot) {
		runtime.EventsEmit(a.ctx, "avatar:persona", slot)
	}, func(bond persona.BondData) {
		runtime.EventsEmit(a.ctx, "avatar:bond", bond)
	})

	// 异步尝试启动本地伴生服务（如果存在模型）
	go func() {
		_ = a.sidecar.Start(context.Background())
	}()

	// 初始广播当前人格与羁绊数据
	if a.personaMgr != nil {
		runtime.EventsEmit(a.ctx, "avatar:persona", a.personaMgr.GetActiveSlot())
	}
	if a.bondMgr != nil {
		runtime.EventsEmit(a.ctx, "avatar:bond", a.bondMgr.GetData())
	}
}

// shutdown is called when the app terminates
func (a *App) shutdown(ctx context.Context) {
	a.sidecar.Stop()
	a.pipeline.Interrupt()
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

// ==================== 记忆管理 API ====================

func (a *App) GetMemoryData() memory.MemoryData {
	if a.vault == nil {
		return memory.MemoryData{}
	}
	return a.vault.GetData()
}

func (a *App) AddOrUpdateProfile(category, key, value string) memory.ProfileItem {
	if a.vault == nil {
		return memory.ProfileItem{}
	}
	item := a.vault.AddOrUpdateProfile(category, key, value)
	runtime.EventsEmit(a.ctx, "avatar:memory_updated", a.vault.GetData())
	return item
}

func (a *App) DeleteProfile(id string) bool {
	if a.vault == nil {
		return false
	}
	ok := a.vault.DeleteProfile(id)
	if ok {
		runtime.EventsEmit(a.ctx, "avatar:memory_updated", a.vault.GetData())
	}
	return ok
}

func (a *App) AddEpisode(content string, tags []string) memory.EpisodeItem {
	if a.vault == nil {
		return memory.EpisodeItem{}
	}
	item := a.vault.AddEpisode(content, tags)
	runtime.EventsEmit(a.ctx, "avatar:memory_updated", a.vault.GetData())
	return item
}

func (a *App) DeleteEpisode(id string) bool {
	if a.vault == nil {
		return false
	}
	ok := a.vault.DeleteEpisode(id)
	if ok {
		runtime.EventsEmit(a.ctx, "avatar:memory_updated", a.vault.GetData())
	}
	return ok
}

// ==================== 人格卡槽 API ====================

func (a *App) GetPersonaData() persona.PersonaData {
	if a.personaMgr == nil {
		return persona.PersonaData{}
	}
	return a.personaMgr.GetData()
}
// SetWindowExpanded 动态调整窗口大小（紧凑胶囊 vs 展开管理抽屉）
func (a *App) SetWindowExpanded(expanded bool) {
	if a.ctx == nil {
		return
	}
	if expanded {
		runtime.WindowSetSize(a.ctx, 360, 440)
	} else {
		runtime.WindowSetSize(a.ctx, 360, 80)
	}
}

func (a *App) SetActivePersona(id string) (persona.PersonaSlot, error) {
	if a.personaMgr == nil {
		return persona.PersonaSlot{}, fmt.Errorf("persona manager not initialized")
	}
	slot, err := a.personaMgr.SetActiveSlot(id)
	if err == nil {
		runtime.EventsEmit(a.ctx, "avatar:persona", slot)
	}
	return slot, err
}

func (a *App) RotatePersona() persona.PersonaSlot {
	if a.personaMgr == nil {
		return persona.PersonaSlot{}
	}
	slot := a.personaMgr.RotateSlot()
	runtime.EventsEmit(a.ctx, "avatar:persona", slot)
	return slot
}

func (a *App) SetAutoSchedule(enable bool) {
	if a.personaMgr != nil {
		a.personaMgr.SetAutoSchedule(enable)
	}
}

// ==================== 关系羁绊 API ====================

func (a *App) GetBondData() persona.BondData {
	if a.bondMgr == nil {
		return persona.BondData{}
	}
	return a.bondMgr.GetData()
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
