package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"avatar/internal/llm"
)

type ExtractedMemory struct {
	Type     string   `json:"type"`     // "profile" 或 "episode"
	Category string   `json:"category"` // identity, preference, habit, work, event
	Key      string   `json:"key"`
	Value    string   `json:"value"`
	Tags     []string `json:"tags"`
}

// ExtractMemoriesAsync 在对话完成后由后台 Goroutine 静默调用，提炼新记忆
func ExtractMemoriesAsync(llmClient *llm.Client, model string, userQuery, assistantReply string, vault *Vault) {
	if llmClient == nil || vault == nil || len(userQuery) < 4 {
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()

		systemPrompt := `你是一个敏锐的伴侣记忆提炼器。请分析用户与助手的这轮对话，提炼出关于用户的“长期核心特征”(profile)或“近期重要事件”(episode)。
严格规则：
1. 仅提炼用户本人明确表达出的信息（如：职业、喜好、宠物、作息、正在做的事情、计划等）。
2. 如果没有任何值得记住的个人事实或仅为打招呼/闲聊，必须返回空数组 []。
3. 严格输出标准 JSON 数组，禁止任何多余解释。
格式示例：
[
  {"type": "profile", "category": "preference", "key": "饮食偏好", "value": "喝咖啡不加糖"},
  {"type": "episode", "category": "event", "value": "今天把项目重构成Go语言", "tags": ["编程", "工作"]}
]`

		userPrompt := fmt.Sprintf("用户说: %s\n助手回复: %s", userQuery, assistantReply)

		var outputBuilder strings.Builder
		err := llmClient.StreamChat(ctx, llm.ChatCompletionRequest{
			Model: model,
			Messages: []llm.Message{
				{Role: "system", Content: systemPrompt},
				{Role: "user", Content: userPrompt},
			},
			Temperature: 0.1,
			MaxTokens:   256,
		}, func(token string) error {
			outputBuilder.WriteString(token)
			return nil
		})

		if err != nil {
			return
		}

		rawJSON := strings.TrimSpace(outputBuilder.String())
		// 清理可能存在的 markdown code block
		rawJSON = strings.TrimPrefix(rawJSON, "```json")
		rawJSON = strings.TrimPrefix(rawJSON, "```")
		rawJSON = strings.TrimSuffix(rawJSON, "```")
		rawJSON = strings.TrimSpace(rawJSON)

		var items []ExtractedMemory
		if err := json.Unmarshal([]byte(rawJSON), &items); err != nil {
			return
		}

		for _, item := range items {
			if item.Type == "profile" && item.Key != "" && item.Value != "" {
				vault.AddOrUpdateProfile(item.Category, item.Key, item.Value)
			} else if item.Type == "episode" && item.Value != "" {
				vault.AddEpisode(item.Value, item.Tags)
			}
		}
	}()
}
