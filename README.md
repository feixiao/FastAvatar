# Avatar — 跨平台桌面端极简语音伴侣 (Go + Wails)

本项目是一套**单二进制开箱即用、低时延流式切句、全双工可打断（Barge-in）**的跨平台桌面端极简语音伴侣。

---

## 一、 架构与定位

### 1.1 核心特性
1. **纯 Go 驱动原生桌面应用**：
   - 彻底移除 Python 运行时依赖，单二进制（仅 ~6.5MB），开机极速自启，常驻内存仅 ~25-35MB。
2. **极简动态悬浮胶囊 (Dynamic Island Pill)**：
   - 屏幕置顶、无边框透明窗口，支持任意位置拖拽。
   - 4 种视觉与交互状态流转：`待命 🎙️` ➔ `倾听 👂` ➔ `思考 💭` ➔ `说话 🔊`。
3. **免唤醒外放 + 硬件级回声消除 (AEC)**：
   - 采用系统级 Web Audio 硬件回声消除，扬声器外放时麦克风不会收录自身发声。
   - 用户说话即刻触发毫秒级 **Barge-in 打断**，伴侣立即闭嘴切回倾听。
4. **本地端侧大模型优先 (SLM-First)**：
   - 优先通过后台无头伴生进程直连本地端侧小模型（如 Qwen2.5-1.5B/3B GGUF）或本地 Ollama。
   - 本地未启动时平滑静默降级为云端兼容 API（如 DeepSeek-V3 / Qwen-Max）。
5. **流式断句 (Sentence Splitter) 与极速发音**：
   - 首句 4~6 字符快速切割立即送入 TTS 合成，首包响应时延 (TTFA) < 500ms。
   - macOS 原生内置高质量语音合成（`say -v Ting-Ting`），零依赖即时播放。

---

## 二、 目录结构

```
Avatar/
├── cmd/
├── internal/
│   ├── config/             # 配置管理 (config.json)
│   ├── engine/             # 核心会话状态机、流式断句器 (splitter.go)
│   ├── llm/                # OpenAI 兼容流式 SSE 客户端与端口健康自检
│   ├── sidecar/            # 本地无头 llama-server 伴生进程守护管理
│   └── tts/                # 语音合成抽象 (mac_say / mock)
├── frontend/
│   └── src/                # 极简无边框透明声波胶囊 (HTML + CSS + Web Audio)
├── build/                  # 打包资源与图标
├── app.go                  # Wails 应用绑定胶水层
├── main.go                 # 桌面应用生命周期入口
├── package_mac.sh          # 一键构建 macOS .app 打包脚本
├── Makefile                # 快捷构建、调试与管理指令
├── go.mod
└── README.md
```

---

## 三、 快速开始

### 3.1 Make 快捷指令

项目提供了标准 `Makefile` 封装日常操作：

| 指令 | 作用 |
| :--- | :--- |
| `make run`（或 `make dev`） | 直接运行应用（开发调试） |
| `make start` | 一键打包并立即启动 macOS 原生 App |
| `make package` | 编译并打包为 macOS 原生应用（生成 `build/bin/Avatar.app`） |
| `make build` | 仅编译可执行二进制文件（`build/bin/Avatar`） |
| `make test` | 运行所有单元测试 |
| `make clean` | 清理构建产物及临时文件 |
| `make help` | 显示 Makefile 指令说明 |

### 3.2 极速启动与打包

```bash
# 开发调试：直接启动胶囊
make run

# 生产打包并直接启动应用
make start

# 运行单元测试
make test
```

> 亦可手动执行 `./package_mac.sh` 进行打包，产物位于 `build/bin/Avatar.app`。

### 3.3 大模型与接口配置
应用首次运行会自动生成配置文件：`~/.avatar/config.json`。可根据需要配置本地或云端大模型：
- **本地 Ollama**：将 `llm.base_url` 设为 `http://127.0.0.1:11434/v1`，`model` 设为对应模型名称（如 `qwen2.5:1.5b`）。
- **云端 API（DeepSeek / 兼容 OpenAI）**：配置 `base_url`、`api_key` 及 `model` 即可。
- **语音合成（TTS）**：macOS 原生内置免依赖合成（默认 `Ting-Ting` 音色），开箱即用。

### 3.4 交互说明
- **免唤醒畅聊**：直接对电脑说话，伴侣自动感知并流式语音回复；在伴侣说话期间随时开口即可插话打断。
- **模式切换**：点击胶囊上的「免唤醒」按钮可切换为「快捷键」模式。
- **文字输入**：点击胶囊上的 `💬` 按钮可弹出极简文字框进行键入提问。
- **一键静音/打断**：点击 `⏹` 或胶囊核心呼吸球，瞬间终止发音并重置状态。
