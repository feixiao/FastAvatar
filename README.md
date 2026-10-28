# Avatar — 端云协同全双工智能语音伴侣 (Go + Wails + LiveKit)

本项目是一套**端云协同、双环驱动（Dual-Loop）、低时延流式、全双工可打断（Barge-in）**的跨平台智能语音伴侣系统，包含**轻量桌面端（Go + Wails）**、**移动端（Pixel 原生 Android）**以及**实时云端网关服务（Go + LiveKit SFU）**。

---

## 一、 架构与定位

系统采用**端云双环架构（Dual-Loop Architecture）**：
* **端侧极速快环（Local Fast Loop ≤ 150ms）**：端侧感知、毫秒级意图分流路由器（Local Intent Dispatcher）、本地物理控制与车控模拟执行引擎（Mock Vehicle Engine）、离线 ASR 与极速发音，断网状态仍可控。
* **云端深度慢环（Cloud Deep Loop）**：基于 LiveKit SFU 与 Go 实时网关，支持全双工低时延实时音视频流、多轮长程对话大模型推理、云端协同打断信令与隐私合规安全护栏。

```
                ┌────────────────────────────────────────────────────────┐
                │             端侧设备 (macOS 桌面端 / Android 原生)       │
                │                                                        │
                │  麦克风/AEC ──> 离线 VAD ──> 端侧意图分流 (Dispatcher)   │
                │                               │                        │
                │            ┌──────────────────┴──────────────────┐     │
                │            ▼ (本地控制意图 ≤150ms)  ▼ (复杂对话/推理)    │
                │       本地物理/车控执行引擎       端云长连接隧道         │
                └──────────────────────────────────────┬─────────────────┘
                                                       │ WebRTC Audio / REST 信令
                                                       ▼
                ┌────────────────────────────────────────────────────────┐
                │             云端基础设施 (Cloud Gateway & SFU)           │
                │                                                        │
                │   • LiveKit SFU (WebRTC 音视频分发, :7880)              │
                │   • Go 云端实时网关 (Token 鉴权 / 打断 / 会话审计, :8088) │
                │   • 云端 Realtime LLM 与任务编排 Agent                   │
                └────────────────────────────────────────────────────────┘
```

### 1.1 核心特性
1. **端云协同毫秒级打断（Barge-in）**：
   - 用户开嗓瞬间触发端侧本地打断，毫秒级掐断本地 TTS 发音并切换倾听状态。
   - 端侧同步向云端服务发送打断信令（`/api/v1/session/interrupt`），云端即刻截断流式生成与音频下发。
2. **纯 Go 驱动原生桌面应用**：
   - 彻底移除 Python 运行时依赖，单二进制（仅 ~6.5MB），开机极速自启，常驻内存仅 ~25-35MB。
3. **极简动态悬浮胶囊 (Dynamic Island Pill)**：
   - 屏幕置顶、无边框透明窗口，支持任意位置拖拽。
   - 4 种视觉与交互状态流转：`待命 🎙️` ➔ `倾听 👂` ➔ `思考 💭` ➔ `说话 🔊`。
4. **免唤醒外放 + 硬件级回声消除 (AEC)**：
   - 采用硬件/系统级回声消除，扬声器外放时麦克风不会收录自身发声。
5. **本地大模型优先 (SLM-First) 与云端平滑降级**：
   - 优先通过后台无头伴生进程直连本地端侧小模型（如 Qwen2.5 GGUF / 本地 Ollama）；按需平滑路由至云端实时模型。
6. **严格隐私安全护栏 (Zero-Retention Guardrail)**：
   - 云端网关仅记录非敏感审计日志（时延、错误码、会话状态），严禁在服务端持久化用户原始音频流与完整文本内容。
   - 云端连接严格校验用户显式授权（Consent Guard），未授权立即阻断。

---

## 二、 目录结构

