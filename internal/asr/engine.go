package asr

import "context"

// Engine defines the speech-to-text and voice activity detection interface.
type Engine interface {
	// Start starts the microphone capture and ASR processing.
	Start(ctx context.Context) error

	// Stop terminates the microphone capture and shuts down the engine.
	Stop() error

	// SetMode configures the interaction mode: "hands_free" or "ptt".
	SetMode(mode string) error

	// SetSpeaking informs the ASR whether the companion is currently speaking (for echo-cancellation & barge-in).
	SetSpeaking(speaking bool) error

	// PTTStart triggers recording for push-to-talk mode.
	PTTStart() error

	// PTTStop ends recording for push-to-talk mode and triggers transcription.
	PTTStop() error

	// SetCallbacks registers event handlers for streaming partials, final utterances, VAD, and errors.
	SetCallbacks(
		onPartial func(text string),
		onFinal func(text string),
		onVAD func(speaking bool, energy float64),
		onError func(err string),
	)
}
