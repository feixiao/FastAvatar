package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestStreamChat(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[]}`))
			return
		}
		if r.URL.Path == "/chat/completions" {
			w.Header().Set("Content-Type", "text/event-stream")
			w.WriteHeader(http.StatusOK)
			flusher, ok := w.(http.Flusher)
			if !ok {
				t.Fatal("expected flusher")
			}

			tokens := []string{"你", "好", "呀", "！"}
			for _, tok := range tokens {
				chunk := "data: {\"choices\":[{\"delta\":{\"content\":\"" + tok + "\"}}]}\n\n"
				_, _ = fmt.Fprint(w, chunk)
				flusher.Flush()
			}
			_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if !ProbeEndpoint(ctx, ts.URL, "") {
		t.Fatal("ProbeEndpoint failed")
	}

	client := NewClient(ts.URL, "")
	var received string
	err := client.StreamChat(ctx, ChatCompletionRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: "user", Content: "hi"},
		},
	}, func(tok string) error {
		received += tok
		return nil
	})

	if err != nil {
		t.Fatalf("StreamChat failed: %v", err)
	}

	if received != "你好呀！" {
		t.Fatalf("expected '你好呀！', got '%s'", received)
	}
}

func TestAutoDetectEndpoint(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"data":[{"id":"mock-qwen-model"}]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	model, err := FetchFirstModel(ctx, ts.URL, "")
	if err != nil {
		t.Fatalf("FetchFirstModel failed: %v", err)
	}
	if model != "mock-qwen-model" {
		t.Fatalf("expected 'mock-qwen-model', got '%s'", model)
	}
}
