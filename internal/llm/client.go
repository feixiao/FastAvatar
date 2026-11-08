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
			Content          string `json:"content"`
			ReasoningContent string `json:"reasoning_content"`
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
// FetchAllModels 获取可用模型列表中的所有非 embedding 模型
func FetchAllModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	url := fmt.Sprintf("%s/models", strings.TrimRight(baseURL, "/"))
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %d", resp.StatusCode)
	}

	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, err
	}

	var models []string
	for _, m := range payload.Data {
		if !strings.Contains(strings.ToLower(m.ID), "embed") {
			models = append(models, m.ID)
		}
	}
	return models, nil
}

// DetectDualModels 从指定端点智能分析分离端侧小模型与云端大模型
func DetectDualModels(ctx context.Context, baseURL, apiKey string) (edgeModel string, cloudModel string, ok bool) {
	models, err := FetchAllModels(ctx, baseURL, apiKey)
	if err != nil || len(models) == 0 {
		return "", "", false
	}
	if len(models) == 1 {
		return models[0], models[0], true
	}

	smallKeywords := []string{"0.5b", "1.5b", "1b", "2b", "3b", "mini", "small", "tiny"}
	largeKeywords := []string{"7b", "8b", "14b", "32b", "70b", "deepseek", "large", "max"}

	var foundSmall, foundLarge string
	for _, m := range models {
		lower := strings.ToLower(m)
		if foundSmall == "" {
			for _, kw := range smallKeywords {
				if strings.Contains(lower, kw) {
					foundSmall = m
					break
				}
			}
		}
		if foundLarge == "" {
			for _, kw := range largeKeywords {
				if strings.Contains(lower, kw) {
					foundLarge = m
					break
				}
			}
		}
	}

	if foundSmall != "" && foundLarge != "" && foundSmall != foundLarge {
		return foundSmall, foundLarge, true
	}
	if foundSmall != "" {
		for _, m := range models {
			if m != foundSmall {
				return foundSmall, m, true
			}
		}
	}
	if foundLarge != "" {
		for _, m := range models {
			if m != foundLarge {
				return m, foundLarge, true
			}
		}
	}

	return models[0], models[1], true
}

// AutoDetectDualEndpoints 自动探测并配置端云双模型 (LM Studio 双模型加载或 LM Studio+Ollama)
func AutoDetectDualEndpoints(ctx context.Context) (edgeURL, edgeModel, cloudURL, cloudModel string, ok bool) {
	lmStudioURL := "http://127.0.0.1:1234/v1"
	ollamaURL := "http://127.0.0.1:11434/v1"

	lmOk := ProbeEndpoint(ctx, lmStudioURL, "")
	ollamaOk := ProbeEndpoint(ctx, ollamaURL, "")

	if lmOk {
		models, _ := FetchAllModels(ctx, lmStudioURL, "")
		if len(models) >= 2 {
			eM, cM, _ := DetectDualModels(ctx, lmStudioURL, "")
			return lmStudioURL, eM, lmStudioURL, cM, true
		}
		if len(models) == 1 {
			if ollamaOk {
				olModel, _ := FetchFirstModel(ctx, ollamaURL, "")
				return ollamaURL, olModel, lmStudioURL, models[0], true
			}
			return lmStudioURL, models[0], lmStudioURL, models[0], true
		}
	}

	if ollamaOk {
		models, _ := FetchAllModels(ctx, ollamaURL, "")
		if len(models) >= 2 {
			eM, cM, _ := DetectDualModels(ctx, ollamaURL, "")
			return ollamaURL, eM, ollamaURL, cM, true
		}
		if len(models) == 1 {
			return ollamaURL, models[0], ollamaURL, models[0], true
		}
	}

	return "", "", "", "", false
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
	hasContent := false
	var reasoningBuf strings.Builder
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

		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			if delta.Content != "" {
				hasContent = true
				if err := onToken(delta.Content); err != nil {
					return err
				}
			} else if delta.ReasoningContent != "" {
				reasoningBuf.WriteString(delta.ReasoningContent)
			}
		}
	}

	// 若大模型（如思考模型）耗尽 Token 未吐出 content，但已产生思考过程，则提取兜底答复
	if !hasContent && reasoningBuf.Len() > 0 {
		raw := strings.TrimSpace(reasoningBuf.String())
		lines := strings.Split(raw, "\n")
		var fallback string
		for i := len(lines) - 1; i >= 0; i-- {
			l := strings.TrimSpace(lines[i])
			if l != "" && len([]rune(l)) > 4 {
				fallback = l
				break
			}
		}
		if fallback == "" {
			fallback = raw
		}
		_ = onToken(fallback)
	}

	return nil
}
