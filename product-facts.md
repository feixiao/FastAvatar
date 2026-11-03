# Product Facts: Avatar macOS 桌面智能伴侣

## 1. 产品定义与背景
- **产品名称**：Avatar (端云协同全双工智能语音伴侣)
- **运行平台**：macOS (Apple Silicon / Intel, macOS 14 Sonoma, macOS 15 Sequoia, 及未来概念)
- **底层架构**：
  - **端云双环架构 (Dual-Loop)**：
    - 端侧快环 (≤ 150ms)：Swift 原生 ASR、本地意图路由、轻量离线 TTS 与本地记忆库，纯 Go 单二进制常驻内存约 25~35MB。
    - 云端慢环 (P50 ≤ 800ms)：Go Gateway + LiveKit SFU 实时音视频网关，大模型多模态深度推理。
  - **毫秒级全双工打断 (Barge-in ≤ 80ms)**：用户说话瞬间掐断发音并向云端发送截断。
  - **Zero-Retention 零留存隐私**：未经许可不上云，端侧物理隔离。

## 2. 交互形态与核心需求
- **交互形态一：悬浮动态胶囊 (Floating Capsule)**
  - 极小视觉侵入，常驻屏幕边缘或顶部菜单栏下方。
  - 具备端云双环状态律动球（待命、倾听、思考、发音、休眠）、时延指示、人格标签、快捷静音/打断。
- **交互形态二：灵动案头伴侣 (Desktop Pet / Spatial Companion)**
  - 陪伴式实体，支持拖拽、触碰、投喂、表情律动与对话气泡。
- **交互形态三：伴侣中枢管理面板 (Companion Command Hub / Drawer)**
  - 记忆中枢 (Profile 画像 + Episode 情景记忆)、人格卡槽 (Persona Slots 与时段流转)、羁绊成长 (Bond & XP 系统)。

## 3. 当前实现痛点（现状走查）
- 原界面使用大量系统默认 Emoji (`🎙️`, `💬`, `🐟`, `✨`, `🎭`, `🔄`, `🧠`, `⏹️`) 作为按钮与功能标识，缺乏专业级设计语言。
- 采用通用 Tailwind 深色调（#0F172A 深蓝黑），缺乏 macOS 原生系统的材质深度、光影与精致感。
- 胶囊形态与抽屉面板层级生硬，缺乏系统级流动感与物理质感。
