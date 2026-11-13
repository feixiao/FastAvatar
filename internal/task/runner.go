package task

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"avatar/internal/cloud"
	"avatar/internal/config"
	"avatar/internal/dispatcher"
	"avatar/internal/interaction"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/orchestrator"
	"avatar/internal/persona"
	"avatar/internal/splitter"
	"avatar/internal/tts"
)

// Runner executes background cognitive and generative tasks.
type Runner struct {
	llmCfg        config.LLMConfig
	llmClient     *llm.Client
	edgeClient    *llm.Client
	edgeCfg       config.ModelEndpointConfig
	cloudGateway  *cloud.Client
	ttsEngine     tts.Synthesizer
	vault         *memory.Vault
	personaMgr    *persona.Manager
	bondMgr       *persona.BondManager
	coord         *orchestrator.Coordinator
	splitter      *splitter.SentenceSplitter
	history       []llm.Message
	mu            sync.RWMutex
	onSpeechChunk func(string)
	onBondSync    func(persona.BondData)
	onModelRoute  func(route string, model string, isEdge bool)
	onPetAction   func(interaction.PetActionPayload)
}

// NewRunner creates a new cognitive task runner.
func NewRunner(
	llmCfg config.LLMConfig,
	client *llm.Client,
	synth tts.Synthesizer,
	vault *memory.Vault,
	personaMgr *persona.Manager,
	bondMgr *persona.BondManager,
	coord *orchestrator.Coordinator,
) *Runner {
	if synth == nil {
		synth = tts.NewMockSynthesizer()
	}
	edgeCfg := llmCfg.GetEdgeEndpoint()
	var edgeCli *llm.Client
	if edgeCfg.BaseURL != "" {
		edgeCli = llm.NewClient(edgeCfg.BaseURL, edgeCfg.APIKey)
	}
	return &Runner{
		llmCfg:     llmCfg,
		llmClient:  client,
		edgeClient: edgeCli,
		edgeCfg:    edgeCfg,
		ttsEngine:  synth,
		vault:      vault,
		personaMgr: personaMgr,
		bondMgr:    bondMgr,
		coord:      coord,
		splitter:   splitter.NewSentenceSplitter(),
		history:    make([]llm.Message, 0),
	}
}

// SetSpeechChunkCallback registers callback for incremental speech chunks.
func (r *Runner) SetSpeechChunkCallback(cb func(string)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onSpeechChunk = cb
}

// SetBondSyncCallback registers callback for updated bond levels.
func (r *Runner) SetBondSyncCallback(cb func(persona.BondData)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onBondSync = cb
}

// SetModelRouteCallback registers callback for which model (edge vs cloud) answered this turn.
func (r *Runner) SetModelRouteCallback(cb func(route string, model string, isEdge bool)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onModelRoute = cb
}

// SetPetActionCallback registers callback for pet actions extracted from LLM responses.
func (r *Runner) SetPetActionCallback(cb func(interaction.PetActionPayload)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.onPetAction = cb
}

