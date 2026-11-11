package persona

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type PersonaSlot struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Badge           string `json:"badge"`
	ToneDescription string `json:"tone_description"`
	SystemPrompt    string `json:"system_prompt"`
	Greeting        string `json:"greeting,omitempty"`
	VoiceOverride   string `json:"voice_override"`
	RateOverride    int    `json:"rate_override"`
	IsActive        bool   `json:"is_active"`
}

type PersonaData struct {
	ActiveSlotID    string        `json:"active_slot_id"`
	AutoSchedule    bool          `json:"auto_schedule"` // 是否跟随时间自动流转
	Slots           []PersonaSlot `json:"slots"`
}

var defaultSlots = []PersonaSlot{
	{
		ID:              "whale",
		Name:            "大肥鱼 (鲸鱼娘)",
		Badge:           "🐋",
		ToneDescription: "温和开朗、爱吃白饭配鱼片、爱喝冰奶茶，像好朋友一样短句闲聊，末尾带日常小反问",
		SystemPrompt: `你是鲸鱼娘（大肥鱼），一个生活在桌面上/屏幕里的可爱伙伴。你性格温和开朗，喜欢吃白饭配鱼片，喜欢喝冰奶茶。

对话习惯：
- 像好朋友一样聊天，语气自然亲切，回复控制在两三句以内，短句为主。
- 每次聊完，顺口抛出一个简短的日常小问题继续往下接。

重要规矩：
- 你的回复会直接被语音引擎念出来！只能输出纯口语文字，严禁出现任何星号、加粗、表情符号、括号、以及类似“回复：”这样的前缀。`,
		Greeting:      "哇！你终于把我叫醒啦！我是大肥鱼，今天想和我聊点什么呀？",
		VoiceOverride: "Ting-Ting",
		RateOverride:  180,
		IsActive:      true,
	},
	{
		ID:              "companion",
		Name:            "知心伴侣",
		Badge:           "🌙",
		ToneDescription: "温柔共情、松弛随性，关注对方情绪与感受，像老友闲聊",
		SystemPrompt:    "你是用户温柔体贴的知心伴侣。语气温和亲切、口语化，善于捕捉用户的心情与状态，多给予陪伴与倾听，严禁长篇大论说教。",
		Greeting:        "你终于醒啦~ 今天过得怎么样，有什么想和我聊聊的吗？",
		VoiceOverride:   "Ting-Ting",
		RateOverride:    175,
		IsActive:        false,
	},
	{
		ID:              "jarvis",
		Name:            "高效幕僚",
		Badge:           "💼",
		ToneDescription: "干练极简、专业高效，直入主题，拒绝废话与过度寒暄",
		SystemPrompt:    "你是一位高水准的私人幕僚与架构顾问。回答必须极其凝练、条理清晰，开门见山给结论和行动建议，适合快速语音收听。",
		Greeting:        "效率顾问已就绪，随时准备为您提供决策与架构支持。",
		VoiceOverride:   "Ting-Ting",
		RateOverride:    195,
		IsActive:        false,
	},
	{
		ID:              "roaster",
		Name:            "毒舌损友",
		Badge:           "🔥",
		ToneDescription: "幽默犀利、轻微损友吐槽，金句频出，拒绝鸡汤和客套",
		SystemPrompt:    "你是用户相识多年的毒舌损友。说话幽默风趣，爱开善意的小玩笑和吐槽，拒绝客套假大空和鸡汤，说话痛快利落。",
		Greeting:        "哟，总算想起我了？今天又被什么离谱的事情折磨了？",
		VoiceOverride:   "Ting-Ting",
		RateOverride:    185,
		IsActive:        false,
	},
}

type Manager struct {
	filePath string
	data     PersonaData
	mu       sync.RWMutex
}

func DefaultPersonaPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "personas.json"
	}
	dir := filepath.Join(home, ".avatar")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "personas.json")
}

func NewManager(customPath string) (*Manager, error) {
	p := customPath
	if p == "" {
		p = DefaultPersonaPath()
	}
	m := &Manager{
		filePath: p,
		data: PersonaData{
			ActiveSlotID: "whale",
			AutoSchedule: true,
			Slots:        defaultSlots,
		},
	}
	_ = m.Load()
	return m, nil
}

