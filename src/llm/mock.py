"""Deterministic Mock LLM for local testing and reproducible benchmarks."""

import asyncio
from typing import AsyncIterator, List, Dict, Optional
from src.llm.base import BaseLLM


class MockLLM(BaseLLM):
    """Generates deterministic token stream for testing without external services."""

    def __init__(
        self,
        default_reply: str = "主人你好！我是你的语音数字人助手，今天过得开心吗？有什么想和我聊聊的呀？",
        initial_delay_ms: float = 30.0,
        token_interval_ms: float = 8.0,
    ):
        self.default_reply = default_reply
        self.initial_delay_ms = initial_delay_ms
        self.token_interval_ms = token_interval_ms

    async def chat_stream(
        self,
        messages: List[Dict[str, str]],
        system_prompt: Optional[str] = None,
        temperature: float = 0.7,
        max_tokens: int = 256,
    ) -> AsyncIterator[str]:
        # Simulate Time to First Token (TTFT)
        if self.initial_delay_ms > 0:
            await asyncio.sleep(self.initial_delay_ms / 1000.0)

        # Tokenize by small units (1-3 characters) to mimic real LLM token streaming
        text = self.default_reply
        i = 0
        while i < len(text):
            step = 2 if i + 2 <= len(text) else 1
            token = text[i : i + step]
            i += step
            yield token
            if self.token_interval_ms > 0:
                await asyncio.sleep(self.token_interval_ms / 1000.0)
