# FastAvatar Cloud Voice Agent Worker

基于 **LiveKit Agents** 与 **OpenAI 兼容推理引擎 (vLLM / MLX-LM / Ollama)** 的实时全双工语音智能体工作者。

---

## 架构职责与工作流

```text
[客户端 (Pixel / Mac)]
       │
       ▼ (WebRTC 音频轨 Opus/PCM)
[LiveKit SFU (端口: 7880 / 7882:udp)]
       │
       ▼ (实时媒体订阅)
[Agent Worker (worker.py)]
  ├── Silero VAD (超快端侧语义判停与插话打断 Barge-in)
  ├── 实时 STT (流式语音转文本)
  ├── OpenAI-compatible LLM (指向 Mac Studio 本地 MLX/vLLM 或云端集群)
  └── 实时流式 TTS (标点切句，首包 < 200ms)
       │
       ▼ (WebRTC 音频轨回推)
[客户端播放]
```

---

## 环境变量配置

| 变量名 | 默认值 | 作用说明 |
| :--- | :--- | :--- |
| `LIVEKIT_URL` | `ws://localhost:7880` | LiveKit SFU 服务端 WebSocket 地址 |
| `LIVEKIT_API_KEY` | `devkey` | LiveKit 鉴权 API Key |
| `LIVEKIT_API_SECRET` | `secret_avatar_livekit_key_2026` | LiveKit 鉴权 API Secret |
| `LLM_BASE_URL` | `http://127.0.0.1:8000/v1` | 上游大模型端点（vLLM / MLX-LM / Ollama） |
| `LLM_API_KEY` | `dummy-key` | 大模型 API Key |
| `LLM_MODEL` | `Qwen/Qwen2.5-7B-Instruct` | 挂载的基础或 LoRA 模型名称 |
| `AGENT_PERSONA` | `assistant` | 人设预设 (`assistant` / `jarvis` / `roaster` / `pet`) |

---

## 本地启动与调试 (Mac Studio / 开发者机器)

### 1. 准备 Python 虚拟环境与依赖
```bash
cd server/agent
python3 -m venv .venv
source .venv/bin/activate
pip install -r requirements.txt
```

### 2. 启动上游推理服务（以 MLX 或 Ollama 为例）
* **MLX-LM (Mac Apple Silicon 推荐)**:
  ```bash
  mlx_lm.server --model mlx-community/Qwen2.5-7B-Instruct-4bit --port 8000
  ```
* **Ollama**:
  ```bash
  ollama run qwen2.5:7b
  ```

### 3. 开发模式运行 Agent
```bash
python worker.py dev
```
Worker 会自动注册到本机的 LiveKit SFU 并进入待命监听状态。当客户端获得 Room Token 建立连接后，Worker 自动加入房间并开启实时全双工对话。
