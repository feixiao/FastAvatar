package persona

import (
	"path/filepath"
	"strings"
	"testing"

	"avatar/internal/memory"
)

func TestPersonaSlotSwitchAndRotate(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "test_personas.json")

	mgr, err := NewManager(p)
	if err != nil {
		t.Fatalf("NewManager failed: %v", err)
	}

	slot := mgr.GetActiveSlot()
	if slot.ID != "companion" {
		t.Fatalf("expected default companion, got %s", slot.ID)
	}

	// 切换到 jarvis
	slot, err = mgr.SetActiveSlot("jarvis")
	if err != nil || slot.ID != "jarvis" {
		t.Fatalf("expected jarvis slot, got %v", err)
	}

	// 轮转卡槽
	next := mgr.RotateSlot()
	if next.ID == "jarvis" {
		t.Fatalf("expected next slot after jarvis, got %s", next.ID)
	}
}

func TestOralPersonaSwitch(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "oral_personas.json")
	mgr, _ := NewManager(p)

	// 1. 口语切换为幕僚
	intercepted, confirm, newSlot := mgr.TryInterceptPersonaSwitch("切换成高效幕僚模式")
	if !intercepted || newSlot.ID != "jarvis" || !strings.Contains(confirm, "幕僚") {
		t.Fatalf("expected jarvis switch, got %v, %s, %v", intercepted, confirm, newSlot)
	}

	// 2. 口语切换为毒舌
	intercepted, confirm, newSlot = mgr.TryInterceptPersonaSwitch("变成毒舌损友")
	if !intercepted || newSlot.ID != "roaster" {
		t.Fatalf("expected roaster switch, got %v, %v", intercepted, newSlot)
	}

	// 3. 普通对话不拦截
	intercepted, _, _ = mgr.TryInterceptPersonaSwitch("今天的工作好忙啊")
	if intercepted {
		t.Fatalf("expected normal conversation not to be intercepted")
	}
}

func TestBondProgression(t *testing.T) {
	tmpDir := t.TempDir()
	p := filepath.Join(tmpDir, "test_bond.json")
	b, err := NewBondManager(p)
	if err != nil {
		t.Fatalf("NewBondManager failed: %v", err)
	}

	// 初始状态
	data := b.GetData()
	if data.Level != 1 || data.XP != 0 {
		t.Fatalf("unexpected initial bond: %v", data)
	}

	for range 10 {
		b.AddInteraction()
	}

	data = b.GetData()
	if data.XP < 100 || data.Level != 2 {
		t.Fatalf("expected Lv.2 after 100 XP, got level %d, XP %d", data.Level, data.XP)
	}
}

func TestAssembleSystemPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	pMem := filepath.Join(tmpDir, "mem.json")
	pPer := filepath.Join(tmpDir, "per.json")
	pBnd := filepath.Join(tmpDir, "bnd.json")

	vault, _ := memory.NewVault(pMem)
	vault.AddOrUpdateProfile("identity", "称呼", "阿飞")
	vault.AddEpisode("刚刚重构完成了Go伴侣系统", []string{"工作", "Go"})

	personaMgr, _ := NewManager(pPer)
	bondMgr, _ := NewBondManager(pBnd)

	active := personaMgr.GetActiveSlot()
	prompt := AssembleSystemPrompt(active, bondMgr, vault, "Go重构进展如何？")

	if !strings.Contains(prompt, "阿飞") {
		t.Fatalf("expected prompt to contain profile '阿飞'")
	}
	if !strings.Contains(prompt, "重构完成了Go伴侣系统") {
		t.Fatalf("expected prompt to recall episode")
	}
	if !strings.Contains(prompt, "关系羁绊") {
		t.Fatalf("expected prompt to contain bond info")
	}
}
