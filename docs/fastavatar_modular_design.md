# FastAvatar 模块化设计方案

## 1. 目标与范围

本文将 [端侧 AI 语音助手架构](on_device_ai_voice_assistant_architecture.md) 映射到 FastAvatar 当前 Go + Wails 项目，定义模块边界、依赖方向、接口契约和从现有代码演进的顺序。

这是设计方案，不表示其中所有模块和接口均已实现。初期保持单进程，通过进程内接口和 Go `context` 协作；不为模块化引入服务化、IPC 或消息总线。设计以当前桌面应用的真实能力为首期范围，为后续能力留出接口边界。

## 2. 分层与依赖方向

```text
Wails / 前端适配层
       │ 用户输入、播放控制、状态展示
       ▼
应用编排层（Session / Turn / Task Coordinator）
       ├── 前台交互：输入适配、轮次识别、快路径、播报与打断
       └── 后台执行：上下文、模型、工具计划、执行与结果校验
                │             │             │
                ▼             ▼             ▼
            模型端口       工具端口       记忆/人格端口
                │             │             │
                └────── 平台适配与持久化 ─────┘
```

依赖只向抽象端口流动：编排层定义用例和领域数据，模型、音频、工具、持久化等适配器实现端口。领域模块不依赖 Wails，也不直接启动外部命令或读写前端状态。`main.go` / `NewApp` 负责组合实现并注入依赖；Wails 绑定只调用应用用例，不持有业务流程。

## 3. 模块职责与代码映射

| 模块 | 职责与边界 | 当前代码映射 / 规划位置 |
| :--- | :--- | :--- |
| 应用装配与 UI 适配 | 应用生命周期、依赖装配、Wails 方法绑定；把前端动作转换为应用命令并发布 UI 状态 | `main.go`、`app.go`；保持为薄适配层 |
| 会话与轮次协调器 | 管理 session/turn ID、状态转换、活动任务、取消、迟到事件过滤；不做模型推理与音频处理 | 从 `internal/engine.Pipeline` 拆出协调职责，归入 `internal/session` 或 `internal/orchestrator`，实施时统一命名 |
| 前台语音交互 | 接收音频/文本输入事件，汇总部分识别、EOT、快路径决策，管理播报队列和 barge-in | 新增 `internal/interaction`；首阶段由当前前端 Web Audio VAD 与 Go 文本入口提供输入，不要求先实现设备级 KWS/ASR |
| 音频输入与感知适配 | 麦克风、AEC、VAD、ASR、时间戳与文本对齐的适配接口；不拥有会话任务状态 | 新增 `internal/audio` 端口；当前 Web Audio 能力通过 Wails 前端适配，平台原生实现按需接入 |
| 后台任务执行器 | 接收稳定用户目标，组装上下文、调用模型、处理流式输出、协调工具与最终答复 | 从 `internal/engine.Pipeline.Chat` 拆出后台执行用例；复用 `internal/llm`、`internal/engine` 的句子切分能力，逐步缩小 Pipeline |
| 模型路由与推理适配 | 统一文本生成/流式生成端口；实现本地优先、云端回退及取消传播 | `internal/llm` 作为 provider 适配；`internal/sidecar` 仅管理本地推理进程，不决定会话流程 |
| 工具目录与执行器 | 描述工具 Schema、权限级别、确认策略、幂等性和超时；校验后通过白名单适配器执行 | 新增 `internal/tools`；先接入真实且已授权的应用能力，未接入的工具不得以占位实现对外暴露 |
| 上下文与记忆 | 组装对话上下文，读取/写入长期记忆，处理来源与删除策略 | 复用 `internal/memory`、`internal/persona`；通过端口供后台用例调用，不把存储细节泄露给模型接口 |
| 语音输出适配 | 文本分块、合成、播放、停止，并报告播放确认边界 | `internal/tts` 提供合成端口；播放状态/实际已播文本锚点由交互层持有，补齐回调契约 |
| 配置与策略 | 配置加载、默认值、端云路由偏好及风险策略；不在模型输出中动态放宽权限 | 复用 `internal/config`，逐步将策略配置与普通模型配置区分 |

表中新增目录是目标边界，不要求一次性创建所有空包。优先按可验证的职责拆分已有 `Pipeline`，只有存在真实调用方时才引入新模块。`internal/engine.SentenceSplitter` 是流式文本处理组件，可保留在 engine 或迁入专门的输出模块；最终位置以单一职责和现有调用关系为准。

