# Avatar — 端云协同全双工智能语音伴侣

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Wails](https://img.shields.io/badge/Wails-v2-DF1A2A?style=flat)](https://wails.io)
[![Architecture](https://img.shields.io/badge/Architecture-Unified%20Edge%20TTS-007ACC?style=flat)](#)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Android-green?style=flat)](#)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Avatar 是一套**端云协同、双环驱动（Dual-Loop）、全链路文本流（SSE）、端侧统一发声（Unified Edge TTS）、毫秒级可打断（Barge-in）**的跨平台智能语音伴侣系统。包含**轻量桌面端（Go + Wails + Swift 原生 ASR，Mac 优先）**、**移动端（Pixel 原生 Android）**以及**极简云端实时网关服务（Go Realtime Gateway :8088）**。

---

## 🌟 核心特性

- ⚡️ **端云双环 + 全链路传文本架构 (Dual-Loop & Text Streaming)**：
  - **全链路传文本**：云端只负责认知推理，通过 HTTP SSE（Server-Sent Events）将大模型生成的 Token 文本流以最快速度推给端侧，零音视频流带宽与服务器成本开销。
  - **端侧极速快环（≤ 150ms）**：毫秒级意图短路拦截器、本地记忆底座、多模态人格引擎与离线伴生小模型（SLM），断网物理隔离依然可用。
  - **云端深度慢环（P50 ≤ 800ms）**：基于 Go 实时网关，支持流式 SSE 极速吐字、多轮深度推理与首句微切（4~6 字）。
- 🎙️ **端侧统一语音合成 (Unified Edge TTS · 零音色割裂)**：
  - 无论是端侧快环 A 还是云端慢环 B，最终文字全部汇入**端侧同一个高质量离线 TTS 引擎发声**，彻底根除“本地机械音 vs 云端真人音”的人格割裂，音色与语速 100% 绝对一致。
- ⚡️ **毫秒级全双工打断 (Barge-in ≤ 80ms)**：
  - 用户开嗓瞬间在端侧清空待播切句并掐断扬声器，同时向云端下发中断信令立即强杀大模型生成协程（CancelFunc）。
- 🛡️ **严格隐私合规 (Zero-Retention Guardrail)**：
  - 原始音频流绝对不出端，未经用户显式授权严禁文本上云；云端零留存，严禁在服务端持久化任何用户转写内容。
- 🖥️ **跨平台原生体验 (macOS 优先)**：
  - **macOS**：无 Python 依赖纯 Go 单二进制，常驻内存仅 ~25-35MB，极简透明悬浮胶囊，支持硬件级回声消除（AEC）与动态声波呼吸。
  - **Android**：针对 Google Pixel 10 定制的 Material 3 + 11 状态严密状态机前台服务。

> 📖 **关于完整的架构拓扑与协议细节，请查阅 [系统工程架构设计文档 (docs/architecture.md)](docs/architecture.md)。**  
> 🐋 **关于桌面宠物互动、干饭彩蛋与全双工可打断对聊，请查阅 [Desktop Pet 桌面宠物与语音互动指南 (docs/desktop-pet.md)](docs/desktop-pet.md)。**

---

## 🚀 快速开始

### 1. 统一启动云端后台服务 (Docker Compose)

```bash
# 复制环境变量配置文件（可选，默认自动接入本地 Ollama，或配置云端商业大模型）
cp server/.env.example server/.env

# 一键启动后台网关容器 (Go 实时网关 :8088)
make server

# 查看后台服务运行日志
make server-logs

# 停止后台服务集群
make server-stop
```

### 2. 启动桌面端应用 (macOS)

```bash
# 本地调试运行（自动编译 Swift 原生 ASR 并在本地启动）
make run

# 或直接打包为 macOS 原生 .app 并启动
make start
```

### 3. 运行端云协同端到端集成测试

```bash
# 一键验证「网关启动 ➔ Token 获取 ➔ 意图路由 ➔ 端侧打断 ➔ 云端截断」全链路贯通
make test-e2e
```

---

## 🛠️ Makefile 常用指令

| 指令 | 说明 |
| :--- | :--- |
| **`make server`** | 统一通过 Docker Compose 后台启动云端轻量网关服务（Go 网关 :8088） |
| **`make server-stop`** | 停止后台容器服务 |
| **`make server-logs`** | 查看后台容器实时运行日志 |
| `make server-dev` | 本地宿主机直接秒级运行 Go 实时网关（零 Docker 依赖，开发调试推荐） |
| **`make test-e2e`** | 运行端云协同端到端对接与毫秒级协同打断集成测试 |
| `make start` | 一键打包并直接启动 macOS 原生 App |
| `make package` | 编译并打包为 macOS 原生应用（生成 `build/bin/Avatar.app`） |
| `make build` | 仅编译桌面端可执行二进制文件 |
| `make asr` | 编译 macOS 原生离线语音识别组件（`bin/avatar-asr`） |
| `make test` | 运行项目所有 Go 单元测试 |
| `make android-test` | 运行 Android 原生客户端单元测试 |
| `make android-build` (或 `make apk`) | 构建 Android 原生客户端 Debug APK |
| `make android-install` (或 `make install-apk` / `make install`) | 构建并安装 Android Debug APK 到已连接的设备或模拟器 |
| `make clean` | 清理构建产物与临时文件 |

---

## 📂 项目结构

```
├── server/                 # 【云端服务】轻量实时网关 (Go) 容器与信令编排
├── internal/               # 【核心内部模块】
│   ├── cloud/              # 端侧与云端服务的长连接协同客户端 (Token / 打断 / 静音)
│   ├── dispatcher/         # 端侧意图分流路由器 (本地控制 vs 云端慢环决策)
│   ├── engine/             # 桌面端状态机与会话调度循环
│   ├── splitter/           # 流式文本断句器 (首句微切低延时播报)
│   ├── asr/                # 语音识别抽象层
│   └── tts/                # 语音合成抽象层 (macOS say / 云端 TTS)
├── cmd/                    # 【可执行工具与测试套件】
│   ├── avatar-asr/         # macOS 原生高性能 Swift 离线语音识别 CLI
│   └── e2e-test/           # 端云协同端到端集成与打断贯通验收测试
├── android/                # 【移动端】Google Pixel 10 原生 Android 客户端
├── frontend/               # 【桌面端前端】透明动态岛声波胶囊 (HTML + CSS + Web Audio)
├── docs/                   # 【系统设计与规范】
│   └── architecture.md     # 🌟 系统全景架构设计文档 (时序图/状态机/接口契约)
├── app.go / main.go        # Wails 应用绑定胶水层与桌面入口
└── Makefile                # 统一构建、测试与运维指令
```

---

## ⚙️ 配置说明

桌面端首次启动会自动在用户目录下生成配置文件：`~/.avatar/config.json`：
- **端侧小模型 (Local SLM)**：客户端直连本地轻量模型（Ollama / LM Studio），配置 `llm.base_url`（如 `http://127.0.0.1:11434/v1`）与 `model`（如 `qwen2.5-0.5b-instruct`），负责极速闲聊、日常问候与本地情绪安抚；
- **云端实时网关 (`cloud_server_url`)**：配置指向自建云网关实例（默认 `http://127.0.0.1:8088`）。客户端严禁直连外部大模型及携带厂商 API Key，所有复杂认知推理统一由网关代理；
- **联网大模型授权 (`cloud_consent`)**：客户端联网开关（默认 `false` 纯离线隐私保护）。开启时允许复杂问题上云网关，关闭时仅在端侧响应小模型与本地规则记忆。
---

## 📚 详细设计与技术文档

所有底层技术设计、协议定义与工程规范请查阅 `docs/` 目录：
- 🌟 **[系统工程架构文档 (System Architecture)](docs/architecture.md)**：双环架构、客户端分层、云端基础设施、时序图、状态机、REST API 契约与安全机制。
- 📱 **[Android 移动端开发指南](android/README.md)**：Pixel 10 客户端构建、测试与前台服务配置。
- 🐋 **[Desktop Pet 桌面宠物与语音互动指南 (docs/desktop-pet.md)](docs/desktop-pet.md)**：macOS 桌面宠物交互规范、干饭动效、嘴型闭环、大肥鱼及全桌宠自由替换说明。
