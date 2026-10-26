package memory

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestVaultOperations(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "test_memory.json")

	vault, err := NewVault(p)
	if err != nil {
		t.Fatalf("NewVault failed: %v", err)
	}

	// 1. 测试添加与更新画像
	vault.AddOrUpdateProfile("preference", "咖啡", "美式不加糖")
	data := vault.GetData()
	if len(data.Profile) != 1 || data.Profile[0].Value != "美式不加糖" {
		t.Fatalf("expected 1 profile item, got %v", data.Profile)
	}

	vault.AddOrUpdateProfile("preference", "咖啡", "燕麦拿铁")
	data = vault.GetData()
	if len(data.Profile) != 1 || data.Profile[0].Value != "燕麦拿铁" {
		t.Fatalf("expected updated value '燕麦拿铁', got %v", data.Profile[0].Value)
	}

	// 2. 测试情景事件
	vault.AddEpisode("今天把 Avatar 重构成 Go", []string{"工作", "Go"})
	vault.AddEpisode("下周三架构评审", []string{"日程"})
	data = vault.GetData()
	if len(data.Episodes) != 2 {
		t.Fatalf("expected 2 episodes, got %d", len(data.Episodes))
	}

	// 3. 测试检索召回
	recalled := vault.RetrieveRelevantEpisodes("你的Go重构做得怎么样了？", 2)
	if len(recalled) == 0 || !strings.Contains(recalled[0].Content, "Go") {
		t.Fatalf("expected recalled Go episode, got %v", recalled)
	}

	// 4. 测试删除
	vault.DeleteProfile("咖啡")
	if len(vault.GetData().Profile) != 0 {
		t.Fatalf("expected empty profile after delete")
	}
}

func TestOralMemoryCommands(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "cmd_memory.json")
	vault, _ := NewVault(p)

	// 1. 记住口令拦截
	res := vault.TryInterceptMemoryCommand("记住我叫阿飞")
	if !res.Intercepted || !strings.Contains(res.Confirmation, "阿飞") {
		t.Fatalf("expected intercepted '阿飞', got %v", res)
	}

	prof := vault.GetData().Profile
	if len(prof) != 1 || !strings.Contains(prof[0].Value, "阿飞") {
		t.Fatalf("expected profile to store '阿飞', got %v", prof)
	}

	// 2. 遗忘口令拦截
	resForget := vault.TryInterceptMemoryCommand("忘掉阿飞")
	if !resForget.Intercepted || !strings.Contains(resForget.Confirmation, "抹去") {
		t.Fatalf("expected forget interception, got %v", resForget)
	}

	if len(vault.GetData().Profile) != 0 {
		t.Fatalf("expected profile deleted")
	}

	// 3. 普通对话不拦截
	resNormal := vault.TryInterceptMemoryCommand("今天天气真好呀")
	if resNormal.Intercepted {
		t.Fatalf("expected normal query not to be intercepted")
	}
}

func TestTimeDecayRetrieval(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "decay_memory.json")
	vault, _ := NewVault(p)

	// 添加一个旧事件和一个新事件
	vault.AddEpisode("喝了一杯拿铁", []string{"生活"})
	// 调整旧事件时间戳为 30 天前
	vault.mu.Lock()
	vault.data.Episodes[0].CreatedAt = time.Now().Unix() - 30*86400
	vault.mu.Unlock()

	// 添加新事件
	vault.AddEpisode("今天买了一杯拿铁咖啡", []string{"生活"})

	recalled := vault.RetrieveRelevantEpisodes("拿铁", 1)
	if len(recalled) != 1 || !strings.Contains(recalled[0].Content, "今天买了一杯") {
		t.Fatalf("expected more recent episode to rank higher, got %v", recalled)
	}
}
