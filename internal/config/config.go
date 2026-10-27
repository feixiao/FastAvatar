package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

type InteractionMode string

const (
	ModeHandsFree InteractionMode = "hands_free" // 免唤醒连续畅聊
	ModePTT       InteractionMode = "ptt"        // 快捷键按住说话/切换
)

type LLMConfig struct {
	Engine      string  `json:"engine"`       // "local_llama", "ollama", "openai_compatible"
	BaseURL     string  `json:"base_url"`     // 本地或云端 Base URL
	APIKey      string  `json:"api_key"`      // 云端 API Key (可选)
	Model       string  `json:"model"`        // 模型名称 (例如 qwen2.5-1.5b-instruct)
	Temperature float64 `json:"temperature"`  // 温度
	MaxTokens   int     `json:"max_tokens"`   // 最大生成 Token
}

type TTSConfig struct {
	Engine   string `json:"engine"`    // "mac_say", "edge_tts"
	Voice    string `json:"voice"`     // 例如 "Ting-Ting" 或 "zh-CN-XiaoxiaoNeural"
	Rate     int    `json:"rate"`      // 语速百分比调节
}
type ASRConfig struct {
	Engine string `json:"engine"` // "native_mac", "mock"
	Locale string `json:"locale"` // "zh-CN", "en-US"
}


type AppConfig struct {
	DisplayForm  string          `json:"display_form"` // "pet" (桌面宠物) 或 "capsule" (极简胶囊)
	PetSkin      string          `json:"pet_skin"`     // "cat", "fox", "spirit" 或自定义皮肤 ID
	Mode         InteractionMode `json:"mode"`
	AlwaysOnTop  bool            `json:"always_on_top"`
	Hotkey       string          `json:"hotkey"` // 例如 "Option+Space"
	LLM          LLMConfig       `json:"llm"`
	TTS          TTSConfig       `json:"tts"`
	ASR          ASRConfig       `json:"asr"`
	LocalGGUF    string          `json:"local_gguf"`    // 本地 GGUF 模型路径
	LlamaServer  string          `json:"llama_server"`  // llama-server 路径 (空则自动探测或下载)
	SidecarPort  int             `json:"sidecar_port"`  // 本地伴生服务端口 (默认 50088)
}

var (
	defaultConfig = AppConfig{
		DisplayForm: "pet",
		PetSkin:     "cat",
		Mode:        ModeHandsFree,
		AlwaysOnTop: true,
		Hotkey:      "Option+Space",
		LLM: LLMConfig{
			Engine:      "local_llama",
			BaseURL:     "http://127.0.0.1:50088/v1",
			Model:       "qwen2.5-1.5b-instruct",
			Temperature: 0.7,
			MaxTokens:   512,
		},
		TTS: TTSConfig{
			Engine: "mac_say",
			Voice:  "Ting-Ting",
			Rate:   180,
		},
		LocalGGUF:   "models/qwen2.5-1.5b-instruct-q4.gguf",
		ASR: ASRConfig{
			Engine: "native_mac",
			Locale: "zh-CN",
		},
		LlamaServer: "bin/llama-server",
		SidecarPort: 50088,
	}
	cfgLock sync.RWMutex
	current AppConfig
)

func init() {
	current = defaultConfig
}

func ConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "avatar_config.json"
	}
	dir := filepath.Join(home, ".avatar")
	_ = os.MkdirAll(dir, 0755)
	return filepath.Join(dir, "config.json")
}

func LoadConfig() AppConfig {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	p := ConfigPath()
	data, err := os.ReadFile(p)
	if err != nil {
		current = defaultConfig
		return current
	}

	var loaded AppConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		current = defaultConfig
		return current
	}
	if loaded.DisplayForm == "" {
		loaded.DisplayForm = "pet"
	}
	if loaded.PetSkin == "" {
		loaded.PetSkin = "cat"
	}
	if loaded.ASR.Engine == "" {
		loaded.ASR.Engine = "native_mac"
	}
	if loaded.ASR.Locale == "" {
		loaded.ASR.Locale = "zh-CN"
	}
	current = loaded
	return current
}

func SaveConfig(cfg AppConfig) error {
	cfgLock.Lock()
	defer cfgLock.Unlock()

	current = cfg
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath(), data, 0644)
}

func GetConfig() AppConfig {
	cfgLock.RLock()
	defer cfgLock.RUnlock()
	return current
}
