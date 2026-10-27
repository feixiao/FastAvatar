package task

import (
	"context"
	"fmt"
	"sync"
	"time"

	"avatar/internal/config"
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
	return &Runner{
		llmCfg:     llmCfg,
		llmClient:  client,
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
					cb(sentence)
				}
				_ = r.ttsEngine.Speak(ctx, sentence)
			}
		}
	}()

	// Goroutine 2: LLM 流式消费与断句
	var fullResponse string
	r.splitter.Reset()

	req := llm.ChatCompletionRequest{
		Model:       r.llmCfg.Model,
		Messages:    msgs,
		Temperature: r.llmCfg.Temperature,
		MaxTokens:   r.llmCfg.MaxTokens,
	}

	err := r.llmClient.StreamChat(ctx, req, func(token string) error {
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
