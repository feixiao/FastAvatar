package server

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCloudNotConsented = errors.New("ERR_CLOUD_NOT_CONSENTED: user has not consented to cloud processing")
	ErrInvalidIdentity   = errors.New("ERR_INVALID_IDENTITY: user_id or device_id is empty")
)

// TokenConfig holds session token signing configuration.
type TokenConfig struct {
	SecretKey string
	TokenTTL  time.Duration
}

// TokenRequest comes from Pixel or Mac client.
type TokenRequest struct {
	UserID       string `json:"user_id"`
	DeviceID     string `json:"device_id"`
	ClientType   string `json:"client_type"` // "mac" or "pixel"
	CloudConsent bool   `json:"cloud_consent"`
}

// TokenResponse returns session identification and access token.
type TokenResponse struct {
	SessionID string `json:"session_id"`
	RoomName  string `json:"room_name"` // Compatibility alias for session_id
	Token     string `json:"token"`
	ExpiresIn int64  `json:"expires_in_seconds"`
}

// TokenService creates isolated session tokens with strict PRD guardrails.
type TokenService struct {
	cfg TokenConfig
}

// NewTokenService creates a new token service.
func NewTokenService(cfg TokenConfig) *TokenService {
	if cfg.TokenTTL == 0 {
		cfg.TokenTTL = 5 * time.Minute // Short-lived token per PRD
	}
	if cfg.SecretKey == "" {
		cfg.SecretKey = "secret_avatar_gateway_key_2026"
	}
	return &TokenService{cfg: cfg}
}

// IssueToken validates consent & identity, creates an isolated session name, and signs bearer token.
func (s *TokenService) IssueToken(req TokenRequest) (*TokenResponse, error) {
	// Guardrail 1: Explicit user consent check
	if !req.CloudConsent {
		return nil, ErrCloudNotConsented
	}

	// Guardrail 2: Identity validation
	if req.UserID == "" || req.DeviceID == "" {
		return nil, ErrInvalidIdentity
	}

	// Guardrail 3: Isolated Session / Room Name per device/session
	roomName := fmt.Sprintf("room_sess_%s_%d", req.DeviceID, time.Now().UnixNano())

	// Sign HMAC-SHA256 Token with expiry
	expiresAt := time.Now().Add(s.cfg.TokenTTL).Unix()
	payload := fmt.Sprintf("%s:%s:%s:%d", roomName, req.UserID, req.DeviceID, expiresAt)
	h := hmac.New(sha256.New, []byte(s.cfg.SecretKey))
	h.Write([]byte(payload))
	sig := hex.EncodeToString(h.Sum(nil))
	token := base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + sig

	return &TokenResponse{
		SessionID: roomName,
		RoomName:  roomName,
		Token:     token,
		ExpiresIn: int64(s.cfg.TokenTTL.Seconds()),
	}, nil
}
