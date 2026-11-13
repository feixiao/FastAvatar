#include "audio_agent.h"
#include "AudioAgentPipeline.hpp"
#include "StreamingSentenceChunker.hpp"
#include "IntentArbiter.hpp"
#include "MnnLlmEngine.hpp"
#include "MockCloudLlmEngine.hpp"
#include "SherpaOnnxPerceptionEngine.hpp"

#include <cstring>
#include <cstdlib>
#include <memory>
#include <string>
#include <ostream>

using namespace audio_agent;

// -------------------------------------------------------------
// Internal wrapper structs
// -------------------------------------------------------------

class DefaultCapiTtsEngine : public ISpeechSynthesisEngine {
public:
    bool SynthesizeStreaming(const std::string&, AudioChunkCallback callback) override {
        std::vector<float> pcm(480, 0.0f);
        return callback(pcm.data(), static_cast<int32_t>(pcm.size()), 1.0f) != 0;
    }
    int32_t GetSampleRate() const override { return 24000; }
};

struct audio_agent_pipeline_t {
    std::shared_ptr<AudioAgentPipeline> pipeline;
    std::shared_ptr<ISpeechPerceptionEngine> perception;
    std::shared_ptr<ILlmEngine> llm;
    std::shared_ptr<ISpeechSynthesisEngine> tts;
    audio_agent_callbacks_t callbacks;
};

struct audio_agent_chunker_t {
    std::unique_ptr<StreamingSentenceChunker> chunker;
    audio_agent_sentence_cb on_sentence;
    audio_agent_tool_cb on_tool;
    void* user_data;
};

// -------------------------------------------------------------
// Pipeline Lifecycle Implementation
// -------------------------------------------------------------

audio_agent_pipeline_t* audio_agent_pipeline_create(
    const char* system_prompt,
    const audio_agent_callbacks_t* callbacks
) {
    auto* handle = new audio_agent_pipeline_t();
    if (callbacks) {
        handle->callbacks = *callbacks;
    }

    handle->perception = std::make_shared<SherpaOnnxPerceptionEngine>();
    handle->llm = std::make_shared<MockCloudLlmEngine>();
    handle->tts = std::make_shared<DefaultCapiTtsEngine>();

    PipelineObserver observer;
    if (handle->callbacks.on_state_changed) {
        observer.on_state_changed = [handle](AgentState new_state, AgentState old_state) {
            handle->callbacks.on_state_changed(
                static_cast<audio_agent_state_t>(new_state),
                static_cast<audio_agent_state_t>(old_state),
                handle->callbacks.user_data
            );
        };
    }
    if (handle->callbacks.on_tts_sentence) {
        observer.on_tts_sentence_start = [handle](const std::string& s) {
            handle->callbacks.on_tts_sentence(s.c_str(), handle->callbacks.user_data);
        };
    }
    if (handle->callbacks.on_tool_call) {
        observer.on_tool_call = [handle](const std::string& tool_json) {
            handle->callbacks.on_tool_call(tool_json.c_str(), handle->callbacks.user_data);
        };
    }
    if (handle->callbacks.on_barge_in) {
        observer.on_barge_in = [handle](const std::string& spoken) {
            handle->callbacks.on_barge_in(spoken.c_str(), handle->callbacks.user_data);
        };
    }
    if (handle->callbacks.on_persona_switched) {
        observer.on_persona_switched = [handle](const std::string& id, const std::string& name) {
            handle->callbacks.on_persona_switched(id.c_str(), name.c_str(), handle->callbacks.user_data);
        };
    }

    std::string prompt_str = system_prompt ? system_prompt : "";
    handle->pipeline = std::make_shared<AudioAgentPipeline>(
        handle->perception,
        handle->llm,
        handle->tts,
        observer,
        prompt_str
    );

    return handle;
}

bool audio_agent_pipeline_start(audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return false;
    return handle->pipeline->Start();
}

void audio_agent_pipeline_stop(audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return;
    handle->pipeline->Stop();
}

void audio_agent_pipeline_destroy(audio_agent_pipeline_t* handle) {
    if (!handle) return;
    if (handle->pipeline) {
        handle->pipeline->Stop();
    }
    delete handle;
}

void audio_agent_pipeline_submit_query(audio_agent_pipeline_t* handle, const char* query) {
    if (!handle || !handle->pipeline || !query) return;
    handle->pipeline->SubmitUserQuery(query);
}

void audio_agent_pipeline_trigger_barge_in(audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return;
    handle->pipeline->TriggerBargeIn();
}

audio_agent_state_t audio_agent_pipeline_get_state(const audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return AUDIO_AGENT_STATE_IDLE;
    return static_cast<audio_agent_state_t>(handle->pipeline->GetState());
}

// -------------------------------------------------------------
// Mode A: Platform Audio PCM Push / Pull
// -------------------------------------------------------------

size_t audio_agent_pipeline_push_mic_pcm(audio_agent_pipeline_t* handle, const float* data, size_t count) {
    if (!handle || !handle->pipeline || !data) return 0;
    return handle->pipeline->PushMicAudio(data, count);
}

size_t audio_agent_pipeline_pull_speaker_pcm(audio_agent_pipeline_t* handle, float* dest, size_t count) {
    if (!handle || !handle->pipeline || !dest) return 0;
    return handle->pipeline->PullSpeakerAudio(dest, count);
}

