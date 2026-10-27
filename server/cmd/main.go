package main

import (
	"log"
	"net/http"
	"os"
	"time"

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

	log.Printf("[Cloud Gateway] Listening on :%s (LiveKit SFU: %s)", port, livekitURL)
	if err := http.ListenAndServe(":"+port, cloudSrv.Routes()); err != nil {
		log.Fatalf("Server exited: %v", err)
	}
}
