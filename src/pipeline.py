"""Core streaming conversation pipeline for Voice Digital Avatar."""

import asyncio
import time
from typing import AsyncIterator, Dict, Any, List, Optional, Tuple
from pydantic import BaseModel

from src.config import PipelineConfig, LLMProviderType, ASRProviderType, TTSProviderType
from src.sentence_splitter import SentenceSplitter
from src.lip_sync import LipSyncExtractor
from src.asr.base import BaseASR
from src.asr.mock import MockASR
from src.asr.sherpa import SherpaOnnxASR
from src.llm.base import BaseLLM
from src.llm.mock import MockLLM
from src.llm.ollama import OllamaLLM
from src.llm.openai_compatible import OpenAICompatibleLLM
from src.tts.base import BaseTTS, FallbackTTS
from src.tts.mock import MockTTS
from src.tts.mac_say import MacSayTTS
from src.tts.edge import EdgeTTS
from src.tts.sherpa import SherpaOnnxTTS
from src.companion import CompanionManager

class PipelineEvent(BaseModel):
    event_type: str  # "text_chunk", "audio_chunk", "viseme", "metrics", "interrupted"
    payload: Dict[str, Any]


class PipelineMetrics(BaseModel):
    turn_latency_ms: float = 0.0          # Time to first audio packet (TTFA)
    ttft_ms: float = 0.0                  # Time to first LLM token
    asr_latency_ms: float = 0.0           # ASR duration
    tts_first_chunk_ms: float = 0.0       # Duration to synthesize chunk 1
    total_audio_bytes: int = 0
    chunks_count: int = 0
    full_text: str = ""


