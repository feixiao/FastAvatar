FROM python:3.11-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends \
    build-essential \
    ffmpeg \
    curl \
    && rm -rf /var/lib/apt/lists/*

COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

COPY worker.py .

ENV PYTHONUNBUFFERED=1
ENV LIVEKIT_URL=ws://livekit:7880
ENV LIVEKIT_API_KEY=devkey
ENV LIVEKIT_API_SECRET=secret_avatar_livekit_key_2026
ENV LLM_BASE_URL=http://host.docker.internal:8000/v1
ENV LLM_MODEL=Qwen/Qwen2.5-7B-Instruct
ENV AGENT_PERSONA=assistant

CMD ["python", "worker.py", "start"]
