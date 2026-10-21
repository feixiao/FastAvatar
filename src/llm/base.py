"""LLM provider base interface."""

from abc import ABC, abstractmethod
from typing import AsyncIterator, List, Dict, Any, Optional


class BaseLLM(ABC):
    """Abstract interface for LLM chat engines with streaming output."""

    @abstractmethod
    async def chat_stream(
        self,
        messages: List[Dict[str, str]],
        system_prompt: Optional[str] = None,
        temperature: float = 0.7,
        max_tokens: int = 256,
    ) -> AsyncIterator[str]:
        """Stream response tokens one by one."""
        pass