class VoiceAvatarPipeline:
    """Orchestrates streaming ASR -> LLM -> SentenceSplitter -> TTS -> LipSync."""

    def __init__(self, config: Optional[PipelineConfig] = None):
        self.config = config or PipelineConfig()
        self.asr = self._create_asr()
        self.llm = self._create_llm()
        self.tts = self._create_tts()
        self.lip_sync = LipSyncExtractor(sample_rate=self.config.sample_rate)
        self.splitter = SentenceSplitter(
            min_length=self.config.min_chunk_length,
            max_length=self.config.max_chunk_length,
            split_punctuations=self.config.split_punctuations,
        )
        self.companion = CompanionManager()
        self.chat_history: List[Dict[str, str]] = []
        self._interrupt_event = asyncio.Event()
    def _create_asr(self) -> BaseASR:
        cfg = self.config.asr
        if cfg.provider == ASRProviderType.SHERPA_ONNX:
            return SherpaOnnxASR()
        # Auto-detect if offline SherpaOnnx model was downloaded
        from src.asr.sherpa import _find_default_sense_voice
        m_path, _ = _find_default_sense_voice()
        if m_path is not None:
            return SherpaOnnxASR()
        return MockASR()

    def clear_history(self):
        """Clear conversation history for fresh continuous chatting."""
        self.chat_history.clear()
    def _create_llm(self) -> BaseLLM:
        cfg = self.config.llm
        if cfg.provider == LLMProviderType.OLLAMA:
            return OllamaLLM(model=cfg.model, base_url=cfg.base_url)
        elif cfg.provider in (LLMProviderType.OPENAI_COMPATIBLE, LLMProviderType.LM_STUDIO):
            return OpenAICompatibleLLM(
                base_url=cfg.base_url or "http://localhost:1234/v1",
                api_key=cfg.api_key or "lm-studio",
                model=cfg.model,
            )
        return MockLLM()

    def _create_tts(self) -> BaseTTS:
        import sys
        cfg = self.config.tts
        if cfg.provider == TTSProviderType.EDGE_TTS:
            edge = EdgeTTS(voice=cfg.voice)
            fallback = MacSayTTS(voice="Ting-Ting") if sys.platform == "darwin" else MockTTS(sample_rate=self.config.sample_rate)
            return FallbackTTS(primary=edge, secondary=fallback)
        elif cfg.provider == TTSProviderType.MAC_SAY:
            return MacSayTTS(voice=cfg.voice)
        elif cfg.provider == TTSProviderType.SHERPA_ONNX:
            return SherpaOnnxTTS()
        return MockTTS(sample_rate=self.config.sample_rate)

    def trigger_interrupt(self):
        """Immediately abort active speaking / generation."""
        if self.config.enable_barge_in:
            self._interrupt_event.set()

    def reset_interrupt(self):
        self._interrupt_event.clear()

    async def process_user_turn(
        self,
        audio_input: Optional[bytes] = None,
        text_input: Optional[str] = None,
        reset_interrupt: bool = True,
    ) -> AsyncIterator[PipelineEvent]:
        """Execute a full conversational turn with streaming output and detailed latency metrics."""
        if reset_interrupt:
            self.reset_interrupt()
        if self._interrupt_event.is_set():
            yield PipelineEvent(event_type="interrupted", payload={"reason": "pre_interrupted"})
            return

        metrics = PipelineMetrics()

        self.splitter.reset()
        splitter = self.splitter
        # Step 1: Speech-to-Text (ASR) if audio given
        user_text = text_input or ""
        turn_start_time = time.perf_counter()
        if audio_input and not user_text:
            user_text = await self.asr.transcribe(audio_input, sample_rate=self.config.sample_rate)
            metrics.asr_latency_ms = (time.perf_counter() - turn_start_time) * 1000.0
        if not user_text.strip():
            return

        # Emit user transcription event so frontend sees the transcribed voice immediately
        yield PipelineEvent(
            event_type="user_transcription",
            payload={"text": user_text, "asr_latency_ms": metrics.asr_latency_ms},
        )

        self.chat_history.append({"role": "user", "content": user_text})
        max_hist = getattr(self.config, "max_history_messages", 20)
        if len(self.chat_history) > max_hist:
            self.chat_history = self.chat_history[-max_hist:]

        full_reply = ""
        first_token_received = False
        first_audio_produced = False
        tts_first_chunk_start = 0.0

        chunk_queue: asyncio.Queue[Optional[Tuple[str, asyncio.Task[bytes]]]] = asyncio.Queue()
        pending_tts_tasks: List[asyncio.Task] = []

        def dispatch_chunk(text: str):
            nonlocal tts_first_chunk_start
            clean_t = text.strip()
            if not clean_t or self._interrupt_event.is_set():
                return
            if tts_first_chunk_start == 0.0:
                tts_first_chunk_start = time.perf_counter()
            tts_task = asyncio.create_task(self.tts.synthesize(text))
            pending_tts_tasks.append(tts_task)
            chunk_queue.put_nowait((text, tts_task))

        async def llm_producer():
            nonlocal first_token_received, full_reply
            try:
                dyn_system_prompt = self.companion.build_system_prompt(self.config.llm.system_prompt)
                stream = self.llm.chat_stream(
                    messages=self.chat_history,
                    system_prompt=dyn_system_prompt,
                    temperature=self.config.llm.temperature,
                    max_tokens=self.config.llm.max_tokens,
                )
                async for token in stream:
                    if self._interrupt_event.is_set():
                        break
                    if not first_token_received:
                        first_token_received = True
                        metrics.ttft_ms = (time.perf_counter() - turn_start_time) * 1000.0
                    full_reply += token
                    chunks = splitter.process_token(token)
                    for c in chunks:
                        dispatch_chunk(c)

                if not self._interrupt_event.is_set():
                    for c in splitter.flush():
                        dispatch_chunk(c)
            except Exception as exc:
                print(f"[VoiceAvatarPipeline] LLM Stream error: {exc}")
                fallback_msg = f"（连接大模型服务失败: {exc}。请检查本地 LM Studio 或 Ollama 服务是否已开启并监听对应端口）"
                full_reply += fallback_msg
                for c in splitter.process_token(fallback_msg):
                    dispatch_chunk(c)
                for c in splitter.flush():
                    dispatch_chunk(c)
            finally:
                chunk_queue.put_nowait(None)

        producer_task = asyncio.create_task(llm_producer())

        try:
            while True:
                if self._interrupt_event.is_set():
                    yield PipelineEvent(event_type="interrupted", payload={"reason": "user_barge_in"})
                    break

                item = await chunk_queue.get()
                if item is None:
                    break

                sentence_chunk, tts_task = item

                metrics.chunks_count += 1
                yield PipelineEvent(
                    event_type="text_chunk",
                    payload={"chunk_index": metrics.chunks_count, "text": sentence_chunk},
                )

                audio_data = await tts_task
                if self._interrupt_event.is_set():
                    yield PipelineEvent(event_type="interrupted", payload={"reason": "user_barge_in"})
                    break

                if not first_audio_produced:
                    first_audio_produced = True
                    metrics.tts_first_chunk_ms = (time.perf_counter() - tts_first_chunk_start) * 1000.0
                    metrics.turn_latency_ms = (time.perf_counter() - turn_start_time) * 1000.0

                metrics.total_audio_bytes += len(audio_data)

                # Extract lip-sync visemes and emit audio in a single pass
                rms_frames, visemes = self.lip_sync.extract_lip_sync(audio_data)

                yield PipelineEvent(
                    event_type="audio_chunk",
                    payload={
                        "chunk_index": metrics.chunks_count,
                        "audio_bytes_len": len(audio_data),
                        "audio_data": audio_data,
                        "mouth_open": rms_frames,
                        "visemes": visemes,
                    },
                )
        finally:
            for t_task in pending_tts_tasks:
                if not t_task.done():
                    t_task.cancel()
            if not producer_task.done():
                producer_task.cancel()
                try:
                    await producer_task
                except asyncio.CancelledError:
                    pass
        metrics.full_text = full_reply
        self.chat_history.append({"role": "assistant", "content": full_reply})
        max_hist = getattr(self.config, "max_history_messages", 20)
        if len(self.chat_history) > max_hist:
            self.chat_history = self.chat_history[-max_hist:]

        # Cultivation progression and asynchronous background memory extraction
        if full_reply.strip() and not self._interrupt_event.is_set():
            growth_update = self.companion.add_turn_xp(is_voice=bool(audio_input and not text_input))
            yield PipelineEvent(
                event_type="companion_update",
                payload=growth_update,
            )
            asyncio.create_task(self.companion.extract_memory_facts_async(user_text, full_reply))

        # Final event: Telemetry metrics
        yield PipelineEvent(
            event_type="metrics",
            payload=metrics.model_dump(),
        )