func resolveActionTag(tag string) interaction.PetActionPayload {
	norm := strings.ToLower(strings.TrimSpace(tag))
	switch norm {
	case "jump", "jumping":
		return interaction.PetActionPayload{
			Action:  "jump",
			Cue:     "jumping",
			Message: "欢快地纵身一跃，在空中翻了个跟头！",
			XP:      3,
		}
	case "feed", "eat", "running":
		return interaction.PetActionPayload{
			Action:  "feed",
			Cue:     "running",
			Message: "开心地大口吃着美味食物~",
			XP:      3,
		}
	case "wave", "waving":
		return interaction.PetActionPayload{
			Action:  "wave",
			Cue:     "waving",
			Message: "开心地挥了挥小爪子打招呼~",
			XP:      3,
		}
	case "failed", "dizzy":
		return interaction.PetActionPayload{
			Action:  "failed",
			Cue:     "failed",
			Message: "困惑地歪了歪身子装死~",
			XP:      2,
		}
	case "review", "think", "thinking":
		return interaction.PetActionPayload{
			Action:  "review",
			Cue:     "review",
			Message: "若有所思地歪头审视观察~",
			XP:      2,
		}
	case "roam_right":
		return interaction.PetActionPayload{
			Action:  "roam_right",
			Cue:     "running-right",
			Move:    "right",
			Message: "迈着轻快猫步向右边巡视小跑~",
			XP:      2,
		}
	case "roam_left":
		return interaction.PetActionPayload{
			Action:  "roam_left",
			Cue:     "running-left",
			Move:    "left",
			Message: "迈着轻快猫步向左边巡视小跑~",
			XP:      2,
		}
	case "roam":
		return interaction.PetActionPayload{
			Action:  "roam",
			Cue:     "running",
			Message: "轻快地在桌面上漫游巡视~",
			XP:      2,
		}
	case "sleep", "idle":
		return interaction.PetActionPayload{
			Action:  "sleep",
			Cue:     "idle",
			Message: "收起小爪子趴下打盹~",
			XP:      2,
		}
	case "sit", "squat":
		return interaction.PetActionPayload{
			Action:  "sit",
			Cue:     "idle",
			Message: "乖巧地蹲坐下来~",
			XP:      3,
		}
	case "wake":
		return interaction.PetActionPayload{
			Action:  "wake",
			Cue:     "jumping",
			Message: "精神抖擞地站起身来！",
			XP:      3,
		}
	case "meow":
		return interaction.PetActionPayload{
			Action:  "meow",
			Cue:     "waving",
			Message: "软软地叫了一声：喵呜~",
			XP:      3,
		}
	default:
		return interaction.PetActionPayload{
			Action:  norm,
			Cue:     norm,
			Message: "高兴地做出了互动动作！",
			XP:      2,
		}
	}
}


func inferActionFromText(query, reply string) string {
	combined := query + " " + reply
	if strings.Contains(combined, "蹲下") || strings.Contains(combined, "坐下") || strings.Contains(combined, "趴下") || strings.Contains(combined, "躺下") {
		return "sit"
	}
	if strings.Contains(combined, "跳一下") || strings.Contains(combined, "跳一个") || strings.Contains(combined, "翻跟头") || strings.Contains(combined, "跳舞") {
		return "jump"
	}
	if strings.Contains(combined, "小鱼干") || strings.Contains(combined, "干饭") || strings.Contains(combined, "吃鱼") {
		return "feed"
	}
	if strings.Contains(combined, "挥手") || strings.Contains(combined, "打招呼") || strings.Contains(combined, "挥挥爪") || strings.Contains(combined, "握手") {
		return "wave"
	}
	if strings.Contains(combined, "巡视") || strings.Contains(combined, "巡逻") || strings.Contains(combined, "向右") {
		return "roam_right"
	}
	if strings.Contains(combined, "向左") {
		return "roam_left"
	}
	if strings.Contains(combined, "装死") || strings.Contains(combined, "晕倒") {
		return "failed"
	}
	if strings.Contains(combined, "思考") || strings.Contains(combined, "歪头") {
		return "review"
	}
	if strings.Contains(combined, "睡觉") || strings.Contains(combined, "打盹") || strings.Contains(combined, "休息吧") {
		return "sleep"
	}
	return ""
}
// SetCloudGateway sets the cloud gateway client to route complex LLM queries through backend server (:8088).
func (r *Runner) SetCloudGateway(client *cloud.Client) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cloudGateway = client
}
// SetEdgeClient explicitly updates edge small model client and endpoint config.
func (r *Runner) SetEdgeClient(edgeCli *llm.Client, edgeCfg config.ModelEndpointConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edgeClient = edgeCli
	r.edgeCfg = edgeCfg
}

// History returns a copy of the conversation history.
func (r *Runner) History() []llm.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	res := make([]llm.Message, len(r.history))
	copy(res, r.history)
	return res
}

// ClearHistory resets the conversation history.
func (r *Runner) ClearHistory() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = make([]llm.Message, 0)
}

