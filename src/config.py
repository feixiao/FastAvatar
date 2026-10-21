"""Configuration system for FastAvatar Engine."""

from enum import Enum
from typing import Optional
from pydantic import BaseModel, Field


class LLMProviderType(str, Enum):
    MOCK = "mock"
    LM_STUDIO = "lm_studio"
    OLLAMA = "ollama"
    OPENAI_COMPATIBLE = "openai_compatible"

class ASRProviderType(str, Enum):
    MOCK = "mock"
    SHERPA_ONNX = "sherpa_onnx"
    OPENAI_COMPATIBLE = "openai_compatible"


class TTSProviderType(str, Enum):
    MOCK = "mock"
    MAC_SAY = "mac_say"
    EDGE_TTS = "edge_tts"
    SHERPA_ONNX = "sherpa_onnx"


class LLMConfig(BaseModel):
    provider: LLMProviderType = LLMProviderType.LM_STUDIO
    model: str = "qwen2.5-7b-instruct"
    base_url: str = "http://localhost:1234/v1"
    api_key: Optional[str] = "lm-studio"
    system_prompt: str = (
        "你是可爱的AI数字人助手。你的回答需要口语化、亲切自然、简洁，适合直接转成语音播报。"
        "避免使用复杂的表格、列表符号，单次回复控制在2-3句话以内。"
    )
    temperature: float = 0.7
    max_tokens: int = 256


class ASRConfig(BaseModel):
    provider: ASRProviderType = ASRProviderType.MOCK
    model_name: str = "sherpa-onnx-sense-voice-zh-en-ja-ko-yue"
    sample_rate: int = 16000
    language: str = "zh"


class TTSConfig(BaseModel):
    provider: TTSProviderType = TTSProviderType.EDGE_TTS
    voice: str = "zh-CN-XiaoxiaoNeural"  # or macOS voice "Ting-Ting"
    sample_rate: int = 16000
    speed: float = 1.0


class PipelineConfig(BaseModel):
    # Sentence splitter configuration (critical for low first-packet latency)
    min_chunk_length: int = 6        # Minimum chars to trigger TTS chunk
    max_chunk_length: int = 35       # Force split if clause exceeds this
    split_punctuations: str = "，。！？；、\n\r,!?;:"
    
    # Audio settings
    sample_rate: int = 16000
    channels: int = 1
    
    # Enable barge-in (interruption)
    enable_barge_in: bool = True
    
    # Sub-component configs
    llm: LLMConfig = Field(default_factory=LLMConfig)
    asr: ASRConfig = Field(default_factory=ASRConfig)
    tts: TTSConfig = Field(default_factory=TTSConfig)


class AppConfig(BaseModel):
    host: str = "0.0.0.0"
    port: int = 8000
    pipeline: PipelineConfig = Field(default_factory=PipelineConfig)


def get_default_config() -> AppConfig:
    return AppConfig()
