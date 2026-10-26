package engine

import (
	"context"
	"fmt"
	"sync"

	"avatar/internal/config"
	"avatar/internal/llm"
	"avatar/internal/memory"
	"avatar/internal/persona"
	"avatar/internal/tts"
)

type State string

const (
	StateIdle      State = "idle"
	StateListening State = "listening"
	StateThinking  State = "thinking"
	StateSpeaking  State = "speaking"
)

type Pipeline struct {
	cfg           config.AppConfig
	llmClient     *llm.Client
	ttsEngine     tts.Synthesizer
	splitter      *SentenceSplitter
	vault         *memory.Vault
	personaMgr    *persona.Manager
	bondMgr       *persona.BondManager
	state         State
	history       []llm.Message
	activeCtx     context.Context
	cancelFunc    context.CancelFunc
	mu            sync.RWMutex
	onStateChange func(State)
	onSpeechChunk func(string)
	onPersonaSync func(persona.PersonaSlot)
	onBondSync    func(persona.BondData)
}

func NewPipeline(
	cfg config.AppConfig,
	synthesizer tts.Synthesizer,
	vault *memory.Vault,
	personaMgr *persona.Manager,
	bondMgr *persona.BondManager,
) *Pipeline {
	if synthesizer == nil {
		synthesizer = tts.NewMacSaySynthesizer(cfg.TTS.Voice, cfg.TTS.Rate)
	}

	return &Pipeline{
		cfg:        cfg,
		llmClient:  llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey),
		ttsEngine:  synthesizer,
		splitter:   NewSentenceSplitter(),
		vault:      vault,
		personaMgr: personaMgr,
		bondMgr:    bondMgr,
		state:      StateIdle,
		history:    make([]llm.Message, 0),
	}
}

func (p *Pipeline) SetCallbacks(
	onState func(State),
	onSpeech func(string),
	onPersona func(persona.PersonaSlot),
	onBond func(persona.BondData),
) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStateChange = onState
	p.onSpeechChunk = onSpeech
	p.onPersonaSync = onPersona
	p.onBondSync = onBond
}

func (p *Pipeline) setState(s State) {
	p.mu.Lock()
	p.state = s
	cb := p.onStateChange
	p.mu.Unlock()

	if cb != nil {
		cb(s)
	}
}

func (p *Pipeline) State() State {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.state
}

// Interrupt 打断当前正在进行的生成与发音
func (p *Pipeline) Interrupt() {
	p.mu.Lock()
	if p.cancelFunc != nil {
		p.cancelFunc()
		p.cancelFunc = nil
	}
	p.ttsEngine.Stop()
	p.splitter.Reset()
	p.mu.Unlock()

	p.setState(StateIdle)
}

// Chat 接收用户问题，经过前置极速规则拦截 -> 动态组装提示词 -> 流式对话 -> 异步沉淀记忆
func (p *Pipeline) Chat(parentCtx context.Context, userInput string) error {
	p.Interrupt()

	ctx, cancel := context.WithCancel(parentCtx)
	p.mu.Lock()
	p.activeCtx = ctx
	p.cancelFunc = cancel
	p.mu.Unlock()

	// 1. 检查时段流转
	if p.personaMgr != nil {
		if changed, newSlot := p.personaMgr.CheckSchedule(); changed {
			if p.onPersonaSync != nil {
				p.onPersonaSync(newSlot)
			}
		}
	}

	// 2. 0.1ms 快速拦截：口语切换人格
	if p.personaMgr != nil {
		if intercepted, confirm, newSlot := p.personaMgr.TryInterceptPersonaSwitch(userInput); intercepted {
			if p.onPersonaSync != nil {
				p.onPersonaSync(newSlot)
			}
			p.setState(StateSpeaking)
			if p.onSpeechChunk != nil {
				p.onSpeechChunk(confirm)
			}
			_ = p.ttsEngine.Speak(ctx, confirm)
			p.setState(StateIdle)
			return nil
		}
	}

	// 3. 0.1ms 快速拦截：口语记忆指令（"记住..." / "忘掉..."）
	if p.vault != nil {
		res := p.vault.TryInterceptMemoryCommand(userInput)
		if res.Intercepted {
			p.setState(StateSpeaking)
			if p.onSpeechChunk != nil {
				p.onSpeechChunk(res.Confirmation)
			}
			_ = p.ttsEngine.Speak(ctx, res.Confirmation)
			p.setState(StateIdle)
			return nil
		}
	}

	// 4. 组装当前会话的动态 System Prompt (< 1ms)
	var activeSlot persona.PersonaSlot
	if p.personaMgr != nil {
		activeSlot = p.personaMgr.GetActiveSlot()
	}
	systemPrompt := persona.AssembleSystemPrompt(activeSlot, p.bondMgr, p.vault, userInput)

	p.mu.Lock()
	p.history = append(p.history, llm.Message{Role: "user", Content: userInput})
	// 保持滑动窗口不超过 10 轮
	if len(p.history) > 20 {
		p.history = p.history[len(p.history)-20:]
	}

	msgs := make([]llm.Message, 0, len(p.history)+1)
	msgs = append(msgs, llm.Message{Role: "system", Content: systemPrompt})
	msgs = append(msgs, p.history...)
	p.mu.Unlock()

	p.setState(StateThinking)

	sentenceChan := make(chan string, 16)
	errChan := make(chan error, 2)

	// Goroutine 1: 语音朗读消费队列
	var ttsWg sync.WaitGroup
	ttsWg.Add(1)
	go func() {
		defer ttsWg.Done()
		firstChunk := true
		for {
			select {
			case <-ctx.Done():
				p.ttsEngine.Stop()
				return
			case sentence, ok := <-sentenceChan:
				if !ok {
					return
				}
				if firstChunk {
					p.setState(StateSpeaking)
					firstChunk = false
				}
				if p.onSpeechChunk != nil {
					p.onSpeechChunk(sentence)
				}
				_ = p.ttsEngine.Speak(ctx, sentence)
			}
		}
	}()

	// Goroutine 2: LLM 流式生成并送入断句器
	var fullResponse string
	p.splitter.Reset()

	req := llm.ChatCompletionRequest{
		Model:       p.cfg.LLM.Model,
		Messages:    msgs,
		Temperature: p.cfg.LLM.Temperature,
		MaxTokens:   p.cfg.LLM.MaxTokens,
	}

	err := p.llmClient.StreamChat(ctx, req, func(token string) error {
		fullResponse += token
		chunks := p.splitter.Push(token)
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

	// 冲刷剩余未截断文本
	remainders := p.splitter.Flush()
	for _, rem := range remainders {
		select {
		case <-ctx.Done():
		case sentenceChan <- rem:
		}
	}
	close(sentenceChan)

	// 等待 TTS 消费完毕
	ttsWg.Wait()

	if ctx.Err() == nil {
		p.mu.Lock()
		p.history = append(p.history, llm.Message{Role: "assistant", Content: fullResponse})
		p.mu.Unlock()
		p.setState(StateIdle)

		// 5. 羁绊加分
		if p.bondMgr != nil {
			p.bondMgr.AddInteraction()
			if p.onBondSync != nil {
				p.onBondSync(p.bondMgr.GetData())
			}
		}

		// 6. 后台静默记忆提炼
		if p.vault != nil {
			memory.ExtractMemoriesAsync(p.llmClient, p.cfg.LLM.Model, userInput, fullResponse, p.vault)
		}
	}

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}
