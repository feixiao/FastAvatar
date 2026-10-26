package persona

import (
	"fmt"
	"strings"
	"time"

	"avatar/internal/memory"
)

// AssembleSystemPrompt 在 < 1ms 内将人格、羁绊、核心画像、情景记忆与时间感知合成完整的 System Prompt
func AssembleSystemPrompt(
	activeSlot PersonaSlot,
	bond *BondManager,
	vault *memory.Vault,
	userQuery string,
) string {
	var sb strings.Builder

	// 1. 人格核心设定与口吻
	sb.WriteString(activeSlot.SystemPrompt)
	sb.WriteString("\n\n")

	// 2. 语音伴侣通用硬约束
	sb.WriteString("【发音与回复要求】：\n")
	sb.WriteString("- 你正在直接通过扬声器对用户语音朗读，严禁输出任何 Markdown 标记（如粗体、列表、代码块、URL等）。\n")
	sb.WriteString("- 保持口语化，用简短自然的短句回答（通常控制在 1-3 句话之内）。\n\n")

	// 3. 时间感知
	now := time.Now()
	hour := now.Hour()
	timeOfDay := "白天"
	if hour >= 5 && hour < 9 {
		timeOfDay = "清晨"
	} else if hour >= 9 && hour < 12 {
		timeOfDay = "上午"
	} else if hour >= 12 && hour < 18 {
		timeOfDay = "下午"
	} else if hour >= 18 && hour < 23 {
		timeOfDay = "傍晚"
	} else {
		timeOfDay = "深夜"
	}
	sb.WriteString(fmt.Sprintf("【当前时间】：%s (%02d:%02d)，请根据时段流露自然的时间关怀。\n\n",
		timeOfDay, hour, now.Minute()))

	// 4. 关系羁绊
	if bond != nil {
		sb.WriteString(bond.FormatBondForPrompt())
		sb.WriteString("\n\n")
	}

	// 5. 核心画像记忆
	if vault != nil {
		profileText := vault.FormatProfileForPrompt()
		if profileText != "" {
			sb.WriteString(profileText)
			sb.WriteString("\n")
		}

		// 6. 召回相关的近期事件
		episodes := vault.RetrieveRelevantEpisodes(userQuery, 3)
		episodesText := memory.FormatEpisodesForPrompt(episodes)
		if episodesText != "" {
			sb.WriteString(episodesText)
			sb.WriteString("\n")
		}
	}

	return sb.String()
}
