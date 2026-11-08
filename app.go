package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"avatar/internal/asr"
	"avatar/internal/config"
	"avatar/internal/engine"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/persona"
	"avatar/internal/sidecar"
	"avatar/internal/skin"
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
	skinMgr    *skin.Manager
	asrEngine  asr.Engine
	mu         sync.Mutex
	isExpanded bool
	lastVAD    time.Time
}
func (a *App) emitEvent(eventName string, data ...interface{}) {
	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, eventName, data...)
	}
}


// NewApp creates a new App application struct
func NewApp() *App {
	cfg := config.LoadConfig()
	// 自动探测本地端云双模型端点 (LM Studio 跑两个模型，或 LM Studio + Ollama)
	ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 1*time.Second)
	edgeURL, edgeM, cloudURL, cloudM, dualOk := llm.AutoDetectDualEndpoints(ctxProbe)
	if dualOk {
		fmt.Printf("自动检测到端云双模型运行中:\n  - 端侧 SLM: %s (%s)\n  - 云端 LLM: %s (%s)\n", edgeURL, edgeM, cloudURL, cloudM)
		cfg.LLM.Edge.BaseURL = edgeURL
		cfg.LLM.Edge.Model = edgeM
		cfg.LLM.Cloud.BaseURL = cloudURL
		cfg.LLM.Cloud.Model = cloudM
		cfg.LLM.BaseURL = cloudURL
		cfg.LLM.Model = cloudM
		_ = config.SaveConfig(cfg)
	} else if !llm.ProbeEndpoint(ctxProbe, cfg.LLM.BaseURL, cfg.LLM.APIKey) {
		if detectedURL, detectedModel, ok := llm.AutoDetectEndpoint(ctxProbe); ok {
			fmt.Printf("自动检测到本地大模型运行于 %s (模型: %s)，已自动接入\n", detectedURL, detectedModel)
			cfg.LLM.BaseURL = detectedURL
			if detectedModel != "" {
				cfg.LLM.Model = detectedModel
			}
			cfg.LLM.Edge.BaseURL = detectedURL
			cfg.LLM.Edge.Model = detectedModel
			cfg.LLM.Cloud.BaseURL = detectedURL
			cfg.LLM.Cloud.Model = detectedModel
			_ = config.SaveConfig(cfg)
		}
	}
	cancelProbe()

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
	skinMgr := skin.NewManager("")

	// 初始化 ASR 语音识别引擎
	var asrEng asr.Engine
	if cfg.ASR.Engine == "native_mac" {
		asrEng = asr.NewNativeMacEngine(cfg.ASR.Locale, string(cfg.Mode))
	} else {
		asrEng = asr.NewMockEngine()
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
		skinMgr:    skinMgr,
		asrEngine:  asrEng,
	}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// 注册 Pipeline 事件广播
	a.pipeline.SetCallbacks(func(s engine.State) {
		fmt.Printf("[状态流转] -> %s\n", s)
		if a.asrEngine != nil {
			_ = a.asrEngine.SetSpeaking(s == engine.StateSpeaking)
		}
		a.emitEvent("avatar:state", string(s))
	}, func(speech string) {
		fmt.Printf("[伴侣播报] %s\n", speech)
		a.emitEvent("avatar:speech", speech)
	}, func(slot persona.PersonaSlot) {
		a.emitEvent("avatar:persona", slot)
	}, func(bond persona.BondData) {
		a.emitEvent("avatar:bond", bond)
	})

	// 注册端云双模型意图路由遥测
	a.pipeline.SetModelRouteCallback(func(route string, model string, isEdge bool) {
		a.emitEvent("avatar:model_route", map[string]interface{}{
			"route":   route,
			"model":   model,
			"is_edge": isEdge,
		})
	})

	// 注册 ASR 语音识别回调并异步启动
	if a.asrEngine != nil {
		a.asrEngine.SetCallbacks(func(partial string) {
			fmt.Printf("[正在倾听] %s\n", partial)
			if a.pipeline.State() == engine.StateSpeaking {
				a.pipeline.Interrupt()
			}
			a.emitEvent("avatar:speech_partial", partial)
			a.emitEvent("avatar:state", "listening")
		}, func(final string) {
			fmt.Printf("[听到输入] %s\n", final)
			a.emitEvent("avatar:speech_final", final)
			a.emitEvent("avatar:state", "thinking")
			go func() {
				err := a.pipeline.Chat(context.Background(), final)
				if err != nil {
					fmt.Printf("[对聊失败] %v\n", err)
					a.emitEvent("avatar:error", fmt.Sprintf("Chat error: %v", err))
				}
			}()
		}, func(speaking bool, energy float64) {
			now := time.Now()
			a.mu.Lock()
			shouldEmit := now.Sub(a.lastVAD) >= 120*time.Millisecond
			if shouldEmit {
				a.lastVAD = now
			}
			a.mu.Unlock()

			if shouldEmit {
				a.emitEvent("avatar:mic_energy", energy)
			}
			if speaking {
				if a.pipeline.State() == engine.StateSpeaking {
					a.pipeline.Interrupt()
				}
				if a.pipeline.State() == engine.StateIdle {
					a.emitEvent("avatar:state", "listening")
				}
			}
		}, func(errMsg string) {
			fmt.Printf("[ASR 异常] %s\n", errMsg)
			a.emitEvent("avatar:error", errMsg)
		})
		a.asrEngine.SetStateCallback(func(s string) {
			if a.pipeline.State() == engine.StateListening || a.pipeline.State() == engine.StateIdle {
				a.emitEvent("avatar:state", s)
			}
		})

		go func() {
			if err := a.asrEngine.Start(ctx); err != nil {
				fmt.Printf("ASR engine start error: %v\n", err)
				a.emitEvent("avatar:error", fmt.Sprintf("语音引擎启动失败: %v", err))
			}
		}()
	}

	// 异步尝试启动本地伴生服务（如果存在模型）
	go func() {
		_ = a.sidecar.Start(context.Background())
	}()

	// 初始广播当前人格与羁绊数据
	if a.personaMgr != nil {
		a.emitEvent("avatar:persona", a.personaMgr.GetActiveSlot())
	}
	if a.bondMgr != nil {
		a.emitEvent("avatar:bond", a.bondMgr.GetData())
	}

	// 初始广播形态与桌宠外观配置
	a.emitEvent("avatar:display_form", a.cfg.DisplayForm)
	a.emitEvent("avatar:pet_skin", a.cfg.PetSkin)
	if a.cfg.DisplayForm == "capsule" {
		runtime.WindowSetSize(a.ctx, 360, 80)
	} else {
		runtime.WindowSetSize(a.ctx, 320, 360)
	}

	// 异步检测 LLM 服务连通性并提示
	go func() {
		time.Sleep(600 * time.Millisecond)
		ctxTest, cancelTest := context.WithTimeout(context.Background(), 1*time.Second)
		defer cancelTest()
		if !llm.ProbeEndpoint(ctxTest, a.cfg.LLM.BaseURL, a.cfg.LLM.APIKey) {
			a.emitEvent("avatar:error", fmt.Sprintf("大模型未连接 (%s)。请启动 LM Studio 或 Ollama。", a.cfg.LLM.BaseURL))
		}
	}()

	// 初始伴侣醒来开场白（根据当前激活人格动态输出）
	go func() {
		time.Sleep(1200 * time.Millisecond)
		greeting := "你终于醒啦~ 今天想聊点什么呀？"
		if a.personaMgr != nil {
			active := a.personaMgr.GetActiveSlot()
			if active.Greeting != "" {
				greeting = active.Greeting
			}
		}
		a.emitEvent("avatar:speech", greeting)
	}()
}

