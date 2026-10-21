"""OpenAI-compatible cloud and local server provider (vLLM, LM-Studio, DeepSeek, OpenAI)."""

import json
from typing import AsyncIterator, List, Dict, Optional
import httpx
from src.llm.base import BaseLLM


class OpenAICompatibleLLM(BaseLLM):
    """Client for any OpenAI-compatible `/v1/chat/completions` endpoint."""

    def __init__(
        self,
        base_url: str = "https://api.openai.com/v1",
        api_key: Optional[str] = None,
        model: str = "gpt-4o-mini",
        timeout: float = 60.0,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key or "no-key-required"
        self.model = model
        self.timeout = timeout
        self._resolved_model: Optional[str] = None

    async def _get_active_model(self, client: httpx.AsyncClient) -> str:
        """Auto-detect active loaded model from local server if configured model not found."""
        if self._resolved_model:
            return self._resolved_model
        if "localhost" in self.base_url or "127.0.0.1" in self.base_url:
            try:
                resp = await client.get(f"{self.base_url}/models", timeout=2.0)
                if resp.status_code == 200:
                    data = resp.json().get("data", [])
                    all_ids = [m["id"] for m in data if "id" in m]
                    chat_models = [m_id for m_id in all_ids if "embed" not in m_id.lower()]
                    if self.model in all_ids:
                        self._resolved_model = self.model
                        return self._resolved_model
                    if chat_models:
                        self._resolved_model = chat_models[0]
                        return self._resolved_model
            except Exception:
                pass
        self._resolved_model = self.model
        return self._resolved_model
    async def chat_stream(
        self,
        messages: List[Dict[str, str]],
        system_prompt: Optional[str] = None,
        temperature: float = 0.7,
        max_tokens: int = 256,
    ) -> AsyncIterator[str]:
        payload_messages = []
        if system_prompt:
            payload_messages.append({"role": "system", "content": system_prompt})
        payload_messages.extend(messages)

        payload = {
            "model": self.model,
            "messages": payload_messages,
            "stream": True,
            "temperature": temperature,
            "max_tokens": max_tokens,
        }

        headers = {
            "Authorization": f"Bearer {self.api_key}",
            "Content-Type": "application/json",
        }

        async with httpx.AsyncClient(timeout=self.timeout) as client:
            active_model = await self._get_active_model(client)
            payload["model"] = active_model
            async with client.stream(
                "POST",
                f"{self.base_url}/chat/completions",
                json=payload,
                headers=headers,
            ) as response:
                response.raise_for_status()
                async for line in response.aiter_lines():
                    if not line:
                        continue
                    if line.startswith("data: "):
                        line_data = line[6:].strip()
                        if line_data == "[DONE]":
                            break
                        chunk = json.loads(line_data)
                        delta = chunk.get("choices", [{}])[0].get("delta", {})
                        token = delta.get("content", "")
                        if token:
                            yield token
