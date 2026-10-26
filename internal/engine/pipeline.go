package engine

import (
	"context"
	"fmt"
	"sync"

	"avatar/internal/config"
	"avatar/internal/llm"
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
	cfg         config.AppConfig
	llmClient   *llm.Client
	ttsEngine   tts.Synthesizer
	splitter    *SentenceSplitter
	state       State
	history     []llm.Message
	activeCtx   context.Context
	cancelFunc  context.CancelFunc
	mu          sync.RWMutex
	onStateChange func(State)
	onSpeechChunk func(string)
}

func NewPipeline(cfg config.AppConfig, synthesizer tts.Synthesizer) *Pipeline {
	if synthesizer == nil {
		synthesizer = tts.NewMacSaySynthesizer(cfg.TTS.Voice, cfg.TTS.Rate)
	}

	p := &Pipeline{
		cfg:       cfg,
		llmClient: llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey),
		ttsEngine: synthesizer,
		splitter:  NewSentenceSplitter(),
		state:     StateIdle,
		history: []llm.Message{
			{
				Role:    "system",
				Content: "你是一个温柔、亲切且言简意赅的桌面语音伴侣。回答简明口语化，适合直接语音朗读。",
			},
		},
	}
	return p
}

func (p *Pipeline) SetCallbacks(onState func(State), onSpeech func(string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onStateChange = onState
	p.onSpeechChunk = onSpeech
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

// Chat 接收用户问题，启动流式对话 -> 断句 -> 语音朗读
func (p *Pipeline) Chat(parentCtx context.Context, userInput string) error {
	p.Interrupt()

	ctx, cancel := context.WithCancel(parentCtx)
	p.mu.Lock()
	p.activeCtx = ctx
	p.cancelFunc = cancel
	p.history = append(p.history, llm.Message{Role: "user", Content: userInput})
	msgs := make([]llm.Message, len(p.history))
	copy(msgs, p.history)
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
	}

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}
