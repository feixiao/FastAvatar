# Avatar Android 客户端 (Pixel 10 / Android 16)

基于 [`PRD_AI_AUDIO_AGENT_CLIENT_MVP_v1.0.md`](../docs/prd/PRD_AI_AUDIO_AGENT_CLIENT_MVP_v1.0.md) 实现的原生 Android 语音陪伴智能体客户端。

---

## 1. 核心架构与 PRD 对齐

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                       Avatar Android Client (Pixel 10)                      │
│                                                                             │
│  ┌───────────────────────┐          ┌────────────────────────────────────┐  │
│  │   UI (Jetpack Compose)│          │ Foreground Service (Microphone &   │  │
│  │   Material 3 Dark     │          │ MediaPlayback, Screen-off Support) │  │
│  └───────────┬───────────┘          └─────────────────┬──────────────────┘  │
│              ▼                                        ▼                     │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │              SessionStateMachine (11 个状态严格状态机)                  │  │
│  │    STOPPED ⇄ PREPARING ⇄ LISTENING ⇄ ROUTING ⇄ SPEAKING ⇄ PAUSED...   │  │
│  └───────────────────────────────────┬───────────────────────────────────┘  │
│                                      │                                      │
│                  ┌───────────────────┴───────────────────┐                  │
│                  ▼                                       ▼                  │
│      ┌───────────────────────┐               ┌───────────────────────┐      │
│      │  FastDispatcher (<1ms)│               │  CloudClient (LiveKit)│      │
│      └───────────┬───────────┘               └───────────┬───────────┘      │
│                  │                                       │                  │
│        ┌─────────┴─────────┐                 ┌───────────┴───────────┐      │
│        ▼                   ▼                 ▼                       ▼      │
│ ┌──────────────┐   ┌───────────────┐  ┌─────────────┐       ┌─────────────┐ │
│ │ MemoryVault  │   │MockVehicle    │  │Audio Gating │       │Barge-In     │ │
│ │ (设备隔离)   │   │Engine (审计)  │  │(Muted=true) │       │(<80ms 打断) │ │
│ └──────────────┘   └───────────────┘  └─────────────┘       └─────────────┘ │
└─────────────────────────────────────────────────────────────────────────────┘
```

### 1.1 核心组件职责
- **会话状态机 (`SessionStateMachine`)**:
  - 严格遵循 PRD Section 3.3 定义的 11 个状态：`STOPPED`, `PREPARING`, `LISTENING`, `ROUTING`, `LOCAL_PROCESSING`, `LOCAL_ONLY`, `CLOUD_CONNECTING`, `CLOUD_TURN`, `SPEAKING`, `PAUSED`, `OFFLINE`。
- **本地极速分流器 (`FastDispatcher`)**:
  - 优先级 1: 口头记忆命令（`memory.command`: 记住/记一下/忘掉/清除记忆）→ `LOCAL_MEMORY`
  - 优先级 2: Mock 车控意图（`climate.set_temperature`, `window.control`, `drive.switch_mode`）→ `MOCK_VEHICLE`
  - 优先级 3: 人设切换命令（`persona.switch`）→ `LOCAL_PERSONA`
  - 优先级 4: 开放式对话 → 根据授权与联网状态路由至 `CLOUD_CHAT` / `LOCAL_ONLY_PROMPT` / `OFFLINE_PROMPT`
  - 耗时控制：端侧判定时延实测 `< 1ms`（PRD 目标 ≤ 150ms）。
- **Mock 车控审计引擎 (`MockVehicleEngine`)**:
  - 每条意图输出一条严格 JSON Lines 格式审计日志至 `context.filesDir/mock_vehicle.log`；
  - 字段：`timestamp` (UTC RFC 3339)、`intent`、`slots`、`decision` (`MOCK_ACCEPTED` | `MOCK_REJECTED`)、`reason`、`simulation: true`；
  - 安全边界检查：空调温度校验（16.0 ~ 30.0℃，超限返回 `ERR_TEMP_OUT_OF_RANGE`）；危险驾驶模式严格拦截（漂移/赛道/越野返回 `ERR_DANGEROUS_ACTION_REJECTED`）；
  - 数据最小化：严禁记录原始音频或语音转写明文。
- **本地记忆保险库 (`MemoryVault`)**:
  - 两阶段确认机制：提出保存时先暂存复述，用户明确确认后才持久化；
  - 设备隔离：基于设备标识存储，Pixel 与 Mac 端互相隔离，严禁未经确认同步；
  - 永久删除：删除操作物理生效，后续检索不再返回。
- **云端实时客户端 (`CloudClient`)**:
  - 媒体门禁（Audio Track Gating）：音轨默认物理闭锁（`isAudioMuted = true`），仅在云端回合激活时开门；
  - 全双工插话打断（Barge-In）：本地扬声器瞬间静音（耗时 `< 80ms`），同时向云端推送 `/api/v1/session/interrupt`；
  - 离线保护：断网不排队、恢复后不自动补发旧音频。
- **系统生命周期与隐私防护 (Pixel 10 / Android 16)**:
  - 前台服务 (`VoiceSessionService`): 声明 `foregroundServiceType="microphone|mediaPlayback"`，支持息屏与锁屏下会话继续运行；
  - 音频隐私保护 (`AudioPrivacyGuard`): 监听 `ACTION_AUDIO_BECOMING_NOISY`，耳机拔出时瞬间暂停播放，杜绝私密语音外放至扬声器；
  - 音频焦点管理 (`AudioFocusHelper`): 监听电话及第三方音频抢占，自动暂停会话。
- **本地智能情感闲聊引擎 (`LocalCompanionEngine`)**:
  - 杜绝断网或未连云端时无法聊天；提供问候、情绪共情、幽默笑话、睡前故事及人设自适应的本地多轮陪伴对话；
  - 与 `MemoryVault` 深度整合，可记忆用户专属偏好并随时温情唤回。
- **全双工语音输入输出 (`SpeechRecognitionHelper` & `AndroidSpeechSynthesizer`)**:
  - 语音识别输入：集成 Android 原生 `SpeechRecognizer`，支持实时音量波形感知与实时转写预览；
  - 语音合成输出：集成 Android 原生 `TextToSpeech`，真实朗读助手回复，支持可配置语速与多语言；
  - 实体物理打断：插话或点击打断时调用 `tts.stop()` 瞬间闭音（< 80ms）。
- **动态交互精灵球 (`AvatarPetOrb`)**:
  - Jetpack Compose Canvas 原生渲染；提供待机呼吸、倾听声流波形、思考光环自转、说话跳动以及轻触抚摸（伴随震动反馈与情绪反馈）。
- **配置与人设管理 (`AppSettings` & `SettingsSheet`)**:
  - 支持配置云端网关地址、局域网 IP、自定义大模型端点（Ollama / DeepSeek / OpenAI）、TTS 语速与多重人设切换（知心伴侣、元气小猫、高效幕僚、毒舌损友）。
- **诊断日志 (`DiagnosticLogger`)**:
  - 写入 `diagnostics.log`，严格遵循 PRD Section 5.3，仅记录受控元数据，杜绝内容泄露。

---

## 2. 目录结构

```
android/
├── app/
│   ├── build.gradle.kts
│   ├── src/
│   │   ├── main/
│   │   │   ├── AndroidManifest.xml
│   │   │   ├── java/com/avatar/android/
│   │   │   │   ├── VoiceApplication.kt
│   │   │   │   ├── core/
│   │   │   │   │   ├── state/         # SessionState, SessionStateMachine
│   │   │   │   │   ├── dispatcher/    # FastDispatcher, RouteTarget, DispatchResult
│   │   │   │   │   ├── mockvehicle/   # MockVehicleEngine, AuditLogEntry, VehicleIntentParser
│   │   │   │   │   ├── memory/        # MemoryVault, MemoryItem
│   │   │   │   │   ├── cloud/         # CloudClient, CloudModels
│   │   │   │   │   ├── audio/         # AudioFocusHelper, AudioPrivacyGuard, AudioPlaybackController
│   │   │   │   │   └── diagnostics/   # DiagnosticLogger, DiagnosticEvent
│   │   │   │   ├── service/           # VoiceSessionService (Foreground Service)
│   │   │   │   └── ui/                # MainActivity, VoiceAgentViewModel, VoiceAgentScreen
│   │   │   │       ├── components/    # MemorySheet, AuditLogSheet
│   │   │   │       └── theme/         # Color, Theme
│   │   │   └── res/                   # values, xml (backup rules, themes)
│   │   └── test/                      # JUnit 单元测试集
│   │       └── java/com/avatar/android/core/
│   │           ├── FastDispatcherTest.kt
│   │           ├── MockVehicleEngineTest.kt
│   │           ├── MemoryVaultTest.kt
│   │           ├── CloudClientTest.kt
│   │           └── SessionStateMachineTest.kt
├── build.gradle.kts
├── settings.gradle.kts
├── gradle.properties
└── local.properties
```

---

## 3. 构建与测试指令

在项目根目录下，可通过 Makefile 直接调用：

```bash
# 运行 Android 单元测试 (FastDispatcher, MockVehicleEngine, MemoryVault, CloudClient, SessionStateMachine)
make android-test