```
Avatar/
├── server/                 # 【云端服务】轻量实时网关与 LiveKit SFU 编排
│   ├── cmd/main.go         # 云端网关启动入口（默认端口 :8088）
│   ├── server.go           # REST API 路由、活跃会话状态机与协同打断处理
│   ├── auth.go             # LiveKit Token 签发、加密签名与隐私 Consent 校验
│   ├── server_test.go      # 服务端单元测试与合规性验证
│   └── docker-compose.yaml # LiveKit SFU 服务容器配置（端口 7880/7881/7882）
├── internal/               # 【核心内部模块】
│   ├── cloud/              # 端侧与云端服务的长连接协同客户端 (Token / 打断 / 静音)
│   ├── dispatcher/         # 端侧意图分流路由器 (本地控制 vs 云端慢环决策)
│   ├── mockvehicle/        # 本地车控/设备物理控制模拟引擎与安全审计
│   ├── engine/             # 桌面端状态机与会话循环
│   ├── splitter/           # 流式文本断句器 (首句低延时发音)
│   ├── asr/                # 语音识别抽象层
│   ├── tts/                # 语音合成抽象层 (macOS say / 云端 TTS)
│   ├── llm/                # OpenAI 兼容流式客户端
│   └── config/             # 配置管理 (config.json)
├── cmd/                    # 【可执行工具与测试套件】
│   ├── avatar-asr/         # macOS 原生高性能 Swift 离线语音识别 CLI
│   └── e2e-test/           # 端云协同端到端集成与打断贯通验收测试
├── android/                # 【移动端】Google Pixel 10 原生 Android 客户端 MVP
│   ├── app/                # Android 胶囊浮窗与语音交互实现
│   └── README.md           # Android 客户端开发与运行指南
├── frontend/               # 【桌面端前端】透明声波胶囊 (HTML + CSS + Web Audio)
├── docs/                   # 【系统设计与规范】
│   ├── edge_cloud_voice_companion_technical_design.md # 技术架构全景设计
│   ├── hybrid_edge_cloud_agent_architecture.md       # 端云双环架构方案
│   └── prd/                # 核心 PRD 规范与合规护栏定义
├── app.go                  # Wails 应用绑定胶水层
├── main.go                 # 桌面端启动入口
├── package_mac.sh          # macOS .app 原生打包脚本
├── Makefile                # 统一构建、测试与运维指令
└── README.md
```

---

## 三、 云端服务部署与运行 (Cloud Server & LiveKit)

云端服务由两部分组成：
1. **LiveKit SFU**：负责低时延 WebRTC 音频/视频/数据通道分发。
2. **Go 云端实时网关 (Avatar Cloud Gateway)**：负责设备鉴权、LiveKit 房间 Token 动态分发、协同打断信令同步与会话审计。

### 3.1 环境变量配置

启动云端网关前可设置以下环境变量（未设置时使用安全默认值）：

| 环境变量 | 默认值 | 说明 |
| :--- | :--- | :--- |
| `PORT` | `8088` | 云端网关监听的 HTTP 端口 |
| `LIVEKIT_URL` | `ws://localhost:7880` | LiveKit SFU 节点 WebSocket 地址 |
| `LIVEKIT_API_KEY` | `devkey` | LiveKit API Key（需与 SFU 一致） |
| `LIVEKIT_API_SECRET` | `secret` | LiveKit API Secret（需与 SFU 一致） |

### 3.2 启动步骤

#### 一键启动（推荐）
```bash
# 执行 make server 将自动检测并在后台拉起 LiveKit SFU 容器 (:7880)，随后启动 Go 云端网关 (:8088)
make server
```

若需手动控制或在无 Docker 环境下分步调试：
- **单独拉起 LiveKit SFU**：`docker compose -f server/docker-compose.yaml up -d`
- **停止后台 LiveKit 容器**：`make livekit-stop` (或 `docker compose -f server/docker-compose.yaml down`)
- **单独运行 Go 云端网关**：`go run ./server/cmd/main.go`

LiveKit 启动后将在本地暴露：
- `7880`：HTTP / WebSocket 信令端口
- `7881`：WebRTC TCP 传输
- `7882/udp`：WebRTC UDP 流媒体传输

启动成功日志示例：
```
[LiveKit] SFU 已在运行 (:7880)
[Cloud Gateway] Listening on :8088 (LiveKit SFU: ws://localhost:7880)
```

### 3.3 核心 API 接口速查

云端网关提供以下标准 REST API：

* **`GET /health`**：服务健康检查
  - 响应：`{"status":"ok","service":"avatar-cloud-realtime"}`