## 4. 核心接口契约

以下为接口语义草案，具体 Go 类型在实施时由相邻模块共同定义；事件应携带 `SessionID`、`TurnID`、`TaskID` 和递增序号，避免以全局状态或隐式 goroutine 顺序关联请求。

```go
type Interaction interface {
    Submit(ctx context.Context, input UserInput) (TurnID, error)
    Interrupt(ctx context.Context, sessionID SessionID, turnID TurnID) error
}

type TaskRunner interface {
    Run(ctx context.Context, task Task, emit func(TaskEvent)) error
}

type Model interface {
    Stream(ctx context.Context, request ModelRequest) (<-chan ModelEvent, error)
}

type ToolExecutor interface {
    Execute(ctx context.Context, call ToolCall, idempotencyKey string) (ToolResult, error)
}

type SpeechOutput interface {
    Speak(ctx context.Context, text string) (Playback, error)
    Stop(ctx context.Context) error
}
```

契约约束：

- 每个 `Submit` 创建新的轮次标识；同一轮次的增量输入只更新该轮任务，不得误并入下一轮。
- `Interrupt` 必须快速停止播放并取消可取消的生成；已提交的外部副作用返回明确状态，不能假报撤销成功。
- `TaskRunner` 只通过事件报告进度、待确认、完成、失败或取消；所有事件需关联任务 ID，终态之后的事件忽略或归档。
- `Model` 输出候选文本或结构化调用，不持有工具执行权限；模型错误、取消和通道关闭都必须可区分。
- `ToolExecutor` 在执行前验证 Schema、权限、幂等键和必要确认；返回结构化结果及错误类别供任务用例核验。
- `SpeechOutput` 或其播放句柄报告已确认播放的文本片段/时间边界，打断时据此裁剪对话历史；若平台无法精确对齐，报告最小可靠边界。
- 长期记忆写入与删除是显式用例；模型提出的记忆内容只是候选，经过策略处理后才能持久化。

这些接口属于模块间的进程内契约。跨进程 IPC、网络协议和事件总线仅在实际部署要求出现后定义，避免过早固定传输层。

## 5. 关键数据对象

- `UserInput`：输入类型（语音/文本）、文本或音频句柄、时间范围、说话人线索、来源权限与稳定性标记。
- `Turn`：轮次 ID、用户目标、识别修订号、当前阶段及与会话关联的信息。
- `Task`：任务 ID、轮次 ID、目标快照、上下文引用、截止时间、取消信号及风险上下文。
- `TaskEvent`：序号、任务 ID、状态、可选进度、待确认摘要、最终结果或结构化错误。
- `ToolCall` / `ToolResult`：工具标识与版本、Schema 化参数、幂等键、权限决策、执行状态及来源可追溯的结果。
- `Playback`：播放句柄、分块文本锚点、实际播放进度与终止原因。

ID 在单个会话生命周期内唯一且不复用；事件序号在任务内单调递增。大载荷通过带过期策略的句柄引用，避免在事件队列中复制音频数据。

## 6. 从现有 Pipeline 拆分

当前 `internal/engine.Pipeline` 同时持有模型客户端、TTS、句子切分、记忆、人格、对话历史、状态回调和取消上下文。模块化改造建议保留其外部可见的会话行为，按以下顺序迁移职责：

1. 抽取会话协调器，集中管理状态、当前 turn/task、取消和过期事件；让 `Pipeline` 暂时委托协调器，避免一次性改动所有调用方。
2. 抽取后台响应用例，接收已定稿输入及上下文快照，调用模型并输出文本事件；将记忆/人格提示词组装明确为上下文提供者。
3. 抽取播报会话，消费文本块、执行 TTS、上报播放进度；由交互层统一处理停止与历史截断。
4. 抽取快路径处理器，限定处理确定的记忆/人格直控命令及后续明确接入的安全命令；处理结果统一走状态/播报契约。
5. 在所有调用方迁移完成后，删除承载重复流程的旧 `Pipeline` 逻辑，避免长期保留两套状态机。

第一阶段不加入尚无产品接口支撑的日历、车控、视觉、多音区或 KWS/ASR 实现。模块边界预留扩展点，落地范围以当前桌面应用真实能力为准。