# 构建 Debug APK (app/build/outputs/apk/debug/app-debug.apk)
make android-build   # 或 make apk

# 一键构建并安装 APK 到连接的 Pixel 设备/模拟器
make android-install # 或 make install-apk / make install
```

或者进入 `android` 目录运行：

```bash
cd android
./gradlew test            # 执行单元测试
./gradlew assembleDebug   # 编译 APK
./gradlew installDebug    # 编译并安装 APK 到连接设备
```

---

## 4. 验收测试重点

| 验收项目 | 验证步骤 | 预期结果 |
| :--- | :--- | :--- |
| **端侧快环** | 输入"把空调调到24度" | 本地立即响应（< 150ms），生成 `MOCK_ACCEPTED` 审计日志，媒体门禁保持闭锁 |
| **安全规则拦截** | 输入"切换到漂移模式" | 拦截危险意图，生成 `MOCK_REJECTED` 审计日志（`ERR_DANGEROUS_ACTION_REJECTED`） |
| **两阶段本地记忆** | 输入"帮我记一下美式咖啡" | 界面弹出待保存确认卡片，点击确认后方持久化至本地私有文件 |
| **媒体门禁闭锁** | 检查云端客户端媒体状态 | 音轨默认处于 Muted=true 闭锁状态，仅在云端通话回合临时开门推流 |
| **全双工插话打断** | 播报过程中点击"打断" | 扬声器播报立即终止（≤ 80ms），音轨闭锁，发送中断信号给云端 |
| **离线降级** | 切换离线模式 | 车控与记忆功能完全在本地可用；开放式问题友好提示离线，不排队补发旧语音 |
| **耳机拔出防泄露** | 播报时触发 Becoming Noisy | 立即暂停播放，杜绝私密语音从扬声器外放 |