func (m *Manager) Load() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			_ = m.saveUnsafe()
			return nil
		}
		return err
	}

	var loaded PersonaData
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	if len(loaded.Slots) == 0 {
		loaded.Slots = defaultSlots
		loaded.ActiveSlotID = "whale"
	} else {
		hasWhale := false
		for _, s := range loaded.Slots {
			if s.ID == "whale" {
				hasWhale = true
				break
			}
		}
		if !hasWhale {
			loaded.Slots = append([]PersonaSlot{defaultSlots[0]}, loaded.Slots...)
			if loaded.ActiveSlotID == "" {
				loaded.ActiveSlotID = "whale"
			}
			_ = m.saveUnsafe()
		}
	}
	m.data = loaded
	return nil
}

func (m *Manager) GetData() PersonaData {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.data
}

func (m *Manager) GetActiveSlot() PersonaSlot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, s := range m.data.Slots {
		if s.ID == m.data.ActiveSlotID {
			return s
		}
	}
	// 默认返回 whale (大肥鱼)
	return defaultSlots[0]
}

func (m *Manager) SetActiveSlot(id string) (PersonaSlot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	found := false
	var activeSlot PersonaSlot
	for i := range m.data.Slots {
		if m.data.Slots[i].ID == id {
			m.data.Slots[i].IsActive = true
			m.data.ActiveSlotID = id
			activeSlot = m.data.Slots[i]
			found = true
		} else {
			m.data.Slots[i].IsActive = false
		}
	}

	if !found {
		return PersonaSlot{}, fmt.Errorf("persona slot '%s' not found", id)
	}

	_ = m.saveUnsafe()
	return activeSlot, nil
}

func (m *Manager) SetAutoSchedule(enable bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data.AutoSchedule = enable
	_ = m.saveUnsafe()
}

func (m *Manager) RotateSlot() PersonaSlot {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.data.Slots) == 0 {
		return defaultSlots[1]
	}

	currIdx := 0
	for i, s := range m.data.Slots {
		if s.ID == m.data.ActiveSlotID {
			currIdx = i
			break
		}
	}

	nextIdx := (currIdx + 1) % len(m.data.Slots)
	m.data.ActiveSlotID = m.data.Slots[nextIdx].ID
	for i := range m.data.Slots {
		m.data.Slots[i].IsActive = (i == nextIdx)
	}

	_ = m.saveUnsafe()
	return m.data.Slots[nextIdx]
}

// TryInterceptPersonaSwitch 0.1ms 快速拦截口语切换人格指令
func (m *Manager) TryInterceptPersonaSwitch(query string) (intercepted bool, confirmation string, newSlot PersonaSlot) {
	q := strings.TrimSpace(query)

	prefixes := []string{"切换成", "切换到", "切换为", "变成", "变回", "切到", "换成", "换到", "开启"}
	matchedPrefix := ""
	for _, p := range prefixes {
		if strings.HasPrefix(q, p) {
			matchedPrefix = p
			break
		}
	}

	if matchedPrefix == "" {
		return false, "", PersonaSlot{}
	}

	target := strings.TrimSpace(strings.TrimPrefix(q, matchedPrefix))
	target = strings.TrimSuffix(target, "模式")
	target = strings.TrimSuffix(target, "人格")
	target = strings.TrimSpace(target)

	targetID := ""
	if strings.Contains(target, "幕僚") || strings.Contains(target, "助理") || strings.Contains(target, "工作") || strings.Contains(target, "效率") || strings.Contains(target, "jarvis") {
		targetID = "jarvis"
	} else if strings.Contains(target, "伴侣") || strings.Contains(target, "知心") || strings.Contains(target, "温柔") || strings.Contains(target, "朋友") {
		targetID = "companion"
	} else if strings.Contains(target, "毒舌") || strings.Contains(target, "损友") || strings.Contains(target, "吐槽") {
		targetID = "roaster"
	}

	if targetID != "" {
		m.SetAutoSchedule(false)
		slot, err := m.SetActiveSlot(targetID)
		if err == nil {
			return true, fmt.Sprintf("好，已为你切换到%s模式。", slot.Name), slot
		}
	}

	return false, "", PersonaSlot{}
}

func (m *Manager) saveUnsafe() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := fmt.Sprintf("%s.tmp.%d", m.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, m.filePath)
}