// Run implements orchestrator.TaskRunner.
func (r *Runner) Run(ctx context.Context, task orchestrator.Task, emit func(orchestrator.TaskEvent)) error {
	now := time.Now()
	emit(orchestrator.TaskEvent{
		TaskID:    task.TaskID,
		TurnID:    task.TurnID,
		SessionID: task.SessionID,
		State:     orchestrator.TaskStateUnderstanding,
		Timestamp: now,
	})

	// 1. 组装动态 System Prompt (< 1ms)
	var activeSlot persona.PersonaSlot
	if r.personaMgr != nil {
		activeSlot = r.personaMgr.GetActiveSlot()
	}
	systemPrompt := persona.AssembleSystemPrompt(activeSlot, r.bondMgr, r.vault, task.GoalSnapshot)

	r.mu.Lock()
	r.history = append(r.history, llm.Message{Role: "user", Content: task.GoalSnapshot})
	if len(r.history) > 20 {
		r.history = r.history[len(r.history)-20:]
	}

	msgs := make([]llm.Message, 0, len(r.history)+1)
	msgs = append(msgs, llm.Message{Role: "system", Content: systemPrompt})
	msgs = append(msgs, r.history...)
	r.mu.Unlock()

	if r.coord != nil {
		r.coord.SetState(orchestrator.StateThinking)
	}

	emit(orchestrator.TaskEvent{
		TaskID:    task.TaskID,
		TurnID:    task.TurnID,
		SessionID: task.SessionID,
		State:     orchestrator.TaskStateExecuting,
		Timestamp: time.Now(),
	})

	sentenceChan := make(chan string, 16)
	errChan := make(chan error, 2)

	// Goroutine 1: TTS 发音消费队列
	var ttsWg sync.WaitGroup
	ttsWg.Add(1)
	go func() {
		defer ttsWg.Done()
		firstChunk := true
		for {
			select {
			case <-ctx.Done():
				r.ttsEngine.Stop()
				return
			case sentence, ok := <-sentenceChan:
				if !ok {
					return
				}
				cleanSentence := splitter.CleanSpokenText(sentence)
				if strings.TrimSpace(cleanSentence) == "" {
					continue
				}
				if firstChunk {
					if r.coord != nil {
						r.coord.SetState(orchestrator.StateSpeaking)
					}
					firstChunk = false
				}
				r.mu.RLock()
				cb := r.onSpeechChunk
				r.mu.RUnlock()
				if cb != nil {
					cb(cleanSentence)
				}
				if err := r.ttsEngine.Speak(ctx, cleanSentence); err != nil && ctx.Err() == nil {
					fmt.Printf("[TTS 播报异常] %v\n", err)
				}
			}
		}
	}()

	// Goroutine 2: LLM 流式消费与断句
	var fullResponse string
	r.splitter.Reset()

	// 意图复杂度仲裁：决定使用端侧小模型还是云端大模型
	isComplex, complexReason := dispatcher.CheckComplexity(task.GoalSnapshot)
	var actionTriggered bool
	actionRegex := regexp.MustCompile(`\[ACTION:([a-zA-Z0-9_-]+)\]`)
	handleActionStream := func(text string) {
		if !actionTriggered {
			if m := actionRegex.FindStringSubmatch(text); len(m) > 1 {
				actionTriggered = true
				actPayload := resolveActionTag(m[1])
				r.mu.RLock()
				cb := r.onPetAction
				r.mu.RUnlock()
				if cb != nil {
					cb(actPayload)
				}
			} else {
				// 兜底小模型 (如 0.5B / 1.5B) 遗漏标签：从用户指令与回复内容自动推断动作
				inferred := inferActionFromText(task.GoalSnapshot, text)
				if inferred != "" {
					actionTriggered = true
					actPayload := resolveActionTag(inferred)
					r.mu.RLock()
					cb := r.onPetAction
					r.mu.RUnlock()
					if cb != nil {
						cb(actPayload)
					}
				}
			}
		}
	}
	var activeClient *llm.Client
	var activeModel string
	var activeTemp float64
	var activeMaxTokens int
	var routeLabel string

	r.mu.RLock()
	edgeCli := r.edgeClient
	cloudGw := r.cloudGateway
	edgeCfg := r.edgeCfg
	defaultCli := r.llmClient
	defaultCfg := r.llmCfg
	routeCb := r.onModelRoute
	r.mu.RUnlock()

	if isComplex && cloudGw != nil {
		routeLabel = "CLOUD_GATEWAY"
		activeModel = "cloud"
	} else {
		routeLabel = "EDGE_SLM"
		if edgeCli != nil && edgeCfg.BaseURL != "" {
			activeClient = edgeCli
			activeModel = edgeCfg.Model
			activeTemp = edgeCfg.Temperature
			activeMaxTokens = edgeCfg.MaxTokens
		} else if defaultCli != nil {
			activeClient = defaultCli
			activeModel = defaultCfg.Model
			activeTemp = defaultCfg.Temperature
			activeMaxTokens = defaultCfg.MaxTokens
		}

		// 动态检测本地端点实际加载的模型 (支持 LM Studio 热切换，无需重启应用)
		if activeClient != nil && activeClient.BaseURL != "" {
			probeCtx, probeCancel := context.WithTimeout(ctx, 150*time.Millisecond)
			if loaded, err := llm.FetchLoadedModels(probeCtx, activeClient.BaseURL, activeClient.APIKey); err == nil && len(loaded) > 0 {
				isLoaded := false
				for _, lm := range loaded {
					if lm == activeModel {
						isLoaded = true
						break
					}
				}
				if !isLoaded {
					bestEdge := llm.SelectBestEdgeModel(loaded)
					if bestEdge != "" {
						fmt.Printf("[模型热更] 动态检测到端侧模型切换: %s -> %s (无需重启生效)\n", activeModel, bestEdge)
						activeModel = bestEdge
						r.mu.Lock()
						r.edgeCfg.Model = bestEdge
						r.mu.Unlock()
					}
				}
			}
			probeCancel()
		}

		// 端侧小模型使用专门定制的轻量沉浸式 System Prompt (消除代词混淆，防止元语言泄露)
		if len(msgs) > 0 && msgs[0].Role == "system" {
			msgs[0].Content = persona.AssembleEdgeSystemPrompt(activeSlot, r.bondMgr, r.vault)
		}
	}

	if activeClient == nil {
		activeClient = defaultCli
		activeModel = defaultCfg.Model
		activeTemp = defaultCfg.Temperature
		activeMaxTokens = defaultCfg.MaxTokens
	}
	if activeModel == "" {
		activeModel = r.llmCfg.Model
	}
	if activeTemp == 0 {
		activeTemp = 0.7
	}
	if activeMaxTokens == 0 {
		activeMaxTokens = 1024
	}

	var activePersonaName string
	if r.personaMgr != nil {
		slot := r.personaMgr.GetActiveSlot()
		activePersonaName = slot.ID
		if vc, ok := r.ttsEngine.(tts.VoiceConfigurable); ok {
			if slot.VoiceOverride != "" {
				vc.SetVoice(slot.VoiceOverride)
			}
			if slot.RateOverride > 0 {
				vc.SetRate(slot.RateOverride)
			}
		}
	}

	var err error
	if routeLabel == "CLOUD_GATEWAY" && cloudGw != nil {
		activeModel = "cloud"
		fmt.Printf("[模型调度] 决策: 【走云端】通道: CLOUD_GATEWAY (:8088) | 原因: %s | 角色: %s | 用户提问: %q\n", complexReason, activePersonaName, task.GoalSnapshot)
		if routeCb != nil {
			routeCb(routeLabel, activeModel, false)
		}

		err = cloudGw.ChatStream(ctx, string(task.TurnID), task.GoalSnapshot, activePersonaName, func(token string) error {
			fullResponse += token
			handleActionStream(fullResponse)
			chunks := r.splitter.Push(token)
			for _, chunk := range chunks {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case sentenceChan <- chunk:
				}
			}
			return nil
		})
	} else {
		if isComplex {
			fmt.Printf("[模型调度] 决策: 【本地降级】通道: %s (云端网关未就绪，降级端侧) | 模型: %s | 原因: %s | 用户提问: %q\n", routeLabel, activeModel, complexReason, task.GoalSnapshot)
		} else {
			fmt.Printf("[模型调度] 决策: 【本地端侧】通道: %s | 模型: %s | 原因: %s | 状态: 100%% 本地推理，未走云端 | 用户提问: %q\n", routeLabel, activeModel, complexReason, task.GoalSnapshot)
		}
		if routeCb != nil {
			routeCb(routeLabel, activeModel, !isComplex)
		}
		req := llm.ChatCompletionRequest{
			Model:       activeModel,
			Messages:    msgs,
			Temperature: activeTemp,
			MaxTokens:   activeMaxTokens,
		}

		err = activeClient.StreamChat(ctx, req, func(token string) error {
			fullResponse += token
			handleActionStream(fullResponse)
			chunks := r.splitter.Push(token)
			for _, chunk := range chunks {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case sentenceChan <- chunk:
				}
			}
			return nil
		})

		// 若端侧小模型推理失败 (如模型加载中、显存溢出、400 异常等)，且云端网关在线可用，自动无缝降级切换至云端网关
		if err != nil && cloudGw != nil && cloudGw.IsOnline() && ctx.Err() == nil && fullResponse == "" {
			fmt.Printf("[模型调度] 端侧小模型推理失败 (%v)，无缝自动降级切换至【云端网关】(:8088)...\n", err)
			routeLabel = "CLOUD_GATEWAY"
			activeModel = "cloud"
			if routeCb != nil {
				routeCb(routeLabel, activeModel, false)
			}
			err = cloudGw.ChatStream(ctx, string(task.TurnID), task.GoalSnapshot, activePersonaName, func(token string) error {
				fullResponse += token
				handleActionStream(fullResponse)
				chunks := r.splitter.Push(token)
				for _, chunk := range chunks {
					select {
					case <-ctx.Done():
						return ctx.Err()
					case sentenceChan <- chunk:
					}
				}
				return nil
			})
		}
	}

	if err != nil && ctx.Err() == nil {
		errChan <- fmt.Errorf("llm stream: %w", err)
	}

	// 消费残余标点
	remainders := r.splitter.Flush()
	for _, rem := range remainders {
		select {
		case <-ctx.Done():
		case sentenceChan <- rem:
		}
	}
	close(sentenceChan)

	ttsWg.Wait()

	if ctx.Err() != nil {
		r.ttsEngine.Stop()
		r.splitter.Reset()
		if r.coord != nil {
			r.coord.SetState(orchestrator.StateIdle)
		}
		emit(orchestrator.TaskEvent{
			TaskID:    task.TaskID,
			TurnID:    task.TurnID,
			SessionID: task.SessionID,
			State:     orchestrator.TaskStateCancelled,
			Result:    "context cancelled",
			Timestamp: time.Now(),
		})
		return ctx.Err()
	}

	select {
	case err := <-errChan:
		r.ttsEngine.Stop()
		r.splitter.Reset()
		if r.coord != nil {
			r.coord.SetState(orchestrator.StateIdle)
		}
		emit(orchestrator.TaskEvent{
			TaskID:    task.TaskID,
			TurnID:    task.TurnID,
			SessionID: task.SessionID,
			State:     orchestrator.TaskStateFailed,
			Error:     err,
			Timestamp: time.Now(),
		})
		return err
	default:
	}

	// 沉淀对话历史与更新羁绊
	if len(fullResponse) > 0 {
		r.mu.Lock()
		r.history = append(r.history, llm.Message{Role: "assistant", Content: fullResponse})
		r.mu.Unlock()

		if r.bondMgr != nil {
			r.bondMgr.AddInteraction()
			r.mu.RLock()
			bondCb := r.onBondSync
			r.mu.RUnlock()
			if bondCb != nil {
				bondCb(r.bondMgr.GetData())
			}
		}

		// 异步沉淀长期记忆
		if r.vault != nil {
			memory.ExtractMemoriesAsync(r.llmClient, r.llmCfg.Model, task.GoalSnapshot, fullResponse, r.vault)
		}
	}

	if r.coord != nil {
		r.coord.SetState(orchestrator.StateIdle)
	}

	emit(orchestrator.TaskEvent{
		TaskID:    task.TaskID,
		TurnID:    task.TurnID,
		SessionID: task.SessionID,
		State:     orchestrator.TaskStateCompleted,
		Result:    fullResponse,
		Timestamp: time.Now(),
	})

	return nil
}
