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

type ModelEndpointConfig struct {
	Engine      string  `json:"engine,omitempty"`       // "openai_compatible", "ollama", "lmstudio"
	BaseURL     string  `json:"base_url"`               // 本地或云端 Base URL
	APIKey      string  `json:"api_key,omitempty"`      // API Key (可选)
	Model       string  `json:"model"`                  // 模型标识符
	Temperature float64 `json:"temperature,omitempty"`  // 温度
	MaxTokens   int     `json:"max_tokens,omitempty"`   // 最大生成 Token
}

type LLMConfig struct {
	Engine      string  `json:"engine"`       // "local_llama", "ollama", "openai_compatible"
	BaseURL     string  `json:"base_url"`     // 本地或云端 Base URL
	APIKey      string  `json:"api_key"`      // 云端 API Key (可选)
	Model       string  `json:"model"`        // 模型名称 (例如 qwen2.5-1.5b-instruct)
	Temperature float64 `json:"temperature"`  // 温度
	MaxTokens   int     `json:"max_tokens"`   // 最大生成 Token

	Edge  ModelEndpointConfig `json:"edge"`  // 本地端侧小模型 (SLM, 如 0.5B/1.5B)
}

func (c *LLMConfig) GetEdgeEndpoint() ModelEndpointConfig {
	edge := c.Edge
	if edge.BaseURL == "" {
		edge.BaseURL = c.BaseURL
	}
	if edge.Model == "" {
		edge.Model = "qwen2.5-0.5b-instruct"
	}
	if edge.Temperature == 0 {
		edge.Temperature = 0.7
	}
	if edge.MaxTokens == 0 {
		if c.MaxTokens > 0 {
			edge.MaxTokens = c.MaxTokens
		} else {
			edge.MaxTokens = 1024
		}
	}
	return edge
}


type TTSConfig struct {
	Engine   string `json:"engine"`              // "sherpa_onnx", "mac_say"
	Voice    string `json:"voice"`               // 发音人 ID (如 "66" 或别名 "whale")
	Rate     int    `json:"rate"`                // 语速百分比调节
	BinPath  string `json:"bin_path,omitempty"`  // sherpa-onnx-offline-tts-play 路径
	ModelDir string `json:"model_dir,omitempty"` // VITS 模型目录
}
type ASRConfig struct {
	Engine string `json:"engine"` // "native_mac", "mock"
	Locale string `json:"locale"` // "zh-CN", "en-US"
}


type AppConfig struct {
	DisplayForm    string          `json:"display_form"`     // "pet" (桌面宠物) 或 "capsule" (极简胶囊)
	PetSkin        string          `json:"pet_skin"`         // "luoxiaohei2d", "whale", "cat", "fox", "spirit" 或自定义皮肤 ID
	AutoRoaming    bool            `json:"auto_roaming"`     // 桌面自主自由漫步开关 (默认开启)
	RoamSpeed      float64         `json:"roam_speed"`       // 漫游巡视速度倍率 (默认 1.2)
	PetScale       float64         `json:"pet_scale"`        // 桌宠渲染缩放倍率 (默认 1.15)
	Mode           InteractionMode `json:"mode"`
	AlwaysOnTop    bool            `json:"always_on_top"`
	Hotkey         string          `json:"hotkey"`           // 例如 "Option+Space"
	CloudServerURL string          `json:"cloud_server_url"` // 云端实时网关服务地址 (默认 http://127.0.0.1:8088)
	CloudConsent   bool            `json:"cloud_consent"`    // 联网与云端认知大模型授权开关 (true: 允许联网; false: 纯离线隐私模式)
	LLM            LLMConfig       `json:"llm"`              // 端侧小模型 (SLM) 配置
	TTS            TTSConfig       `json:"tts"`
	ASR            ASRConfig       `json:"asr"`
	LocalGGUF      string          `json:"local_gguf"`       // 本地 GGUF 模型路径
	LlamaServer    string          `json:"llama_server"`     // llama-server 路径 (空则自动探测或下载)
	SidecarPort    int             `json:"sidecar_port"`     // 本地伴生服务端口 (默认 50088)
}

var (
	defaultConfig = AppConfig{
		DisplayForm:    "pet",
		PetSkin:        "luoxiaohei2d",
		AutoRoaming:    true,
		RoamSpeed:      1.2,
		PetScale:       1.15,
		Mode:           ModeHandsFree,
		AlwaysOnTop:    true,
		Hotkey:         "Option+Space",
		CloudServerURL: "http://127.0.0.1:8088",
		CloudConsent:   false, // 默认纯离线隐私模式，不主动发网络包
		LLM: LLMConfig{
			Engine:      "openai_compatible",
			BaseURL:     "http://127.0.0.1:1234/v1",
			Model:       "qwen2.5-0.5b-instruct",
			Temperature: 0.7,
			MaxTokens:   512,
			Edge: ModelEndpointConfig{
				Engine:      "openai_compatible",
				BaseURL:     "http://127.0.0.1:1234/v1",
				Model:       "qwen2.5-0.5b-instruct",
				Temperature: 0.7,
				MaxTokens:   256,
			},
		},
		TTS: TTSConfig{
			Engine:   "sherpa_onnx",
			Voice:    "66",
			Rate:     180,
			BinPath:  "bin/sherpa-onnx-offline-tts-play",
			ModelDir: "models/vits-zh-aishell3",
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
	if custom := os.Getenv("AVATAR_CONFIG_PATH"); custom != "" {
		return custom
	}
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
	if loaded.PetSkin == "" || loaded.PetSkin == "whale" {
		loaded.PetSkin = "luoxiaohei2d"
	}
	if loaded.RoamSpeed == 0 {
		loaded.RoamSpeed = 1.2
	}
	if loaded.PetScale == 0 {
		loaded.PetScale = 1.15
	}
	if loaded.Mode == "" {
		loaded.Mode = ModeHandsFree
	}
	if loaded.ASR.Engine == "" {
		loaded.ASR.Engine = "native_mac"
	}
	if loaded.ASR.Locale == "" {
		loaded.ASR.Locale = "zh-CN"
	}
	if loaded.LLM.Edge.BaseURL == "" {
		loaded.LLM.Edge = loaded.LLM.GetEdgeEndpoint()
	}
	if loaded.CloudServerURL == "" {
		loaded.CloudServerURL = "http://127.0.0.1:8088"
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
