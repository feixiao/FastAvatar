# Avatar — 端云协同全双工智能语音伴侣

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat&logo=go)](https://golang.org)
[![Wails](https://img.shields.io/badge/Wails-v2-DF1A2A?style=flat)](https://wails.io)
[![LiveKit](https://img.shields.io/badge/LiveKit-SFU-002B49?style=flat)](https://livekit.io)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Android-green?style=flat)](#)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Avatar 是一套**端云协同、双环驱动（Dual-Loop）、低时延流式、全双工可打断（Barge-in）**的跨平台智能语音伴侣系统。包含**轻量桌面端（Go + Wails + Swift 原生 ASR）**、**移动端（Pixel 原生 Android）**以及**实时云端网关服务（Go + LiveKit SFU）**。

---

## 🌟 核心特性

- ⚡️ **端云双环架构 (Dual-Loop)**：
  - **端侧极速快环（≤ 150ms）**：毫秒级意图分流路由器、本地车控模拟引擎、本地记忆底座与离线 ASR/TTS，断网物理隔离依然可用。
  - **云端深度慢环（P50 ≤ 800ms）**：基于 LiveKit SFU 与 Go 实时网关，支持全双工低时延实时音视频流与大模型多轮深度推理。
- 🎙️ **毫秒级全双工打断 (Barge-in ≤ 80ms)**：用户开嗓瞬间掐断本地发音，端云协同同步截断云端流式生成与下发。
- 🛡️ **严格隐私合规 (Zero-Retention Guardrail)**：未经用户显式授权严禁上云；云端零留存，严禁在服务端持久化任何用户原始音频或转写内容。
- 🖥️ **跨平台原生体验**：
  - **macOS**：无 Python 依赖纯 Go 单二进制，常驻内存仅 ~25-35MB，极简透明悬浮胶囊，支持硬件级回声消除（AEC）。
  - **Android**：针对 Google Pixel 10 定制的 Material 3 + 11 状态严密状态机前台服务。

> 📖 **关于完整的架构拓扑、时序交互、状态机生命周期与协议细节，请查阅 [系统工程架构设计文档 (docs/architecture.md)](docs/architecture.md)。**

---

## 🚀 快速开始

### 1. 启动云端实时服务 (LiveKit SFU + Go Gateway)

```bash
# 自动检测并在后台拉起 LiveKit SFU 容器 (:7880)，随后启动 Go 云端实时网关 (:8088)
make server
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
| **`make server`** | 启动 LiveKit SFU 容器及本地轻量云端实时网关（端口 :8088） |
| **`make test-e2e`** | 运行端云协同端到端对接与毫秒级协同打断集成测试 |
| `make run` / `make dev` | 编译 ASR 并直接运行桌面端（开发模式） |
| `make start` | 一键打包并直接启动 macOS 原生 App |
| `make package` | 编译并打包为 macOS 原生应用（生成 `build/bin/Avatar.app`） |
| `make build` | 仅编译桌面端可执行二进制文件 |
| `make asr` | 编译 macOS 原生离线语音识别组件（`bin/avatar-asr`） |
| `make test` | 运行项目所有 Go 单元测试 |
| `make android-test` | 运行 Android 原生客户端单元测试 |
| `make android-build` (或 `make apk`) | 构建 Android 原生客户端 Debug APK |
| `make android-install` (或 `make install-apk` / `make install`) | 构建并安装 Android Debug APK 到已连接的设备或模拟器 |
| `make livekit-stop` | 停止后台运行的 LiveKit SFU 容器 |
| `make clean` | 清理构建产物与临时文件 |

---

## 📂 项目结构

```
Avatar/
├── server/                 # 【云端服务】轻量实时网关 (Go) 与 LiveKit SFU 容器编排
├── internal/               # 【核心内部模块】
│   ├── cloud/              # 端侧与云端服务的长连接协同客户端 (Token / 打断 / 静音)
│   ├── dispatcher/         # 端侧意图分流路由器 (本地控制 vs 云端慢环决策)
│   ├── mockvehicle/        # 本地车控模拟引擎与安全规则审计
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
│   ├── architecture.md     # 🌟 系统全景架构设计文档 (时序图/状态机/接口契约)
│   ├── prd/                # 核心 PRD 规范与安全合规护栏定义
│   └── ...
├── app.go / main.go        # Wails 应用绑定胶水层与桌面入口
└── Makefile                # 统一构建、测试与运维指令
```

---

## ⚙️ 配置说明

桌面端首次启动会自动在用户目录下生成配置文件：`~/.avatar/config.json`：
- **本地大模型 (Ollama)**：设置 `llm.base_url` 为 `http://127.0.0.1:11434/v1`，`model` 为 `qwen2.5:1.5b` 等；
- **云端大模型 API**：配置兼容 OpenAI 的 `base_url`、`api_key` 与 `model`（如 DeepSeek-V3 / Qwen-Max）；
- **云端实时网关**：配置 `cloud_server_url` 指向你的云网关实例（如 `http://127.0.0.1:8088`）。

---

## 📚 详细设计与技术文档

所有底层技术设计、协议定义与工程规范请查阅 `docs/` 目录：
- 🌟 **[系统工程架构文档 (System Architecture)](docs/architecture.md)**：双环架构、客户端分层、云端基础设施、时序图、状态机、REST API 契约与安全机制。
- 📱 **[Android 移动端开发指南](android/README.md)**：Pixel 10 客户端构建、测试与前台服务配置。
- 📋 **[产品需求与合规护栏规范 (PRD)](docs/prd/)**：端云协同要求、数据零留存与车控安全约束。
