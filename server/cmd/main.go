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

	secretKey := os.Getenv("GATEWAY_SECRET_KEY")
	if secretKey == "" {
		secretKey = "secret_avatar_gateway_key_2026"
	}

	tokenSvc := server.NewTokenService(server.TokenConfig{
		SecretKey: secretKey,
		TokenTTL:  5 * time.Minute,
	})

	cloudSrv := server.NewCloudServer(tokenSvc)

	llmBaseURL := os.Getenv("LLM_BASE_URL")
	llmAPIKey := os.Getenv("LLM_API_KEY")
	llmModel := os.Getenv("LLM_MODEL")

	if llmBaseURL == "" {
		llmBaseURL = "http://127.0.0.1:1234/v1" // 默认对接宿主机 LM Studio
	}
	if llmModel == "" {
		llmModel = "default"
	}

	ctxProbe, cancelProbe := context.WithTimeout(context.Background(), 800*time.Millisecond)
	if llm.ProbeEndpoint(ctxProbe, llmBaseURL, llmAPIKey) {
		if firstModel, err := llm.FetchFirstModel(ctxProbe, llmBaseURL, llmAPIKey); err == nil && firstModel != "" && llmModel == "default" {
			llmModel = firstModel
		}
		llmCli := llm.NewClient(llmBaseURL, llmAPIKey)
		cloudSrv.SetLLM(llmCli, llmModel)
		log.Printf("[Cloud Gateway] Upstream LLM bound: %s (model: %s)", llmBaseURL, llmModel)
	} else if detectedURL, detectedModel, ok := llm.AutoDetectEndpoint(ctxProbe); ok {
		llmCli := llm.NewClient(detectedURL, "")
		cloudSrv.SetLLM(llmCli, detectedModel)
		log.Printf("[Cloud Gateway] Auto-detected local LLM: %s (model: %s)", detectedURL, detectedModel)
	} else {
		llmCli := llm.NewClient(llmBaseURL, llmAPIKey)
		cloudSrv.SetLLM(llmCli, llmModel)
		log.Printf("[Cloud Gateway] Upstream LLM configured to %s (model: %s)", llmBaseURL, llmModel)
	}
	cancelProbe()

	log.Printf("[Cloud Gateway] Listening on :%s (Mode: Realtime Text Streaming & Unified Edge TTS)", port)
	if err := http.ListenAndServe(":"+port, cloudSrv.Routes()); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
