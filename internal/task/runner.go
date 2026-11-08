package task

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"avatar/internal/config"
	"avatar/internal/dispatcher"
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
	cloudClient   *llm.Client
	cloudCfg      config.ModelEndpointConfig
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
	cloudCfg := llmCfg.GetCloudEndpoint()
	var edgeCli, cloudCli *llm.Client
	if edgeCfg.BaseURL != "" {
		edgeCli = llm.NewClient(edgeCfg.BaseURL, edgeCfg.APIKey)
	}
	if cloudCfg.BaseURL != "" {
		cloudCli = llm.NewClient(cloudCfg.BaseURL, cloudCfg.APIKey)
	}
	return &Runner{
		llmCfg:      llmCfg,
		llmClient:   client,
		edgeClient:  edgeCli,
		edgeCfg:     edgeCfg,
		cloudClient: cloudCli,
		cloudCfg:    cloudCfg,
		ttsEngine:   synth,
		vault:       vault,
		personaMgr:  personaMgr,
		bondMgr:     bondMgr,
		coord:       coord,
		splitter:    splitter.NewSentenceSplitter(),
		history:     make([]llm.Message, 0),
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

// SetDualClients explicitly updates edge and cloud clients and endpoint configs.
func (r *Runner) SetDualClients(
	edgeCli *llm.Client,
	edgeCfg config.ModelEndpointConfig,
	cloudCli *llm.Client,
	cloudCfg config.ModelEndpointConfig,
) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.edgeClient = edgeCli
	r.edgeCfg = edgeCfg
	r.cloudClient = cloudCli
	r.cloudCfg = cloudCfg
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
	isComplex := dispatcher.IsComplexQuery(task.GoalSnapshot)
	var activeClient *llm.Client
	var activeModel string
	var activeTemp float64
	var activeMaxTokens int
	var routeLabel string

	r.mu.RLock()
	edgeCli := r.edgeClient
	cloudCli := r.cloudClient
	edgeCfg := r.edgeCfg
	cloudCfg := r.cloudCfg
	defaultCli := r.llmClient
	defaultCfg := r.llmCfg
	routeCb := r.onModelRoute
	r.mu.RUnlock()

	if isComplex {
		routeLabel = "CLOUD_LLM"
		if cloudCli != nil && cloudCfg.BaseURL != "" {
			activeClient = cloudCli
			activeModel = cloudCfg.Model
			activeTemp = cloudCfg.Temperature
			activeMaxTokens = cloudCfg.MaxTokens
		} else if defaultCli != nil {
			activeClient = defaultCli
			activeModel = defaultCfg.Model
			activeTemp = defaultCfg.Temperature
			activeMaxTokens = defaultCfg.MaxTokens
		}
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

	fmt.Printf("[模型调度] 通道: %s | 模型: %s | 用户输入: %q\n", routeLabel, activeModel, task.GoalSnapshot)
	if routeCb != nil {
		routeCb(routeLabel, activeModel, !isComplex)
	}

	req := llm.ChatCompletionRequest{
		Model:       activeModel,
		Messages:    msgs,
		Temperature: activeTemp,
		MaxTokens:   activeMaxTokens,
	}

	err := activeClient.StreamChat(ctx, req, func(token string) error {
		fullResponse += token
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
