package server

import (
	"errors"
	"fmt"
	"time"

	"github.com/livekit/protocol/auth"
)

var (
	ErrCloudNotConsented = errors.New("ERR_CLOUD_NOT_CONSENTED: user has not consented to cloud processing")
	ErrInvalidIdentity   = errors.New("ERR_INVALID_IDENTITY: user_id or device_id is empty")
)

// TokenConfig holds LiveKit API key/secret and configuration.
type TokenConfig struct {
	APIKey      string
	APISecret   string
	LiveKitURL  string
	TokenTTL    time.Duration
}

// TokenRequest comes from Pixel or Mac client.
type TokenRequest struct {
	UserID       string `json:"user_id"`
	DeviceID     string `json:"device_id"`
	ClientType   string `json:"client_type"` // "mac" or "pixel"
	CloudConsent bool   `json:"cloud_consent"`
}

// TokenResponse returns livekit url, assigned room, and access token.
type TokenResponse struct {
	LiveKitURL string `json:"livekit_url"`
	RoomName   string `json:"room_name"`
	Token      string `json:"token"`
	ExpiresIn  int64  `json:"expires_in_seconds"`
}

// TokenService creates isolated LiveKit room tokens with strict PRD guardrails.
type TokenService struct {
	cfg TokenConfig
}

// NewTokenService creates a new token service.
func NewTokenService(cfg TokenConfig) *TokenService {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 5 * time.Minute // Short-lived token per PRD
	}
	if cfg.APIKey == "" {
		cfg.APIKey = "devkey"
	}
	if cfg.APISecret == "" {
		cfg.APISecret = "secret_avatar_livekit_key_2026"
	}
	return &TokenService{cfg: cfg}
}

// IssueToken validates consent & identity, creates an isolated room name, and signs JWT.
func (s *TokenService) IssueToken(req TokenRequest) (*TokenResponse, error) {
	// Guardrail 1: Explicit user consent check
	if !req.CloudConsent {
		return nil, ErrCloudNotConsented
	}

	// Guardrail 2: Identity validation
	if req.UserID == "" || req.DeviceID == "" {
		return nil, ErrInvalidIdentity
	}

	// Guardrail 3: Isolated Room Name per device/session
	roomName := fmt.Sprintf("room_sess_%s_%d", req.DeviceID, time.Now().UnixNano())

	// Build LiveKit Grant
	t := true
	at := auth.NewAccessToken(s.cfg.APIKey, s.cfg.APISecret)
	grant := &auth.VideoGrant{
		RoomJoin:       true,
		Room:           roomName,
		CanPublish:     &t,
		CanSubscribe:   &t,
		CanPublishData: &t,
	}

	at.AddGrant(grant).
		SetIdentity(fmt.Sprintf("%s:%s", req.ClientType, req.DeviceID)).
		SetName(req.UserID).
		SetValidFor(s.cfg.TokenTTL)

	jwt, err := at.ToJWT()
	if err != nil {
		return nil, fmt.Errorf("failed to sign token: %w", err)
	}

	return &TokenResponse{
		LiveKitURL: s.cfg.LiveKitURL,
		RoomName:   roomName,
		Token:      jwt,
		ExpiresIn:  int64(s.cfg.TokenTTL.Seconds()),
	}, nil
}
