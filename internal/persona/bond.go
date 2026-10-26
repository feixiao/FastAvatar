package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type BondData struct {
	XP             int    `json:"xp"`
	Level          int    `json:"level"`
	LevelName      string `json:"level_name"`
	StreakDays     int    `json:"streak_days"`
	LastActiveDate string `json:"last_active_date"` // YYYY-MM-DD
}

var levelThresholds = []struct {
	Level int
	Name  string
	MinXP int
}{
	{1, "初识助手", 0},
	{2, "熟络朋友", 100},
	{3, "知心密友", 300},
	{4, "羁绊伴侣", 700},
	{5, "灵魂莫逆", 1500},
}

type BondManager struct {
	filePath string
	data     BondData
	mu       sync.RWMutex
}

func DefaultBondPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "bond.json"
	}
	dir := filepath.Join(home, ".avatar")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "bond.json")
}

func NewBondManager(customPath string) (*BondManager, error) {
	p := customPath
	if p == "" {
		p = DefaultBondPath()
	}
	b := &BondManager{
		filePath: p,
		data: BondData{
			XP:             0,
			Level:          1,
			LevelName:      "初识助手",
			StreakDays:     1,
			LastActiveDate: time.Now().Format("2006-01-02"),
		},
	}
	_ = b.Load()
	return b, nil
}

func (b *BondManager) Load() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	data, err := os.ReadFile(b.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = b.saveUnsafe()
			return nil
		}
		return err
	}

	var loaded BondData
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	if loaded.Level <= 0 {
		loaded.Level = 1
		loaded.LevelName = "初识助手"
	}
	b.data = loaded
	return nil
}

func (b *BondManager) GetData() BondData {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.data
}

// AddInteraction 记录一次交互，增加 XP 并更新连续陪伴天数 (Streak)
func (b *BondManager) AddInteraction() (newLevel int, leveledUp bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	today := time.Now().Format("2006-01-02")
	yesterday := time.Now().AddDate(0, 0, -1).Format("2006-01-02")

	// 连击天数计算
	if b.data.LastActiveDate == yesterday {
		b.data.StreakDays++
	} else if b.data.LastActiveDate != today {
		b.data.StreakDays = 1
	}
	b.data.LastActiveDate = today

	// 计算经验值增量
	gain := 10
	if b.data.StreakDays >= 7 {
		gain = 15
	} else if b.data.StreakDays >= 3 {
		gain = 12
	}
	b.data.XP += gain

	// 计算等级
	oldLevel := b.data.Level
	currLevel := 1
	currName := "初识助手"
	for _, t := range levelThresholds {
		if b.data.XP >= t.MinXP {
			currLevel = t.Level
			currName = t.Name
		}
	}

	b.data.Level = currLevel
	b.data.LevelName = currName

	_ = b.saveUnsafe()
	return currLevel, currLevel > oldLevel
}

func (b *BondManager) FormatBondForPrompt() string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return fmt.Sprintf("【你们的关系羁绊】：当前处于[%s]，已连续相伴 %d 天。随着羁绊加深，请自然展现出彼此熟悉、默契、心有灵犀的态度，无需客套寒暄。",
		b.data.LevelName, b.data.StreakDays)
}

func (b *BondManager) saveUnsafe() error {
	data, err := json.MarshalIndent(b.data, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := fmt.Sprintf("%s.tmp.%d", b.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, b.filePath)
}
