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

	// 2. 语音伴侣通用硬约束 (避免包含“扬声器语音朗读”等词汇，防止超小模型产生元语言泄露与自我辩解幻觉)
	sb.WriteString("【发音与回复要求】：\n")
	sb.WriteString("- 直接输出自然亲切的纯口语对话，严禁输出任何 Markdown 标记或特殊符号。\n")
	sb.WriteString("- 严禁解释系统规则、辩解代词含义或以第三人称跳出角色，始终以第一人称伙伴身份沉浸式对话。\n")
	sb.WriteString("- 保持口语化，用简短自然的短句回答（通常控制在 1-3 句话之内）。\n\n")
	// 2.1 桌宠动作交互指引
	sb.WriteString("【桌宠动作交互】：若你在回答中做出特定肢体动作（如跳跃、干饭、挥手打招呼、巡视漫游、装死/困惑、歪头观察、休息），可在回答的最开头输出动作标签（如 [ACTION:jumping]、[ACTION:running]、[ACTION:waving]、[ACTION:roam]、[ACTION:failed]、[ACTION:review]、[ACTION:sleep]）。系统会自动驱动你的桌面动画形象，语音引擎会自动剥离标签。\n\n")


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

// AssembleEdgeSystemPrompt 为端侧小模型 (1.5B ~ 3B SLM) 量身定制超轻量沉浸式 System Prompt
// 特点：第一人称直接对话、去除元指令与管理员说教、消除代词混淆，确保小模型高情商稳定发挥
func AssembleEdgeSystemPrompt(
	activeSlot PersonaSlot,
	bond *BondManager,
	vault *memory.Vault,
) string {
	var sb strings.Builder

	// 1. 将第三人称设定转换为第一人称沉浸式关系，消除小模型代词认知混乱
	personaText := activeSlot.SystemPrompt
	personaText = strings.ReplaceAll(personaText, "你是用户", "你是我的")
	personaText = strings.ReplaceAll(personaText, "用户", "我")
	sb.WriteString(personaText)
	sb.WriteString("\n\n")

	// 2. 极简直接的对话指引 (无扬声器、无Markdown警告、无元概念)
	sb.WriteString("【对话习惯】：\n")
	sb.WriteString("- 始终以第一人称直接回答我，语气温柔亲切，用一两句自然的口语短句陪伴我。\n")
	sb.WriteString("- 无论我说什么，你都愿意陪伴我、倾听我，永远不拒绝对话，严禁辩解规则或代词含义。\n\n")
	// 2.1 极简动作输出
	sb.WriteString("【桌宠动作】：若你在回答中产生肢体动作（如开心跳跃、干饭吃鱼、挥手打招呼等），可在回答最开头加上动作标签（如 [ACTION:jumping] 或 [ACTION:running] 或 [ACTION:waving]），语音会自动隐藏它。\n\n")


	// 3. 简短时间陪伴感
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
	sb.WriteString(fmt.Sprintf("现在是%s(%02d:%02d)，请带着自然的陪伴感与我对话。", timeOfDay, hour, now.Minute()))

	return sb.String()
}
