package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type ProfileItem struct {
	ID        string `json:"id"`
	Category  string `json:"category"` // identity, preference, habit, work
	Key       string `json:"key"`
	Value     string `json:"value"`
	UpdatedAt int64  `json:"updated_at"`
}

type EpisodeItem struct {
	ID        string   `json:"id"`
	Tags      []string `json:"tags"`
	Content   string   `json:"content"`
	CreatedAt int64    `json:"created_at"`
}

type MemoryData struct {
	Profile  []ProfileItem `json:"profile"`
	Episodes []EpisodeItem `json:"episodes"`
}

type Vault struct {
	filePath string
	data     MemoryData
	mu       sync.RWMutex
}

func DefaultVaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "memory.json"
	}
	dir := filepath.Join(home, ".avatar")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "memory.json")
}

func NewVault(customPath string) (*Vault, error) {
	p := customPath
	if p == "" {
		p = DefaultVaultPath()
	}
	v := &Vault{
		filePath: p,
		data: MemoryData{
			Profile:  make([]ProfileItem, 0),
			Episodes: make([]EpisodeItem, 0),
		},
	}
	_ = v.Load()
	return v, nil
}

func (v *Vault) Load() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	data, err := os.ReadFile(v.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var loaded MemoryData
	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}
	if loaded.Profile == nil {
		loaded.Profile = make([]ProfileItem, 0)
	}
	if loaded.Episodes == nil {
		loaded.Episodes = make([]EpisodeItem, 0)
	}
	v.data = loaded
	return nil
}

func (v *Vault) Save() error {
	v.mu.RLock()
	data, err := json.MarshalIndent(v.data, "", "  ")
	v.mu.RUnlock()
	if err != nil {
		return err
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", v.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, v.filePath)
}

func (v *Vault) AddOrUpdateProfile(category, key, value string) ProfileItem {
	v.mu.Lock()
	defer v.mu.Unlock()

	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	now := time.Now().Unix()

	for i, item := range v.data.Profile {
		if strings.EqualFold(item.Key, key) {
			v.data.Profile[i].Value = value
			v.data.Profile[i].Category = category
			v.data.Profile[i].UpdatedAt = now
			_ = v.saveUnsafe()
			return v.data.Profile[i]
		}
	}

	newItem := ProfileItem{
		ID:        fmt.Sprintf("prof_%d", time.Now().UnixNano()),
		Category:  category,
		Key:       key,
		Value:     value,
		UpdatedAt: now,
	}
	v.data.Profile = append(v.data.Profile, newItem)
	_ = v.saveUnsafe()
	return newItem
}

func (v *Vault) DeleteProfile(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	idx := -1
	for i, item := range v.data.Profile {
		if item.ID == id || strings.EqualFold(item.Key, id) || strings.Contains(item.Value, id) {
			idx = i
			break
		}
	}
	if idx >= 0 {
		v.data.Profile = append(v.data.Profile[:idx], v.data.Profile[idx+1:]...)
		_ = v.saveUnsafe()
		return true
	}
	return false
}

func (v *Vault) AddEpisode(content string, tags []string) EpisodeItem {
	v.mu.Lock()
	defer v.mu.Unlock()

	content = strings.TrimSpace(content)
	now := time.Now().Unix()

	cleanTags := make([]string, 0, len(tags))
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t != "" {
			cleanTags = append(cleanTags, t)
		}
	}

	newItem := EpisodeItem{
		ID:        fmt.Sprintf("epi_%d", time.Now().UnixNano()),
		Tags:      cleanTags,
		Content:   content,
		CreatedAt: now,
	}
	v.data.Episodes = append(v.data.Episodes, newItem)
	_ = v.saveUnsafe()
	return newItem
}

func (v *Vault) DeleteEpisode(id string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	idx := -1
	for i, item := range v.data.Episodes {
		if item.ID == id {
			idx = i
			break
		}
	}
	if idx >= 0 {
		v.data.Episodes = append(v.data.Episodes[:idx], v.data.Episodes[idx+1:]...)
		_ = v.saveUnsafe()
		return true
	}
	return false
}

func (v *Vault) GetData() MemoryData {
	v.mu.RLock()
	defer v.mu.RUnlock()

	prof := make([]ProfileItem, len(v.data.Profile))
	copy(prof, v.data.Profile)
	epi := make([]EpisodeItem, len(v.data.Episodes))
	copy(epi, v.data.Episodes)

	return MemoryData{
		Profile:  prof,
		Episodes: epi,
	}
}

func (v *Vault) FormatProfileForPrompt() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if len(v.data.Profile) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("【用户核心画像】:\n")
	for _, item := range v.data.Profile {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", item.Key, item.Value))
	}
	return sb.String()
}

func (v *Vault) saveUnsafe() error {
	data, err := json.MarshalIndent(v.data, "", "  ")
	if err != nil {
		return err
	}
	tmpFile := fmt.Sprintf("%s.tmp.%d", v.filePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, v.filePath)
}
