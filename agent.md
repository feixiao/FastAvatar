# FastAvatar Engine — AI Agent 指南 (agent.md)

本文件是面向 AI Coding Agent 与开发者的项目工程指南，记录项目架构、核心设计约定、命令规范与开发注意事项。

---

## 一、 项目定位与核心架构

FastAvatar 是一套**低时延、模块化、本地大模型优先**的流式交互数字人引擎。设计目标是在消费级硬件（尤其是 Apple Silicon / M1 Pro 及以上设备）上实现 **< 800ms 端到端首包音频响应时延 (TTFA)**。

### 1.1 数据流架构
```
[麦克风/输入] ──▶ VAD (能量检测) ──▶ ASR (语音识别)
                                        │
                                        ▼
                                  LLM (流式 Token 生成)
                                        │
                                        ▼
                         SentenceSplitter (流式标点与语义断句)
                                        │ (分句队列)
                                        ▼
                               TTS (并行预取合成)
                                        │
                         LipSync (单 pass 提取口型与元音)
                                        │
[客户端 Web/Live2D] ◀── WebSocket (下发音频流 + Mouth Open + Visemes)
```

### 1.2 核心性能指标
- **TTFA (Time to First Audio / turn_latency_ms)**: 用户停止说话到听到第一个音频块的时间（目标 < 800ms）。
- **TTFT (Time to First Token)**: 大语言模型吐出首个 Token 的时延。
- **tts_first_chunk_ms**: 第一句文本完成 TTS 合成并返回 PCM 的耗时。

---

## 二、 代码目录与模块职责

```
FastAvatar/
├── src/
│   ├── config.py              # 配置中心（Pydantic），统一管理 LLM / ASR / TTS / Pipeline 配置
│   ├── pipeline.py            # 全双工流式流水线编排核心（调度各模块、处理打断、收集 metrics）
│   ├── sentence_splitter.py   # 流式断句器（零冗余分配、C级标点集合判定、数字保护）
│   ├── lip_sync.py            # 口型特征提取器（单 pass 计算 RMS 振幅与 A/I/U/E/O 频域能量）
│   ├── server.py              # FastAPI HTTP + WebSocket 服务端（音频/打断事件通信）
│   ├── companion.py           # 养成系成长与记忆引擎（亲密度、等级、记忆抽取、时间感知）
│   ├── vad/                   # 语音活动检测（EnergyVAD）
│   ├── asr/                   # ASR 抽象与实现（MockASR, SherpaOnnxASR）
│   ├── tts/                   # TTS 抽象与实现（MockTTS, EdgeTTS, MacSayTTS, SherpaOnnxTTS, FallbackTTS）
│   ├── llm/                   # LLM 抽象与实现（MockLLM, OllamaLLM, OpenAICompatibleLLM）
│   └── web/                   # 默认前端交互界面（HTML5 Audio + Canvas 口型可视化）
├── tests/                     # 单元与集成测试（pytest）
├── benchmarks/                # 确定性基准性能测试（run_benchmark.py）
├── autoresearch.sh            # 自动化基准评测脚手架（测试 + 评测输出）
├── pyproject.toml             # 项目元数据与依赖定义（基于 uv）
└── README.md                  # 用户与架构说明文档
```

---

## 三、 常用开发与测试命令

所有命令优先使用 `uv` 运行：

```bash
# 1. 安装/同步依赖
uv sync

# 2. 一键下载离线模型 (ASR/TTS/LLM)
./download_models.sh

# 3. 运行全部单元测试
uv run --extra dev pytest

# 4. 运行单个测试文件
uv run --extra dev pytest tests/test_sentence_splitter.py
uv run --extra dev pytest tests/test_pipeline.py

# 5. 运行确定性延迟基准测试 (Autoresearch)
./autoresearch.sh
# 或直接运行评测脚本：
uv run python benchmarks/run_benchmark.py

# 6. 启动本地服务 (默认监听 http://0.0.0.0:8000)
uv run python -m src.server
```

> **系统依赖要求**：macOS 下使用原生语音合成（`MacSayTTS`）需安装 `ffmpeg`：`brew install ffmpeg`。

---

## 四、 核心设计底线与工程规范 (Engineering Invariants)

编写或重构代码时，必须遵守以下约定：

1. **时延至上（Latency-Critical）**：
   - 严禁在 LLM 生成过程中引入全量等待（如等整段话生成完再送 TTS）。
   - `SentenceSplitter` 首句阈值必须保持小粒度（默认 `min_chunk_length = 5~6`），确保首句在 50~100ms 内切割并启动 TTS。
   - 避免流式循环内部的高频大内存分配；频繁查询的字符集使用预初始化 `Set` 和 C 级优化方法（如 `set.isdisjoint`）。

2. **全双工与打断机制 (Barge-in)**：
   - 用户说话或触发中断时，必须通过 `asyncio.Event` 立即丢弃当前正在排队或合成的音频。
   - WebSocket 协议中打断信号为 `{"type": "interrupt"}`，后端立即停止下发并广播 `interrupted` 事件。

3. **双重降级与健壮性 (Robust Fallback)**：
   - 云端语音（如 `EdgeTTS`）发生网络抖动或超时时，管道已配置 `FallbackTTS` 自动降级为 macOS 原生 `MacSayTTS` 或 `MockTTS`，保证对话流永不中断。

4. **口型与画面驱动契约 (Lip Sync Contract)**：
   - 每个下发的 `audio_chunk` 必须携带对应长度的 `mouth_open`（RMS 归一化能量）和 `visemes`（A/I/U/E/O 字典）。
   - `extract_lip_sync` 计算必须保持在单个 pass 完成，杜绝重复计算 FFT。

5. **Apple Silicon / M1 Pro 推荐配置**：
   - **LLM**：LM Studio 托管 `Qwen2.5-7B-Instruct-GGUF`（GPU Offload: MAX，监听 `1234` 端口）。
   - **TTS**：日常优先使用 `EdgeTTS`（晓晓/云希）或 macOS 原生 `MacSayTTS`（Ting-Ting）。
   - **ASR**：离线测试推荐 `Sherpa-ONNX` (SenseVoice-Small)，单核 RTF < 0.05。
