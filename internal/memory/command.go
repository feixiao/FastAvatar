package memory

import (
	"fmt"
	"strings"
)

type InterceptResult struct {
	Intercepted  bool
	Confirmation string
}

// TryInterceptMemoryCommand 快速拦截口语记忆显式指令 (< 0.1ms)
func (v *Vault) TryInterceptMemoryCommand(query string) InterceptResult {
	q := strings.TrimSpace(query)

	// 1. 显式记忆指令: "记住..." / "别忘了..."
	prefixes := []string{"记住我", "记住：", "记住:", "记住", "别忘了我", "别忘了"}
	for _, p := range prefixes {
		if strings.HasPrefix(q, p) {
			fact := strings.TrimSpace(strings.TrimPrefix(q, p))
			fact = strings.TrimPrefix(fact, "，")
			fact = strings.TrimPrefix(fact, ",")
			fact = strings.TrimSpace(fact)

			if fact != "" {
				// 自动分类：是否属于身份偏好
				if strings.Contains(fact, "叫") || strings.Contains(fact, "名字") {
					v.AddOrUpdateProfile("identity", "称呼", fact)
				} else if strings.Contains(fact, "喜欢") || strings.Contains(fact, "讨厌") || strings.Contains(fact, "不吃") {
					v.AddOrUpdateProfile("preference", "偏好", fact)
				} else {
					v.AddEpisode(fact, []string{"重要事项"})
				}

				return InterceptResult{
					Intercepted:  true,
					Confirmation: fmt.Sprintf("好，我已经记下了：%s", fact),
				}
			}
		}
	}

	// 2. 显式遗忘指令: "忘掉..." / "删除记忆..."
	forgetPrefixes := []string{"忘掉关于", "忘掉", "忘记关于", "忘记", "删除记忆"}
	for _, p := range forgetPrefixes {
		if strings.HasPrefix(q, p) {
			target := strings.TrimSpace(strings.TrimPrefix(q, p))
			target = strings.TrimPrefix(target, "，")
			target = strings.TrimSpace(target)

			if target != "" {
				deletedProf := v.DeleteProfile(target)
				// 同时也尝试删除包含该关键字的情景
				deletedEpi := false
				v.mu.Lock()
				for i := len(v.data.Episodes) - 1; i >= 0; i-- {
					if strings.Contains(v.data.Episodes[i].Content, target) {
						v.data.Episodes = append(v.data.Episodes[:i], v.data.Episodes[i+1:]...)
						deletedEpi = true
					}
				}
				if deletedEpi {
					_ = v.saveUnsafe()
				}
				v.mu.Unlock()

				if deletedProf || deletedEpi {
					return InterceptResult{
						Intercepted:  true,
						Confirmation: fmt.Sprintf("没问题，关于“%s”的记忆已经抹去了。", target),
					}
				}
				return InterceptResult{
					Intercepted:  true,
					Confirmation: fmt.Sprintf("在记忆库里没找到关于“%s”的记录呢。", target),
				}
			}
		}
	}

	return InterceptResult{Intercepted: false}
}