size_t audio_agent_pipeline_speaker_available(const audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return 0;
    return handle->pipeline->GetSpeakerAvailableSamples();
}

void audio_agent_pipeline_enable_hardware_audio(audio_agent_pipeline_t* handle, bool enable) {
    if (!handle || !handle->pipeline) return;
    handle->pipeline->EnableHardwareAudio(enable);
}

// -------------------------------------------------------------
// Cloud Cognitive Stream Turn (Dual-Loop Slow Loop B)
// -------------------------------------------------------------

void audio_agent_pipeline_start_cloud_turn(audio_agent_pipeline_t* handle, const char* query) {
    if (!handle || !handle->pipeline || !query) return;
    handle->pipeline->StartCloudTurn(query);
}

void audio_agent_pipeline_push_cloud_token(audio_agent_pipeline_t* handle, const char* token, size_t len) {
    if (!handle || !handle->pipeline || !token) return;
    std::string tok_str(token, len);
    handle->pipeline->PushCloudToken(tok_str);
}

void audio_agent_pipeline_finish_cloud_turn(audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return;
    handle->pipeline->FinishCloudTurn();
}

void audio_agent_pipeline_abort_cloud_turn(audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return;
    handle->pipeline->AbortCloudTurn();
}

// -------------------------------------------------------------
// Voice Companion Persona Management
// -------------------------------------------------------------

bool audio_agent_pipeline_set_persona(audio_agent_pipeline_t* handle, const char* persona_id) {
    if (!handle || !handle->pipeline || !persona_id) return false;
    return handle->pipeline->SetPersona(persona_id);
}

char* audio_agent_pipeline_get_persona(const audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return nullptr;
    std::string pid = handle->pipeline->GetCurrentPersonaId();
    char* res = static_cast<char*>(malloc(pid.size() + 1));
    if (res) {
        memcpy(res, pid.c_str(), pid.size() + 1);
    }
    return res;
}

char* audio_agent_pipeline_get_persona_name(const audio_agent_pipeline_t* handle) {
    if (!handle || !handle->pipeline) return nullptr;
    std::string name = handle->pipeline->GetCurrentPersonaName();
    char* res = static_cast<char*>(malloc(name.size() + 1));
    if (res) {
        memcpy(res, name.c_str(), name.size() + 1);
    }
    return res;
}

char* audio_agent_pipeline_get_time_greeting(void) {
    std::string greeting = CompanionPersonaManager::GetTimeOfDayContext();
    char* res = static_cast<char*>(malloc(greeting.size() + 1));
    if (res) {
        memcpy(res, greeting.c_str(), greeting.size() + 1);
    }
    return res;
}

// -------------------------------------------------------------
// Intent Arbiter C API Implementation
// -------------------------------------------------------------

char* audio_agent_arbitrate_intent(const char* query) {
    if (!query) return nullptr;
    std::string result = IntentArbiter::ResolveToolCall(query);
    if (result.empty()) return nullptr;
    return strdup(result.c_str());
}

char* audio_agent_dispatch_intent(const char* query) {
    if (!query) return nullptr;
    std::string result = IntentArbiter::ResolveDispatchJson(query);
    if (result.empty()) return nullptr;
    return strdup(result.c_str());
}

char* audio_agent_dispatch_intent_context(const char* query, const char* last_route, const char* last_domain) {
    if (!query) return nullptr;
    std::string route = last_route ? last_route : "";
    std::string domain = last_domain ? last_domain : "";
    std::string result = IntentArbiter::ResolveDispatchJson(query, route, domain);
    if (result.empty()) return nullptr;
    return strdup(result.c_str());
}

void audio_agent_free_string(char* str) {
    if (str) {
        free(str);
    }
}

// -------------------------------------------------------------
// Standalone Sentence Chunker C API Implementation
// -------------------------------------------------------------

audio_agent_chunker_t* audio_agent_chunker_create(
    audio_agent_sentence_cb on_sentence,
    audio_agent_tool_cb on_tool,
    void* user_data
) {
    auto* handle = new audio_agent_chunker_t();
    handle->on_sentence = on_sentence;
    handle->on_tool = on_tool;
    handle->user_data = user_data;

    SentenceChunkerCallbacks callbacks;
    if (on_sentence) {
        callbacks.on_sentence_ready = [handle](const std::string& sentence) {
            handle->on_sentence(sentence.c_str(), handle->user_data);
        };
    }
    if (on_tool) {
        callbacks.on_tool_call = [handle](const std::string& tool_json) {
            handle->on_tool(tool_json.c_str(), handle->user_data);
        };
    }
    callbacks.is_aborted = []() { return false; };

    handle->chunker = std::make_unique<StreamingSentenceChunker>(callbacks);
    return handle;
}

void audio_agent_chunker_push(audio_agent_chunker_t* chunker, const char* token, size_t len) {
    if (!chunker || !chunker->chunker || !token || len == 0) return;
    chunker->chunker->sputn(token, static_cast<std::streamsize>(len));
}

void audio_agent_chunker_flush(audio_agent_chunker_t* chunker) {
    if (!chunker || !chunker->chunker) return;
    chunker->chunker->Flush();
}

void audio_agent_chunker_reset(audio_agent_chunker_t* chunker) {
    if (!chunker || !chunker->chunker) return;
    chunker->chunker->Reset();
}

void audio_agent_chunker_destroy(audio_agent_chunker_t* chunker) {
    if (!chunker) return;
    delete chunker;
}
