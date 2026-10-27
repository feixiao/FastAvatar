package interaction

import (
	"context"
	"fmt"
	"sync"
	"time"

	"avatar/internal/orchestrator"
	"avatar/internal/tts"
)

// SpeechPlayer implements orchestrator.SpeechOutput wrapping tts.Synthesizer.
type SpeechPlayer struct {
	synth     tts.Synthesizer
	mu        sync.RWMutex
	playbackID int64
}

func NewSpeechPlayer(synth tts.Synthesizer) *SpeechPlayer {
	if synth == nil {
		synth = tts.NewMockSynthesizer()
	}
	return &SpeechPlayer{
		synth: synth,
	}
}

// Speak synthesizes and plays audio for the given text.
func (p *SpeechPlayer) Speak(ctx context.Context, text string) (orchestrator.Playback, error) {
	p.mu.Lock()
	p.playbackID++
	id := fmt.Sprintf("pb-%d", p.playbackID)
	p.mu.Unlock()

	startTime := time.Now()
	err := p.synth.Speak(ctx, text)
	endTime := time.Now()

	pb := orchestrator.Playback{
		PlaybackID: id,
		Text:       text,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   endTime.Sub(startTime),
		PlayedText: text,
		Completed:  err == nil,
		Interrupted: err != nil && ctx.Err() != nil,
	}
	return pb, err
}

// Stop stops current audio playback immediately.
func (p *SpeechPlayer) Stop(ctx context.Context) error {
	p.synth.Stop()
	return nil
}
