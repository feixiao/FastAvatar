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
	"regexp"
	"sort"
	"strconv"
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

var paramSizeRegex = regexp.MustCompile(`(?i)(?:^|[^0-9.])([0-9]+(?:\.[0-9]+)?)\s*b(?:[^a-z0-9]|$)`)

// AnalyzeModelAttributes 解析模型名称，分析其估算参数量（以B为单位）、是否思考模型及是否Embedding模型
func AnalyzeModelAttributes(name string) (approxSize float64, isThinking bool, isEmbedding bool) {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "embed") || strings.Contains(lower, "rerank") || strings.Contains(lower, "bge-") {
		return 0, false, true
	}
	if strings.Contains(lower, "thinking") || strings.Contains(lower, "reason") || strings.Contains(lower, "-r1") || strings.Contains(lower, "deepseek-r1") {
		isThinking = true
	}

	matches := paramSizeRegex.FindStringSubmatch(lower)
	if len(matches) >= 2 {
		if val, err := strconv.ParseFloat(matches[1], 64); err == nil && val > 0 {
			return val, isThinking, false
		}
	}

	switch {
	case strings.Contains(lower, "tiny") || strings.Contains(lower, "mini"):
		approxSize = 1.0
	case strings.Contains(lower, "small"):
		approxSize = 3.0
	case strings.Contains(lower, "large") || strings.Contains(lower, "max"):
		approxSize = 32.0
	default:
		approxSize = 7.0
	}
	return approxSize, isThinking, false
}

// SelectBestEdgeModel 挑选最适合端侧实时极速对话的小模型 (严禁思考模型，优先参数量小、Instruct/Chat对齐的小模型)
func SelectBestEdgeModel(models []string) string {
	if len(models) == 0 {
		return ""
	}
	var nonThinking []string
	for _, m := range models {
		_, isThinking, isEmbed := AnalyzeModelAttributes(m)
		if isEmbed {
			continue
		}
		if !isThinking {
			nonThinking = append(nonThinking, m)
		}
	}
	pool := nonThinking
	if len(pool) == 0 {
		for _, m := range models {
			_, _, isEmbed := AnalyzeModelAttributes(m)
			if !isEmbed {
				pool = append(pool, m)
			}
		}
	}
	if len(pool) == 0 {
		return models[0]
	}

	sort.SliceStable(pool, func(i, j int) bool {
		sizeI, _, _ := AnalyzeModelAttributes(pool[i])
		sizeJ, _, _ := AnalyzeModelAttributes(pool[j])
		if sizeI != sizeJ {
			return sizeI < sizeJ
		}
		lowI := strings.ToLower(pool[i])
		lowJ := strings.ToLower(pool[j])
		isInstI := strings.Contains(lowI, "instruct") || strings.Contains(lowI, "chat")
		isInstJ := strings.Contains(lowJ, "instruct") || strings.Contains(lowJ, "chat")
		if isInstI != isInstJ {
			return isInstI
		}
		return len(pool[i]) < len(pool[j])
	})
	return pool[0]
}

// SelectBestCloudModel 挑选最适合云端/深层次复杂推理的大模型 (参数量大、深度认知模型优先)
func SelectBestCloudModel(models []string, edgeModel string) string {
	if len(models) == 0 {
		return ""
	}
	var candidates []string
	for _, m := range models {
		_, _, isEmbed := AnalyzeModelAttributes(m)
		if !isEmbed {
			candidates = append(candidates, m)
		}
	}
	if len(candidates) == 0 {
		return edgeModel
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		sizeI, _, _ := AnalyzeModelAttributes(candidates[i])
		sizeJ, _, _ := AnalyzeModelAttributes(candidates[j])
		if sizeI != sizeJ {
			return sizeI > sizeJ // 越大越适合作为慢环大模型
		}
		return len(candidates[i]) > len(candidates[j])
	})

	for _, c := range candidates {
		if c != edgeModel {
			return c
		}
	}
	return candidates[0]
}

// FetchLoadedModels 优先获取本地端点当前已加载到内存/显存中的模型
func FetchLoadedModels(ctx context.Context, baseURL, apiKey string) ([]string, error) {
	origin := strings.TrimSuffix(baseURL, "/v1")
	origin = strings.TrimSuffix(origin, "/")

	// 1. 尝试 LM Studio 原生接口 (/api/v0/models)
	lmStudioURL := origin + "/api/v0/models"
	req, err := http.NewRequestWithContext(ctx, "GET", lmStudioURL, nil)
	if err == nil {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var lmRes struct {
					Data []struct {
						ID    string `json:"id"`
						State string `json:"state"`
					} `json:"data"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&lmRes); err == nil {
					var loaded []string
					for _, m := range lmRes.Data {
						if m.State == "loaded" && !strings.Contains(strings.ToLower(m.ID), "embed") {
							loaded = append(loaded, m.ID)
						}
					}
					if len(loaded) > 0 {
						return loaded, nil
					}
				}
			}
		}
	}

	// 2. 尝试 Ollama 运行中模型接口 (/api/ps)
	ollamaURL := origin + "/api/ps"
	req, err = http.NewRequestWithContext(ctx, "GET", ollamaURL, nil)
	if err == nil {
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var olRes struct {
					Models []struct {
						Name  string `json:"name"`
						Model string `json:"model"`
					} `json:"models"`
				}
				if err := json.NewDecoder(resp.Body).Decode(&olRes); err == nil {
					var loaded []string
					for _, m := range olRes.Models {
						n := m.Name
						if n == "" {
							n = m.Model
						}
						if n != "" && !strings.Contains(strings.ToLower(n), "embed") {
							loaded = append(loaded, n)
						}
					}
					if len(loaded) > 0 {
						return loaded, nil
					}
				}
			}
		}
	}

	// 3. 通用标准端点 (/v1/models) 回退
	return FetchAllModels(ctx, baseURL, apiKey)
}

// FetchFirstModel 获取可用模型列表中的首选轻量模型名称
func FetchFirstModel(ctx context.Context, baseURL, apiKey string) (string, error) {
	models, err := FetchLoadedModels(ctx, baseURL, apiKey)
	if err != nil || len(models) == 0 {
		return "", err
	}
	best := SelectBestEdgeModel(models)
	if best != "" {
		return best, nil
	}
	return models[0], nil
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
	models, err := FetchLoadedModels(ctx, baseURL, apiKey)
	if err != nil || len(models) == 0 {
		return "", "", false
	}
	if len(models) == 1 {
		return models[0], models[0], true
	}
	edge := SelectBestEdgeModel(models)
	cloud := SelectBestCloudModel(models, edge)
	return edge, cloud, true
}

// AutoDetectDualEndpoints 自动探测并配置端云双模型 (LM Studio 双模型加载或 LM Studio+Ollama)
func AutoDetectDualEndpoints(ctx context.Context) (edgeURL, edgeModel, cloudURL, cloudModel string, ok bool) {
	lmStudioURL := "http://127.0.0.1:1234/v1"
	ollamaURL := "http://127.0.0.1:11434/v1"

	lmOk := ProbeEndpoint(ctx, lmStudioURL, "")
	ollamaOk := ProbeEndpoint(ctx, ollamaURL, "")

	if lmOk {
		models, _ := FetchLoadedModels(ctx, lmStudioURL, "")
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
		models, _ := FetchLoadedModels(ctx, ollamaURL, "")
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
