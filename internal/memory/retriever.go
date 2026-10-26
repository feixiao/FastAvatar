package memory

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

type ScoredEpisode struct {
	Item  EpisodeItem
	Score float64
}

// RetrieveRelevantEpisodes 根据 Query 与时间衰减召回 Top-K 相关事件
func (v *Vault) RetrieveRelevantEpisodes(query string, topK int) []EpisodeItem {
	v.mu.RLock()
	episodes := make([]EpisodeItem, len(v.data.Episodes))
	copy(episodes, v.data.Episodes)
	v.mu.RUnlock()

	if len(episodes) == 0 || topK <= 0 {
		return nil
	}

	queryLower := strings.ToLower(query)
	now := time.Now().Unix()

	var scored []ScoredEpisode

	for _, ep := range episodes {
		var matchScore float64

		// 1. 标签匹配
		for _, tag := range ep.Tags {
			if strings.Contains(queryLower, strings.ToLower(tag)) {
				matchScore += 3.0
			}
		}

		// 2. 内容关键词重叠
		contentLower := strings.ToLower(ep.Content)
		if strings.Contains(contentLower, queryLower) || strings.Contains(queryLower, contentLower) {
			matchScore += 2.0
		} else {
			// 分词匹配（针对短词）
			queryRunes := []rune(queryLower)
			for i := 0; i+1 < len(queryRunes); i += 2 {
				sub := string(queryRunes[i : i+2])
				if strings.Contains(contentLower, sub) {
					matchScore += 0.3
				}
			}
		}

		// 若有基本匹配度，引入时间衰减（最近 7 天权重更高）
		if matchScore > 0 {
			ageDays := float64(now-ep.CreatedAt) / 86400.0
			if ageDays < 0 {
				ageDays = 0
			}
			timeDecay := 1.0 / (1.0 + math.Log1p(ageDays))
			finalScore := matchScore * timeDecay

			scored = append(scored, ScoredEpisode{
				Item:  ep,
				Score: finalScore,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	limit := topK
	if len(scored) < limit {
		limit = len(scored)
	}

	result := make([]EpisodeItem, limit)
	for i := range limit {
		result[i] = scored[i].Item
	}
	return result
}

// FormatEpisodesForPrompt 将召回的事件格式化为 Prompt 上下文
func FormatEpisodesForPrompt(episodes []EpisodeItem) string {
	if len(episodes) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【相关近期事件与记忆】:\n")
	for _, ep := range episodes {
		t := time.Unix(ep.CreatedAt, 0).Format("01月02日")
		tagStr := ""
		if len(ep.Tags) > 0 {
			tagStr = fmt.Sprintf("[%s] ", strings.Join(ep.Tags, ", "))
		}
		sb.WriteString(fmt.Sprintf("- %s%s: %s\n", tagStr, t, ep.Content))
	}
	return sb.String()
}
