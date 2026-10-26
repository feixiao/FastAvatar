package engine

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"avatar/internal/config"
	"avatar/internal/tts"
)

func TestPipelineChatAndInterrupt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, _ := w.(http.Flusher)

		tokens := []string{"你", "好", "呀", "！", "我", "是", "小", "助", "手", "。"}
		for _, tok := range tokens {
			chunk := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"}}]}\n\n", tok)
			_, _ = fmt.Fprint(w, chunk)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
		_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
		flusher.Flush()
	}))
	defer ts.Close()

	cfg := config.LoadConfig()
	cfg.LLM.BaseURL = ts.URL
	cfg.LLM.Model = "test-model"

	mockTTS := tts.NewMockSynthesizer()
	pipeline := NewPipeline(cfg, mockTTS)

	var states []State
	pipeline.SetCallbacks(func(s State) {
		states = append(states, s)
	}, nil)

	ctx := context.Background()
	err := pipeline.Chat(ctx, "你好")
	if err != nil {
		t.Fatalf("Chat error: %v", err)
	}

	if len(mockTTS.Spoken) == 0 {
		t.Fatal("expected TTS to speak chunks")
	}

	if pipeline.State() != StateIdle {
		t.Fatalf("expected state Idle, got %s", pipeline.State())
	}
}