// shutdown is called when the app terminates
func (a *App) shutdown(ctx context.Context) {
	if a.sidecar != nil {
		a.sidecar.Stop()
	}
	if a.asrEngine != nil {
		_ = a.asrEngine.Stop()
	}
	if a.pipeline != nil {
		a.pipeline.Interrupt()
	}
}

// SendTextMessage 处理来自前端的文本提问
func (a *App) SendTextMessage(text string) error {
	if text == "" {
		return nil
	}
	go func() {
		err := a.pipeline.Chat(context.Background(), text)
		if err != nil {
			a.emitEvent("avatar:error", fmt.Sprintf("Chat error: %v", err))
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

// GetMode 获取当前交互模式 ("hands_free" 或 "ptt")
func (a *App) GetMode() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return string(a.cfg.Mode)
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
	if a.asrEngine != nil {
		_ = a.asrEngine.SetMode(string(a.cfg.Mode))
	}
	a.emitEvent("avatar:mode", string(a.cfg.Mode))
	return string(a.cfg.Mode)
}

// StartPTT 开始按键语音输入
func (a *App) StartPTT() error {
	if a.asrEngine == nil {
		return fmt.Errorf("asr engine not available")
	}
	return a.asrEngine.PTTStart()
}

// StopPTT 结束按键语音输入并触发识别
func (a *App) StopPTT() error {
	if a.asrEngine == nil {
		return fmt.Errorf("asr engine not available")
	}
	return a.asrEngine.PTTStop()
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
// SetWindowExpanded 动态调整窗口大小（紧凑胶囊/桌宠 vs 展开管理抽屉）
func (a *App) SetWindowExpanded(expanded bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.isExpanded = expanded
	if a.ctx == nil {
		return
	}
	if expanded {
		runtime.WindowSetSize(a.ctx, 360, 520)
	} else {
		if a.cfg.DisplayForm == "capsule" {
			runtime.WindowSetSize(a.ctx, 360, 80)
		} else {
			runtime.WindowSetSize(a.ctx, 320, 360)
		}
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

	edgeOk := llm.ProbeEndpoint(ctx, a.cfg.LLM.Edge.BaseURL, a.cfg.LLM.Edge.APIKey)
	cloudOk := llm.ProbeEndpoint(ctx, a.cfg.LLM.Cloud.BaseURL, a.cfg.LLM.Cloud.APIKey)
	ollamaOk := llm.ProbeEndpoint(ctx, "http://127.0.0.1:11434/v1", "")

	return map[string]bool{
		"local_sidecar": edgeOk,
		"edge_slm":      edgeOk,
		"cloud_llm":     cloudOk,
		"ollama":        ollamaOk,
	}
}

// GetDualModelInfo 获取当前生效的端侧小模型与云端大模型配置信息
func (a *App) GetDualModelInfo() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return map[string]string{
		"edge_url":    a.cfg.LLM.Edge.BaseURL,
		"edge_model":  a.cfg.LLM.Edge.Model,
		"cloud_url":   a.cfg.LLM.Cloud.BaseURL,
		"cloud_model": a.cfg.LLM.Cloud.Model,
	}
}

// ==================== 桌面宠物 API ====================

// GetDisplayForm 获取当前形态 ("pet" 桌面宠物 或 "capsule" 悬浮胶囊)
func (a *App) GetDisplayForm() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.DisplayForm == "" {
		return "pet"
	}
	return a.cfg.DisplayForm
}

// SetDisplayForm 切换桌面宠物 / 极简胶囊形态并调整窗口
func (a *App) SetDisplayForm(form string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if form != "capsule" {
		form = "pet"
	}
	prevForm := a.cfg.DisplayForm
	a.cfg.DisplayForm = form
	_ = config.SaveConfig(a.cfg)

	if a.ctx != nil {
		if !a.isExpanded {
			if form == "capsule" {
				runtime.WindowSetSize(a.ctx, 360, 80)
			} else {
				runtime.WindowSetSize(a.ctx, 320, 360)
			}
		}
		if prevForm != form {
			runtime.EventsEmit(a.ctx, "avatar:display_form", form)
		}
	}
	return form
}

// GetPetSkin 获取当前桌宠皮肤 ID
func (a *App) GetPetSkin() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cfg.PetSkin == "" {
		return "cat"
	}
	return a.cfg.PetSkin
}

// SetPetSkin 切换桌宠皮肤
func (a *App) SetPetSkin(skinID string) string {
	a.mu.Lock()
	defer a.mu.Unlock()

	if skinID == "" {
		skinID = "whale"
	}
	if a.cfg.PetSkin == skinID {
		return skinID
	}
	a.cfg.PetSkin = skinID
	_ = config.SaveConfig(a.cfg)

	if a.ctx != nil {
		runtime.EventsEmit(a.ctx, "avatar:pet_skin", skinID)
	}
	return skinID
}

// GetSkinsList 获取内置和已导入的皮肤列表
func (a *App) GetSkinsList() []skin.SkinInfo {
	if a.skinMgr == nil {
		return skin.BuiltinSkins
	}
	return a.skinMgr.ListSkins()
}

// ImportSkinPackage 导入 Codex pet 皮肤包（目录、pet.json 或 zip）
func (a *App) ImportSkinPackage(sourcePath string) (skin.SkinInfo, error) {
	if a.skinMgr == nil {
		return skin.SkinInfo{}, fmt.Errorf("skin manager not initialized")
	}
	info, err := a.skinMgr.ImportSkin(sourcePath)
	if err != nil {
		return skin.SkinInfo{}, err
	}
	// 自动切换为新导入的皮肤
	a.SetPetSkin(info.ID)
	return info, nil
}

// InteractPet 桌面宠物交互（抚摸/投喂/戳戳/唤醒）
func (a *App) InteractPet(action string) map[string]interface{} {
	a.mu.Lock()
	defer a.mu.Unlock()

	var bondData persona.BondData
	var leveledUp bool
	if a.bondMgr != nil {
		_, leveledUp = a.bondMgr.AddInteraction()
		bondData = a.bondMgr.GetData()
		if a.ctx != nil {
			runtime.EventsEmit(a.ctx, "avatar:bond", bondData)
		}
	}

	msg := "高兴地跳了起来！"
	cue := "jumping"
	switch action {
	case "pet":
		if a.cfg.PetSkin == "whale" {
			msg = "舒服地眯起眼睛晃动鱼鳍~ (亲密度 +5!)"
		} else if a.cfg.PetSkin == "cat" {
			msg = "舒服地眯起眼睛发出呼噜声~ (亲密度 +5!)"
		} else if a.cfg.PetSkin == "fox" {
			msg = "欢快地摇着毛茸茸的大尾巴~ (亲密度 +5!)"
		} else {
			msg = "舒服地眯起眼睛贴了贴你~ (亲密度 +5!)"
		}
		cue = "jumping"
	case "feed":
		if a.cfg.PetSkin == "whale" {
			msg = "嗷呜一口吃掉白饭配鱼片，满脸满足！"
		} else if a.cfg.PetSkin == "cat" {
			msg = "嗷呜一口吃掉小鱼干，吧唧着嘴巴超开心！"
		} else {
			msg = "嗷呜一口吃掉小零食，满脸满足！"
		}
		cue = "jumping"
	case "rice":
		if a.cfg.PetSkin == "whale" {
			msg = "捧着热气腾腾的大米饭大口干饭！真香~ (亲密度 +5!)"
		} else if a.cfg.PetSkin == "cat" {
			msg = "把小脑袋埋进大白饭碗里大口吧唧吧唧！(亲密度 +5!)"
		} else {
			msg = "开心地享用大米饭，满脸满足！(亲密度 +5!)"
		}
		cue = "rice"
	case "turbo":
		if a.cfg.PetSkin == "whale" {
			msg = "抱紧钢盆疯狂干饭，两只小脚丫欢快扑腾乱蹬！(亲密度 +5!)"
		} else if a.cfg.PetSkin == "cat" {
			msg = "抱住大碗狂吃一空，心满意足地舔着胡须！(亲密度 +5!)"
		} else {
			msg = "风卷残云般狂吃大餐，元气满满！(亲密度 +5!)"
		}
		cue = "turbo"
	case "grass":
		msg = "一个飞扑按住小草玩具，尾巴得意地摇了摇~ (亲密度 +5!)"
		cue = "grass"
	case "poke":
		msg = "歪过头眨巴着眼睛看着你：怎么啦？"
		cue = "jumping"
	case "wake":
		msg = "揉了揉眼睛，元气满满地醒来啦！"
		if a.personaMgr != nil {
			active := a.personaMgr.GetActiveSlot()
			if active.Greeting != "" {
				msg = active.Greeting
			}
		}
		cue = "wake"
	}

	if leveledUp {
		msg = fmt.Sprintf("🎉 羁绊升级到 %s！", bondData.LevelName)
	}

	return map[string]interface{}{
		"action":    action,
		"message":   msg,
		"cue":       cue,
		"xp":        bondData.XP,
		"level":     bondData.Level,
		"levelName": bondData.LevelName,
		"leveledUp": leveledUp,
	}
}

// ToggleAlwaysOnTop 切换窗口置顶
func (a *App) ToggleAlwaysOnTop() bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.cfg.AlwaysOnTop = !a.cfg.AlwaysOnTop
	_ = config.SaveConfig(a.cfg)
	if a.ctx != nil {
		runtime.WindowSetAlwaysOnTop(a.ctx, a.cfg.AlwaysOnTop)
	}
	return a.cfg.AlwaysOnTop
}

// QuitApp 退出应用
func (a *App) QuitApp() {
	if a.ctx != nil {
		runtime.Quit(a.ctx)
	}
}