* **`POST /api/v1/auth/token`**：获取 LiveKit 连接凭证
  - 请求体：
    ```json
    {
      "user_id": "usr_001",
      "device_id": "dev_mac_01",
      "client_type": "mac",
      "cloud_consented": true
    }
    ```
  - 特性：若 `cloud_consented` 为 `false`，接口返回 `403 Forbidden` 并记录审计事件，严防未经授权上云。
* **`POST /api/v1/session/interrupt`**：端侧协同打断通知
  - 端侧检测到用户说话切入时调用，云端立即中断当前轮次语音流合成与下发。
* **`POST /api/v1/session/close`**：会话关闭与资源清理

### 3.4 端云协同端到端集成测试

项目内置了完整的端到端集成验收工具，可自动验证“云端网关启动 ➔ Token 获取 ➔ 意图路由 ➔ 端侧打断 ➔ 云端协同截断”完整闭环：

```bash
# 一键运行端云协同全链路贯通测试
make test-e2e
```

---

## 四、 客户端快速开始

### 4.1 Makefile 常用指令一览

项目提供了一致的指令封装，覆盖桌面端、服务端、移动端与测试：

| 指令 | 作用 |
| :--- | :--- |
| **`make server`** | **启动本地轻量云端实时网关服务（8088 端口）** |
| **`make test-e2e`** | **运行端云协同端到端对接与协同打断贯通测试** |
| `make run`（或 `make dev`） | 编译 ASR 并直接运行桌面端（开发调试） |
| `make start` | 一键打包并立即启动 macOS 原生 App |
| `make package` | 编译并打包为 macOS 原生应用（生成 `build/bin/Avatar.app`） |
| `make build` | 仅编译桌面端可执行二进制文件（`build/bin/Avatar`） |
| `make asr` | 编译 macOS 原生离线语音识别组件（`bin/avatar-asr`） |
| `make test` | 运行项目所有单元测试 |
| `make android-test` | 运行 Android 原生客户端单元测试 |
| `make android-build`| 构建 Android 原生客户端 Debug APK |
| `make clean` | 清理构建产物及临时文件 |
| `make help` | 查看所有 Makefile 帮助说明 |

### 4.2 桌面端运行与打包 (macOS)

```bash
# 开发模式直接启动（自动检测与编译 Swift 原生 ASR）
make run

# 生产环境打包并在本地启动
make start
```

### 4.3 移动端 Android 客户端

Android 原生客户端代码位于 `android/` 目录：
```bash
# 运行 Android 单元测试
make android-test

# 编译 Android Debug 安装包
make android-build
# 产物位于 android/app/build/outputs/apk/debug/app-debug.apk
```
详细配置请查阅 [android/README.md](android/README.md)。

### 4.4 大模型与配置管理

桌面端首次运行会自动在用户根目录下生成配置文件：`~/.avatar/config.json`：
- **本地模型 (Ollama)**：设置 `llm.base_url` 为 `http://127.0.0.1:11434/v1`，`model` 为 `qwen2.5:1.5b` 等。
- **云端大模型 API**：配置兼容 OpenAI 的 `base_url`、`api_key` 与 `model`（如 DeepSeek-V3 / Qwen-Max）。
- **云端实时网关**：可配置 `cloud_server_url` 指向你的云网关实例（如 `http://127.0.0.1:8088`）。

### 4.5 交互说明
- **免唤醒畅聊**：直接对设备说话，伴侣自动感知并流式语音回复；伴侣发音期间随时插话即可打断。
- **模式切换**：点击胶囊上的「免唤醒」按钮可切换为「快捷键」模式。
- **文字输入**：点击胶囊上的 `💬` 按钮可呼出极简文本框键入交流。
- **一键打断/重置**：点击 `⏹` 或胶囊核心呼吸球，瞬间终止发音并重置会话。

---

## 五、 架构设计与技术文档

深入了解系统设计细节与安全规范，请参考 `docs/` 目录下的技术文档：
* [端云协同实时智能体架构方案 (Dual-Loop Architecture)](docs/hybrid_edge_cloud_agent_architecture.md)
* [端云协同极简语音伴侣全栈技术架构设计](docs/edge_cloud_voice_companion_technical_design.md)
* [产品需求与安全合规规范 (PRD)](docs/prd/)
