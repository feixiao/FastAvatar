# FastAvatar 灵动宠伴 (Desktop & Mobile AI Animated Companion) 跨端产品需求文档 (PRD)

| 属性 | 内容 |
| :--- | :--- |
| **文档编号** | PRD-FASTAVATAR-ANIMATED-PET-2026-001 |
| **版本 / 状态** | v1.2 / Baseline Ready (macOS 桌面悬浮态 + Android 全屏态双端齐备) |
| **产品定位** | 融合“极速口语速记 + 情感共情伴聊”的跨端高拟真全双工动画桌宠 (macOS / Android) |
| **默认官方宠伴** | **罗小黑 2D (`luoxiaohei2d`)** —— 纯黑灵动小猫、黄绿大眼、呆萌耳朵与分叉尾巴（源自 Codex Pets 官方生态） |
| **适用终端** | **macOS** (Wails 桌面无框透明悬浮态 + Things 3 风格手账中枢) & **Android** (Jetpack Compose 原生应用内全屏沉浸态) |
| **核心工程形态** | 端侧快环 (≤150ms 本地 ASR/TTS/记忆分流) + 云端慢环 (SSE 深度推理) + 毫秒级声学打断 |
| **文档负责人** | 资深技术产品经理 (TPM) |

---

## 1. 商业动机、用户价值与双端形态收敛 (Why & Form Factors)

### 1.1 目标客群 (ICP) 与触发场景 (Trigger)
- **目标客群 (ICP)**：
  - **macOS 重度桌面创作者**：写代码、码字、做架构设计的工程师与极客，屏幕角落常驻一只灵动小黑猫，随时按快捷键或随口唤醒，不抢焦点，随手记灵感；
  - **Android 移动随身伴侣**：通勤、散步、睡前独处，打开 App 面对面全屏全双工对聊，摸摸头释放压力。

### 1.2 双端形态对比与职责边界

| 维度 | macOS 桌面端 (Desktop Floating) | Android 移动端 (Mobile In-App) |
| :--- | :--- | :--- |
| **核心承载形态** | **透明无框桌面悬浮窗** ($192 \times 208\text{px}$) + **双击展开手账中枢** | **应用内全屏沉浸伴侣** (放弃高危系统浮窗权限) |
| **窗口层级与焦点** | `NSFloatingWindowLevel` 置顶悬浮，`NSWindowStyleMaskNonActivating` 点击不抢占当前 IDE/浏览器焦点 | Android Activity 全屏前台常驻，支持息屏前台服务对聊 |
| **拖拽与物理停靠** | 鼠标按住罗小黑任意拖拽，小黑自动切入 Row 1（右奔）/ Row 2（左奔）动作；松手吸附停靠边缘 | 居中固定舞台，单指滑抚配合触觉线性马达微震动反馈 |
| **交互中枢入口** | **双击罗小黑**：平滑滑出 Things 3 纯白极简手账中枢 (Memory Journal) | 顶部灵感气泡轻量滑出，自动沉淀至 MemoryVault |
| **右键快捷菜单** | 切换人设、投喂小鱼干、打盹休眠、隐藏至顶部状态栏 (MenuBar) | 底部一键交互 Action 栏（干饭、休眠、速记、人设） |

---

## 2. macOS 桌面端设计规范 (macOS Desktop Specification)

### 2.1 窗口与画布规格
- **悬浮窗尺寸**：$192 \times 208\text{px}$ 原生画质，物理像素点对点（Retina 屏自动适配 `@2x`）；
- **透明度与抗锯齿**：窗口透明背景，Canvas 开启 `image-rendering: pixelated`，保留罗小黑极致手绘像素感；
- **拖拽体验**：
  - 按住鼠标左键拖动：位移判定 $\Delta x > 0$ 播放 Row 1（向右飞奔），$\Delta x < 0$ 播放 Row 2（向左飞奔）；
  - 松开鼠标：桌宠自动停靠并重置为 Row 0 `idle`，距离屏幕边缘 $< 30\text{px}$ 时自动吸附贴边。

### 2.2 双击展开：案头手账中枢 (Companion Journal)
- **触发方式**：双击罗小黑身体，悬浮窗右侧平滑滑出 $340 \times 480\text{px}$ 的极简手账中枢面板；
- **视觉风格**：对齐 Things 3 纯白高质感排版，无杂乱弹窗；
- **核心内容**：
  1. **今日捕获的灵感速记流**（按时间倒序排列，带“复制”、“归档”）；
  2. **罗小黑好感度与羁绊徽章**（当前好感度 Lv、累计陪伴天数、干饭次数）；
  3. **模型与端侧配置抽屉**（本地 Ollama / LM Studio 端点、云端 API Key 自带设置）。

---

## 3. 官方默认资产契约：罗小黑 2D (`luoxiaohei2d`)

- **资产源地址**：`https://codex-pets.net/#/pets/luoxiaohei2d` (API: `https://codex-pets.net/api/pets/luoxiaohei2d`)
- **图集尺寸规格 (Atlas Specification)**：
  - **单帧网格尺寸 (Cell Size)**：$192 \times 208\text{px}$
  - **整图尺寸 (Atlas Size)**：$1536 \times 1872\text{px}$（8 列 $\times$ 9 行，共 72 帧）
- **Codex Pet 8 行标准动作网格映射**：
  - **Row 0 (6帧)**：`idle` —— 待机呼吸、眨眼、尾巴摇摆；
  - **Row 1 (8帧)**：`running-right` —— macOS 向右拖拽移动；
  - **Row 2 (8帧)**：`running-left` —— macOS 向左拖拽移动；
  - **Row 3 (4帧)**：`waving` —— 语音播报（模式 B：RMS 能量驱动挥爪）；
  - **Row 4 (5帧)**：`jumping` —— 抚摸戳戳、双击唤醒纵跳 (+5 XP)；
  - **Row 5 (8帧)**：`failed` —— 异常断网、报错困惑；
  - **Row 6 (6帧)**：`waiting` —— 凝神倾听（双耳竖起，绝对闭嘴）；
  - **Row 7 (6帧)**：`running` —— 投喂小鱼干、敏捷暴风干饭；
  - **Row 8 (6帧)**：`review` —— 模型深度思考（沉思审视）。

---

## 4. 声学与状态机驱动规范 (模式 B)

- **RMS 能量三段分级驱动**：
  - $E < 0.05$（语间停顿/换气）：强制归位待机姿势，严禁假唱抽搐；
  - $0.05 \le E < 0.45$（日常发音）：平滑播放 Row 3 动作；
  - $E \ge 0.45$（重音/兴奋）：完整呈现 Row 3 兴奋挥爪；
- **全双工插话打断 (Barge-in)**：用户开口检测有效声学能量时，扬声器瞬断，罗小黑在 **$\le 50\text{ms}$** 内强制收口闭合，瞬切 Row 6 前倾倾听。
