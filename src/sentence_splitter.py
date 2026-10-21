"""Streaming Sentence Splitter for real-time TTS pipelining.

Divides incoming token stream into speakable clauses as soon as punctuation
boundaries are met, minimizing Time to First Audio (TTFA).
"""

import re
from typing import AsyncIterator, List, Set

MARKDOWN_SYMBOLS = set("*_~`#")
MARKDOWN_CLEAN_RE = re.compile(r"[*_~`#]")
WHITESPACE_CLEAN_RE = re.compile(r"\s+")


def clean_speech_text(text: str) -> str:
    """Strips markdown asterisks, bold/italic markers, and hashes for clean TTS reading."""
    if MARKDOWN_SYMBOLS.isdisjoint(text):
        return text.strip()
    cleaned = MARKDOWN_CLEAN_RE.sub("", text)
    cleaned = WHITESPACE_CLEAN_RE.sub(" ", cleaned).strip()
    return cleaned

class SentenceSplitter:
    """Buffers incoming tokens and emits speakable text chunks based on punctuation and length."""

    def __init__(
        self,
        min_length: int = 5,
        max_length: int = 35,
        split_punctuations: str = "，。！？；、\n\r,!?;:",
    ):
        self.min_length = min_length
        self.max_length = max_length
        self.split_chars: Set[str] = set(split_punctuations)
        self.strong_enders: Set[str] = set("。！？\n!?")
        self.all_punctuations: Set[str] = self.split_chars | self.strong_enders
        self.buffer = ""

    def reset(self):
        """Reset buffer state between conversational turns."""
        self.buffer = ""
    def process_token(self, token: str) -> List[str]:
        """Add a token to the buffer and return any completed chunks."""
        self.buffer += token
        buf_len = len(self.buffer)

        # Fast path: token contains no punctuation and buffer has not exceeded max_length
        if buf_len < self.max_length and self.all_punctuations.isdisjoint(token):
            return []

        emitted: List[str] = []

        while self.buffer:
            buf_len = len(self.buffer)
            split_idx = -1
            # 1. Check strong enders (。！？\n) which make complete thoughts even if short (>= 2 chars)
            for idx, ch in enumerate(self.buffer):
                if ch in self.strong_enders and idx >= 1:
                    split_idx = idx
                    break
                elif ch in self.split_chars and (idx + 1) >= self.min_length:
                    # Decimal protection (e.g., 3.14)
                    if (
                        ch == "."
                        and idx > 0
                        and idx < buf_len - 1
                        and self.buffer[idx - 1].isdigit()
                        and self.buffer[idx + 1].isdigit()
                    ):
                        continue
                    split_idx = idx
                    break

            # 2. If buffer exceeds max_length and no punctuation, force split at max_length
            if split_idx == -1 and buf_len >= self.max_length:
                split_idx = self.max_length - 1

            if split_idx != -1:
                raw_chunk = self.buffer[: split_idx + 1].strip()
                self.buffer = self.buffer[split_idx + 1 :].lstrip()
                chunk = clean_speech_text(raw_chunk)
                if chunk:
                    emitted.append(chunk)
            else:
                break

        return emitted

    def flush(self) -> List[str]:
        """Flush remaining buffer at the end of LLM stream."""
        remaining = clean_speech_text(self.buffer)
        self.buffer = ""
        if remaining:
            return [remaining]
        return []

    async def split_stream(self, token_stream: AsyncIterator[str]) -> AsyncIterator[str]:
        """Convenience async generator wrapping an async token iterator."""
        async for token in token_stream:
            chunks = self.process_token(token)
            for chunk in chunks:
                yield chunk
        for chunk in self.flush():
            yield chunk
