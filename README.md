# FastAvatar: 低时延语音交互数字人 (Fast Streaming Voice Avatar)

本项目是一套**低时延、模块化、本地大模型优先**的语音互动数字人系统。
设计理念深度参考了 [Open-LLM-VTuber](https://github.com/Open-LLM-VTuber/Open-LLM-VTuber/blob/main/README.CN.md)，采用端到端流式流水线（Streaming Pipeline）架构，当前重点落地**高质量语音对话交互**，底层预留并实现了完整的**音视频驱动数据通道（Mouth Viseme / Blendshapes / Live2D / 3D VRM / 神经人脸驱动）**，方便平滑扩展为带画面的完整虚拟主播与桌面陪伴数字人。

---

## 目录
- [一、 整体架构与分阶段落地路线](#一-整体架构与分阶段落地路线)
- [二、 核心设计要点与时延优化](#二-核心设计要点与时延优化)
- [三、 模型选型（本地优先 + 云端统一抽象）](#三-模型选型本地优先--云端统一抽象)
- [四、 视频/画面驱动方案扩展设计（Phase 2-4）](#四-视频画面驱动方案扩展设计phase-2-4)
- [五、 快速开始与使用指南](#五-快速开始与使用指南)
- [六、 自动化评测与基准性能 (Autoresearch)](#六-自动化评测与基准性能-autoresearch)

---

## 一、 整体架构与分阶段落地路线

### 1.1 总体架构数据流

```
                    ┌────────────────────────────────────────────────────────┐
                    │                      用户交互终端                        │
                    │   (Web 页面 / 桌面客户端 Electron / 手机移动端)            │
                    └───────────▲───────────────────────────────┬────────────┘
                                │                               │
                   实时音频流 (PCM/Opus)                   麦克风录音输入
                   + 嘴型/动效元数据 (Visemes)              + 打断信号 (Barge-in)
                                │                               │
                                │ (WebSocket 双工通道)           │
                                │                               ▼
┌───────────────────────────────┴───────────────────────────────┴────────────────────────┐
│                               FastAvatar 后端流式核心引擎                               │
│                                                                                        │
│  ┌───────────────┐     ┌───────────────┐     ┌──────────────────────────────────────┐  │
│  │   VAD 模块    │────▶│   ASR 模块    │────▶│              LLM 对话引擎             │  │
│  │ 语音活动检测   │     │   语音转文字   │     │  (Ollama Qwen2.5 / OpenAI 兼容接口)   │  │
│  └───────────────┘     └───────────────┘     └──────────────────┬───────────────────┘  │
│                                                                 │ Token 实时流式输出     │
│                                                                 ▼                      │
│                                              ┌──────────────────────────────────────┐  │
│                                              │      Sentence Splitter 智能流式断句器 │  │
│                                              │ (按标点+语义实时切分成 TTS 短句短语)   │  │
│                                              └──────────────────┬───────────────────┘  │
│                                                                 │ 短句队列 (Streaming)  │
│                                                                 ▼                      │
│  ┌─────────────────────────────────────┐     ┌──────────────────────────────────────┐  │
│  │        Lip Sync 嘴型特征提取器       │◀────│               TTS 语音合成           │  │
│  │  (RMS能量 + 5元音 Visemes / 动作)   │     │ (macOS say / Edge-TTS / Sherpa-ONNX) │  │
│  └──────────────────┬──────────────────┘     └──────────────────────────────────────┘  │
│                     │ 音频包 + 嘴型特征帧                                                │
│                     └──────────────────────▶ 编码下发                                    │
└────────────────────────────────────────────────────────────────────────────────────────┘
```

### 1.2 四阶段实现路线图

| 阶段 | 交付形态 | 核心功能 | 状态 |
| :--- | :--- | :--- | :--- |
| **Phase 1** | **语音交互 MVP（本项目当前实现）** | 全双工流式语音交互、智能流式断句、用户插话打断（Barge-in）、本地大模型适配、基础 Web 交互端与口型波形可视化 | **已落地** |
| **Phase 2** | **Live2D / 2.5D 虚拟主播模式** | 对接 PixiJS / Live2D Cubism SDK，利用后端吐出的 `mouth_open` 与 `visemes` 直接驱动 `ParamMouthOpenY`，配合情绪映射驱动表情（喜怒哀乐） | 下一步接入 |
| **Phase 3** | **3D VRM / 桌宠客户端** | 基于 Three.js / Tauri，加载 3D VRM 角色模型，支持透明桌面置顶穿透（Desktop Pet），支持凝视跟随与动作混合 | 规划中 |
| **Phase 4** | **写实人脸视频流式生成** | 对接 MuseTalk / LivePortrait / SadTalker 等深度学习口型生成模型，后端利用 WebRTC 实时推送 25fps 数字人视频流 | 进阶规划 |

### 1.3 伴侣养成系成长系统 (Cultivation & Memory Engine)

项目内置完整的**低时延养成系伴侣成长系统**，具备以下三大维度：

1. **亲密度等级进阶（Progression Engine）**：
   - 每次有效语音/文字交互自动累积亲密度经验值 (XP)，连续多天相伴触发连击加成（Streak）。
   - 关系阶段逐步解锁：`Lv.1 初识助手` ➔ `Lv.2 熟络朋友` ➔ `Lv.3 知心密友` ➔ `Lv.4 羁绊伴侣` ➔ `Lv.5 灵魂契约`。
   - 随着等级提升，数字人的称呼、语调与情感深度自然进化。
2. **零延迟后台记忆沉淀（Zero-Latency Memory Vault）**：
   - 在对话完成后，后台异步微任务自动提炼对话中关于用户的关键事实（喜好、宠物、工作、习惯、称呼约定），存储于本地 `data/companion.json`。
   - 下一轮对话自动检索最相关事实毫秒级动态注入 System Prompt，做到“越聊越懂你”。
   - Web 端提供可视化**记忆档案抽屉**，支持用户自主查看、添加或删除记忆条目。
3. **视觉与情绪进化（Visual & Mood Evolution）**：
   - 核心律动球与声波频谱随等级进化呈现不同主题光晕（科技蓝 ➔ 薰衣草紫 ➔ 暖珊瑚粉 ➔ 耀金双色 ➔ 幻彩星云）。
   - 时间感知与思念机制：感知清晨/傍晚/深夜，并根据相隔未互动时长触发动态思念问候。
   - 升阶时触发专属里程碑庆祝弹窗与台词。
### 1.4 全双工免唤醒「连续对话」交互 (Hands-Free Continuous Dialogue)

针对长时间陪伴、自然畅聊场景，系统在 Web 端默认提供**全双工免唤醒连续对话模式**：

1. **全自动 VAD 语音分段**：麦克风保持常开监听，用户开口即开始录音；句末自然停顿约 750ms 后自动切句并打包发送，全程无需按任何按键。
2. **硬件级回声消除（AEC）**：启用浏览器原生回声消除、底噪抑制与自动增益，防止扬声器外放声音引起误触循环。
3. **随时随地口语打断（Barge-in）**：在数字人回答期间，用户只要直接开口插话，系统在 200ms 内自动丢弃未播放音频并切换为倾听状态。
4. **多模式灵活切换**：针对安静环境提供“连续对话（免唤醒）”，针对嘈杂公共场景保留“按住说话 (PTT)”与“点击录音”。
---

## 二、 核心设计要点与时延优化

在语音交互数字人系统中，**对话时延（Latency）是决定体验生死的核心**。

### 2.1 传统非流式调用的严重缺陷
如果等待 LLM 生成完整段落（30-50字，约 2-3 秒），再将整段文本送给 TTS（约 1-2 秒），用户说完一句话后需要**等待 4-6 秒**才能听到声音，互动感完全断裂。

### 2.2 本项目的流式流水线优化机制
1. **Sentence Splitter 智能流式断句**：
   - LLM 逐 Token 输出时，断句器根据中英文强终止符（`。！？\n`）与弱停顿符（`，；,:`）结合最小长度动态切割。
   - 当生成第 1 个短句（例如“*你好呀！*”，仅需 4 个字、约 50-100ms）时，立即启动 TTS 合成！
2. **TTS 并行预取与流水线推流**：
   - 在播放第 1 句音频的同时，LLM 正在生成第 2 句，TTS 也已在后台合成第 2 句。
   - **首包音频响应时延（Time to First Audio, TTFA）可缩短至 300~800ms**，达到真人口语对话级别的流畅感。
3. **全双工打断（Barge-in / Interruption）**：
   - 用户在数字人说话过程中随时说话或点击打断，系统通过 `asyncio.Event` 立即丢弃当前正在合成与排队的音频队列，重置对话状态。

---

## 三、 模型选型（本地优先 + 云端统一抽象）

系统在 `src/config.py` 中提供了通用配置接口，可在本地离线与云端 API 之间自由切换：

### 3.1 LLM 大语言模型
- **本地优先推荐（默认支持）**：
  - **LM Studio**（已设为默认原生支持）：提供 OpenAI 兼容的 Local Server（端口 `1234`，URL `http://localhost:1234/v1`）。系统已实测适配本地加载的 `qwen/qwen3-vl-8b`、`qwen3.8-27b` 等模型，端到端首包时延实测约 **560ms**！
  - **Ollama**：支持 `qwen2.5:7b`、`llama3.2:3b` 等。
  - **vLLM / llama.cpp**：针对高并发或纯 C++ 场景。
- **云端兼容支持**：
  - 标准 OpenAI 兼容接口，可直连 DeepSeek-V3/R1、Qwen-Max、Claude 3.5 Sonnet、GPT-4o 等。
### 3.2 ASR 语音识别
- **本地优先推荐**：
  - **Sherpa-ONNX (SenseVoice-Small)**：阿里巴巴开源的极速多语言语音识别模型，体积仅约 200MB，支持富文本情感检测，推理速度比实时快 10 倍以上（RTF < 0.05）。
  - **Faster-Whisper / Whisper.cpp**：通用多语言识别。
- **云端备选**：Groq Whisper（云端 100ms 极速 ASR）。

### 3.3 TTS 语音合成
- **本地优先推荐**：
  - **macOS Say Native**（本项目已内置适配）：利用系统原生高质量女声（如 Ting-Ting），零网络依赖，毫秒级快速合成。
  - **Sherpa-ONNX (Kokoro / VITS)**：跨平台离线轻量神经网络语音合成。
  - **CosyVoice / GPT-SoVITS**（进阶音色克隆）：支持 3 秒声音样本克隆专属声线。
- **云端优质备选**：
  - **Edge-TTS**（本项目已内置适配）：微软免费高自然度神经语音（如 `zh-CN-XiaoxiaoNeural`, `zh-CN-YunxiNeural`），音质自然灵动。

---

## 四、 视频/画面驱动方案扩展设计（Phase 2-4）

系统已在 `src/lip_sync.py` 与 WebSocket 消息格式中设计好音视频联动的完整通信契约：

```json
{
  "type": "audio_chunk",
  "data": {
    "chunk_index": 1,
    "audio_base64": "...PCM/WAV...",
    "mouth_open": [0.0, 0.25, 0.65, 0.42, 0.10],
    "visemes": [
      {"A": 0.6, "I": 0.1, "U": 0.0, "E": 0.2, "O": 0.1}
    ]
  }
}
```

### 4.1 方案 A：Live2D 驱动实现（参考 Open-LLM-VTuber）
- 前端引入 `live2d.min.js` 或 `@pixi/display` 与 Live2D SDK。
- 将 `mouth_open` 帧直接赋值给模型的 `ParamMouthOpenY` 参数（范围 0.0 ~ 1.0）。
- 将 LLM 输出中的情绪标签（如 `[微笑]`、`[惊讶]`）映射至 Live2D 的 Motion/Expression 文件。

### 4.2 方案 B：3D VRM / 骨骼驱动
- 前端使用 `@pixiv/three-vrm` 加载 `.vrm` 模型。
- 使用 `visemes` 字典中的 A, I, U, E, O 映射到 BlendShape:
  - `currentVrm.expressionManager.setValue('aa', viseme.A);`
  - `currentVrm.expressionManager.setValue('ih', viseme.I);`

### 4.3 方案 C：真人/写实人脸视频流（MuseTalk / LivePortrait）
- 后端部署基于 ONNX / TensorRT 的口型视频推理服务。
- 输入音频 PCM 块，实时生成人脸视频帧并压入 WebRTC MediaStream，通过视频轨道推送给浏览器。

---

## 五、 快速开始与使用指南

### 5.1 环境要求
- 操作系统：macOS (Apple Silicon 推荐)、Linux、Windows
- Python: 3.10 ~ 3.12
- 依赖包管理：推荐使用 `uv`，也支持常规 `pip`

### 5.2 安装与启动
```bash
# 1. 克隆或进入项目目录
git clone https://github.com/feixiao/FastAvatar.git
cd FastAvatar
uv venv .venv --python 3.12
source .venv/bin/activate
uv pip install -e .

# 3. 启动服务（默认监听 http://0.0.0.0:8000）
uv run python -m src.server
```

打开浏览器访问 `http://localhost:8000`，即可进入网页端进行文本与麦克风实时语音交互！

### 5.3 一键模型下载与准备 (One-Click Setup)

项目内置自动化下载脚本 `download_models.sh`，支持交互式菜单与一键参数调用，自动下载解压离线 ASR、TTS 权重并自动完成模型路径绑定：

```bash
# 1. 运行一键脚本（提供交互式数字菜单，推荐）
./download_models.sh

# 或直接一键下载全部推荐模型
./download_models.sh all

# 或按需单独下载对应模块：
./download_models.sh asr   # 离线语音识别 SenseVoice-Small (~200MB)
./download_models.sh tts   # 离线语音合成 VITS Aishell3 (~50MB)
./download_models.sh llm   # 检查并拉取 Ollama / LM Studio Qwen2.5 模型
```

*(也可使用下方手动 CLI 步骤单独操作)*
#### 1. LLM 对话模型下载与加载

**方式 A：LM Studio CLI (`lms`)（推荐默认）**
```bash
# 1. 启动 LM Studio 本地服务端（监听 1234 端口）
lms server start

# 2. 命令行拉取模型（以 Qwen2.5 为例）
lms get qwen2.5-7b-instruct

# 3. 加载模型到 GPU/内存
lms load qwen2.5-7b-instruct --gpu max -y
```

**方式 B：Ollama CLI**
```bash
# 拉取并运行 Qwen2.5
ollama run qwen2.5:7b
```

#### 2. 离线 ASR 模型下载（Sherpa-ONNX SenseVoice-Small）
如果需要纯离线语音识别，使用命令行直接拉取解压模型（约 200MB）：
```bash
mkdir -p models
curl -L https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17.tar.bz2 | tar -xjf - -C models/
```

#### 3. 离线 TTS 模型下载（Sherpa-ONNX VITS / Kokoro）
如需离线神经网络语音合成，命令行拉取解压 VITS 中文模型（约 50MB）：
```bash
mkdir -p models
curl -L https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/vits-zh-aishell3.tar.bz2 | tar -xjf - -C models/
```
> *注：若使用本项目默认开箱即用的 **Edge-TTS**（微软神经语音）或 macOS 原生 **say**（如 `Ting-Ting`），则无需下载离线 TTS 权重。*

#### 4. 启动 FastAvatar 服务
```bash
uv run python -m src.server
```
打开浏览器访问 `http://localhost:8000` 即可开始交互。
---

## 六、 自动化评测与基准性能 (Autoresearch)

项目内置确定性性能基准评测脚手架，用于持续测试与优化流式流水线的交互时延：

```bash
./autoresearch.sh
```

**评测输出指标**：
- `turn_latency_ms`：端到端首包音频响应时延 (TTFA，用户停止说话到听到第一个音频块的时间，**越小越好**)
- `ttft_ms`：大模型首字返回时延 (Time to First Token)
- `tts_first_chunk_ms`：第一句音频合成耗时
- `total_duration_ms`：全轮对话总处理时间
