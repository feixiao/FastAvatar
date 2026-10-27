package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

type ChatCompletionChunk struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
}

type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ProbeEndpoint 检测指定服务端点是否可用
func ProbeEndpoint(ctx context.Context, baseURL, apiKey string) bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	url := fmt.Sprintf("%s/models", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// FetchFirstModel 获取可用模型列表中的首个模型名称
func FetchFirstModel(ctx context.Context, baseURL, apiKey string) (string, error) {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	url := fmt.Sprintf("%s/models", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return "", err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status: %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	for _, m := range payload.Data {
		if !strings.Contains(m.ID, "embed") {
			return m.ID, nil
		}
	}
	if len(payload.Data) > 0 {
		return payload.Data[0].ID, nil
	}
	return "", fmt.Errorf("no models available")
}

// AutoDetectEndpoint 自动探测本地正在运行的大模型服务 (LM Studio, Ollama, LocalAI 等)
func AutoDetectEndpoint(ctx context.Context) (baseURL string, model string, ok bool) {
	candidates := []string{
		"http://127.0.0.1:1234/v1",  // LM Studio
		"http://127.0.0.1:11434/v1", // Ollama
		"http://127.0.0.1:8000/v1",  // vLLM / LocalAI
		"http://127.0.0.1:50088/v1", // Avatar sidecar
	}
	for _, c := range candidates {
		if ProbeEndpoint(ctx, c, "") {
			m, _ := FetchFirstModel(ctx, c, "")
			return c, m, true
		}
	}
	return "", "", false
}

// StreamChat 发送流式对话请求，并通过 onToken 回调输出逐字 Token
func (c *Client) StreamChat(ctx context.Context, req ChatCompletionRequest, onToken func(token string) error) error {
	req.Stream = true
	bodyBytes, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", c.BaseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if c.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("post chat completions: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}

	reader := bufio.NewReader(resp.Body)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return fmt.Errorf("read stream: %w", err)
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data:") {
			continue
		}

		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "[DONE]" {
			break
		}

		var chunk ChatCompletionChunk
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
			token := chunk.Choices[0].Delta.Content
			if err := onToken(token); err != nil {
				return err
			}
		}
	}

	return nil
}
