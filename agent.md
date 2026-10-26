# Avatar Engine — AI Agent 指南 (agent.md)

本文件是面向 AI Coding Agent 与开发者的项目工程指南，记录项目架构、核心设计约定与开发规范。

---

## 一、 项目定位与技术栈

- **定位**：桌面端轻量级极简语音伴侣，单二进制，零外部重型运行时依赖。
- **架构**：Go + Wails (v2) + 原生 Web Audio (AEC 回声消除)。
- **核心流**：用户说话 ➔ Web Audio VAD ➔ 状态流转 (Listening) ➔ LLM SSE 流式生成 (Thinking) ➔ SentenceSplitter 毫秒级标点断句 ➔ 并行 TTS 合成 (Speaking) ➔ 实时打断机制 (Barge-in)。

---

## 二、 工程规范与代码约定

1. **时延优先 (Latency-Critical)**：
   - 严禁在 LLM 生成过程中全量缓冲，首句阈值保持在 4~6 字立即切分进入发音队列。
   - `SentenceSplitter` 在中英文强弱标点上精准切割，同时保护浮点数（如 `3.14`）不被截断。

2. **打断与并发控制 (Barge-in)**：
   - 使用 Go 标准库 `context.WithCancel` 与 Channel 调度，打断信号一触发，立即 `cancel()` 掐断 HTTP 流并终止 TTS 播放进程。

3. **双轨模型路由 (Local-First)**：
   - 优先通过 `sidecar/llama_manager.go` 探测或拉起本地端侧小模型（1.5B ~ 3B）。
   - 探测失败自动平滑退避到云端兼容 API。

---

## 三、 常用命令

```bash
# 1. 运行所有单元测试
go test -v ./...

# 2. 编译并打包 macOS 应用
./package_mac.sh

# 3. 运行单个模块测试
go test -v ./internal/engine
go test -v ./internal/llm
go test -v ./internal/tts
```
