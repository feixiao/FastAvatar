# Avatar Engine — AI Agent 指南 (agent.md)

本文件是面向 AI Coding Agent 与开发者的项目工程指南，记录项目架构、核心设计约定与开发规范。

---

## 一、 项目定位与技术栈

- **定位**：桌面端轻量级极简语音伴侣，单二进制，零外部重型运行时依赖。
- **架构**：Go + Wails (v2) + 原生 Web Audio (AEC 回声消除)。
- **核心流**：用户说话 ➔ Web Audio VAD ➔ 状态流转 (Listening) ➔ LLM SSE 流式生成 (Thinking) ➔ SentenceSplitter 毫秒级标点断句 ➔ 并行 TTS 合成 (Speaking) ➔ 实时打断机制 (Barge-in)。

---

## 二、 生产级工程规范 (Production-Grade Engineering Invariant)

本项目严禁任何 Demo 级/玩具级的粗暴临时补丁（如硬编码字数阈值、单字粗暴包含、无状态盲切）。所有方案必须达到工业级生产可用标准：

1. **端云分流严禁 Demo 级启发式规则 (Contextual Intent Routing)**：
   - **严禁依赖文本长度阈值**（如 `len > 25`）：字数无法衡量认知复杂度（30 字的日常闲聊应留端侧，5 字的高数推理或代码追问必须上云）。
   - **单字全字严格防误触**：单汉字指令（如“停”、“坐”、“跳”）严禁使用模糊子串搜索，必须全词精准匹配，杜绝“暂停播放”被“停”误拦截。
   - **多轮会话状态感知 (DST: Dialogue State Tracking)**：路由必须继承上轮会话状态。若上一轮在云端进行复杂代码或专业推演，后续短代词指代（如“为什么”、“第三行呢”、“换个写法”）必须强制继承云端，严禁无状态盲切回本地导致上下文精神分裂。
   - **意图特征分类与分级判定**：优先通过语义分类器或领域槽位（数学、编程、深度推理 vs 日常问候、情绪安抚）判定复杂度，而非简单的 `strings.Contains` 扫词。

2. **时延优先与硬实时指标 (Hard Latency SLA)**：
   - **端侧指令仲裁**：$\le 10\mu s$（C++ 纳秒级查表与 `std::string_view`，零堆内存分配）。
   - **首句发音时延 (TTFT)**：$\le 150\text{ms}$，严禁 LLM 全量缓冲，首句阈值保持在 4~6 字立即原子切分发音。
   - **声学打断 (Barge-in)**：$\le 50\text{ms}$，打断一经触发，立即执行硬件级闭音、清空音频环形缓冲与大模型 KV Cache 截断回滚。

3. **零幻觉与槽位追问 (Zero Hallucination)**：
   - 关键参数缺失（如闹钟时间、联系人电话号码）时，严禁私自脑补虚假默认参数，必须明确标记 `SLOT_MISSING` 走追问澄清链路。

4. **容灾熔断与平滑降级 (Graceful Degradation)**：
   - 云端实时网关断网或超时，必须在 $\le 50\text{ms}$ 内无缝自动降级至端侧伴生小模型；
   - 端侧模型显存溢出或异常时，平滑溢出至云端大模型，全过程对用户发音体验无感知。

---

## 三、 核心语音处理模块 (core/) 架构准则：高内聚与低耦合

`core/` 目录为 C++17 跨平台全双工语音交互引擎底座 (`libaudio_agent`)，必须严格遵守**高内聚、低耦合**架构规范：

1. **高内聚边界 (In-Scope · 语音处理核心职责)**：
   - **全链路声学流闭环**：麦克风 PCM 帧采集、LockFree 环形缓冲、VAD 端点检测、流式 ASR/TTS、$\le 50\text{ms}$ 硬件级插话打断瞬断（Barge-in）与 KV Cache 截断回滚。
   - **确定性口语指令仲裁 (`IntentArbiter`)**：端侧 $\le 10\mu s$ 极速嗅探强控制指令（动作指令 `ActionCommand`、系统硬件工具 `ToolCall`、记忆 `MemoryCommand`、人设 `PersonaSwitch`），零堆内存分配。
   - **流式认知中枢与标签剥离 (`StreamingSentenceChunker`)**：4~6B 首句微切分，流式消费中原子剥离 `[ACTION:xxx]` 动作标签并即时派发事件，净化 TTS 发声文本，杜绝读出 Markdown 与系统标记。
   - **全双工状态机 (FSM)**：统一闭环驱动 `IDLE` $\leftrightarrow$ `LISTENING` $\leftrightarrow$ `THINKING` $\leftrightarrow$ `SPEAKING` 状态机流转。

2. **低耦合边界 (Out-of-Scope · 严禁侵入核心库)**：
   - **零角色与物种概念**：核心库只处理动作元语，严禁在 C++ 内部硬编码任何具体角色名称（如罗小黑、大肥鱼）、物种习性（小鱼干、猫咪摇尾）或特定角色台词。
   - **零表现层资产感知**：核心库严禁感知外部皮肤文件（如 JSON 资产清单、WebP 图集、帧率网格）。
   - **零应用层数值与 UI 业务**：严禁侵入好感度、亲密度经验值（XP）、手账中枢、桌面原生窗口坐标计算。
   - **极简正交契约**：动作指令仅输出抽象元语（如 `ActionType::SIT`、`ActionType::JUMP`、`MotionVector { direction, distance }`），角色台词插值、动画帧渲染与物理位移 100% 交由宿主层（macOS Canvas、Android Compose、硬件电机）自行处理。

3. **单一决策源 (Single Source of Truth)**：
   - 意图嗅探与指令分流规则统一收敛于 C++ `IntentArbiter`；
   - 宿主端（macOS Go CGO / Android Kotlin JNI）纯作为消费者接收决策事件，禁止在双端宿主各自重复编写匹配规则导致代码漂移。

---

## 四、 常用命令

```bash
# 1. 运行 Go 全量单元测试
go test -v ./...

# 2. 运行 C++ 核心库单元与端到端测试
make native-test

# 3. 运行 Android 核心单元测试
make android-test

# 4. 编译并打包 macOS 应用
./package_mac.sh

# 5. 运行指定 Go 模块测试
go test -v ./internal/dispatcher
go test -v ./internal/interaction
go test -v ./internal/engine
go test -v ./internal/task
```
