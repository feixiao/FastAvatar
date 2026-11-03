# Product Facts: Avatar macOS 桌面智能伴侣

## 1. 产品定义与背景
- **运行平台**：macOS (桌面轻量常驻端) 与 Android (Google Pixel 10 / Tensor G5 / Android 16 Baklava 移动原生端)
- **底层架构**：
  - **端云双环架构 (Dual-Loop)**：
    - 端侧快环 (≤ 150ms)：Swift 原生 ASR (macOS) / Jetpack Compose + FastDispatcher (Android <1ms 路由判定)、离线 TTS 与本地记忆库，常驻内存仅 ~25-35MB。
    - 云端慢环 (P50 ≤ 800ms)：Go Gateway + LiveKit SFU 实时音视频网关，大模型多模态深度推理。
  - **毫秒级全双工打断 (Barge-in ≤ 80ms)**：用户说话瞬间掐断发音并向云端发送截断。
  - **Zero-Retention 零留存隐私与硬件媒体门禁**：未经用户许可不上云；Android 媒体门禁物理闭锁（Muted=true，仅在通话回合临时开门推流），耳机拔出瞬间暂停播放防泄露。

## 2. macOS 交互形态 (已选定 Things 3 纯白精修手账)
- 悬浮胶囊形态 (Floating Capsule)
- 灵动桌宠形态 (Desktop Pet)
- 伴侣案头手账中枢 (Companion Journal)
## 3. Android 交互形态与核心规格 (Pixel 10 / Android 16)
- **严密 11 状态会话状态机 (SessionStateMachine)**：
  `STOPPED` ⇄ `PREPARING` ⇄ `LISTENING` ⇄ `ROUTING` ⇄ `LOCAL_PROCESSING` ⇄ `LOCAL_ONLY` ⇄ `CLOUD_CONNECTING` ⇄ `CLOUD_TURN` ⇄ `SPEAKING` ⇄ `PAUSED` ⇄ `OFFLINE`
- **极速本地意图分流器 (FastDispatcher)**：
  优先级分流（口头记忆 -> 人设切换 -> 开放式云端对话），耗时实测 < 1ms。
- **两阶段本地记忆保险库 (MemoryVault)**：
  提出保存时先暂存复述，用户明确确认后才持久化；设备隔离，严禁未经确认同步。
- **前台服务与锁屏常驻 (Foreground Service)**：
  声明 `microphone|mediaPlayback`，锁屏与息屏下依然实时全双工陪伴。
- **动态交互精灵球 (AvatarPetOrb)**：
  待机呼吸、倾听声流波形、思考光环自转、说话跳动以及轻触抚摸微震动。

## 4. 当前 Android 实现痛点（现状走查）
- 当前 Android 代码（`VoiceAgentScreen.kt`）采用通用深蓝黑背景（`#111827`）配合杂乱霓虹色（青/绿/紫/黄/红），典型的“GitHub-dark 偷懒解”。
- 单调的线性列表堆砌，顶部应用栏狭小，缺乏 Google Pixel 10 旗舰机专属的 Material 3 Expressive 高度流动性、物理弹簧感与现代设计语言。
- 缺少沉浸式的全屏展示与多机全貌视图（会话主屏、记忆保险库、多模态人设、锁屏前台通知）。
