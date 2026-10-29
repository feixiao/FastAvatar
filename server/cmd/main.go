package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"avatar/internal/llm"
	"avatar/server"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8088"
	}

	livekitURL := os.Getenv("LIVEKIT_URL")
	if livekitURL == "" {
		livekitURL = "ws://localhost:7880"
	}

	apiKey := os.Getenv("LIVEKIT_API_KEY")
	if apiKey == "" {
		apiKey = "devkey"
	}

	apiSecret := os.Getenv("LIVEKIT_API_SECRET")
	if apiSecret == "" {
		apiSecret = "secret_avatar_livekit_key_2026"
	}

	tokenSvc := server.NewTokenService(server.TokenConfig{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		LiveKitURL: livekitURL,
		TokenTTL:   5 * time.Minute,
	})

	cloudSrv := server.NewCloudServer(tokenSvc)

	llmBaseURL := os.Getenv("LLM_BASE_URL")
	llmAPIKey := os.Getenv("LLM_API_KEY")
	llmModel := os.Getenv("LLM_MODEL")

	if llmBaseURL != "" {
		if llmModel == "" {
			llmModel = "default"
		}
		llmCli := llm.NewClient(llmBaseURL, llmAPIKey)
		cloudSrv.SetLLM(llmCli, llmModel)
		log.Printf("[Cloud Gateway] Upstream LLM bound: %s (model: %s)", llmBaseURL, llmModel)
	} else {
		ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 800*time.Millisecond)
		if detectedURL, detectedModel, ok := llm.AutoDetectEndpoint(ctxProbe); ok {
			llmCli := llm.NewClient(detectedURL, "")
			cloudSrv.SetLLM(llmCli, detectedModel)
			log.Printf("[Cloud Gateway] Auto-detected local LLM: %s (model: %s)", detectedURL, detectedModel)
		} else {
			log.Printf("[Cloud Gateway] No upstream LLM specified; using fallback mock persona responder")
		}
		cancelProbe()
	}

	log.Printf("[Cloud Gateway] Listening on :%s (LiveKit SFU: %s)", port, livekitURL)
	if err := http.ListenAndServe(":"+port, cloudSrv.Routes()); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
