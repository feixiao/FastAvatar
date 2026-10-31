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
// Intent Arbiter C API Implementation
// -------------------------------------------------------------

char* audio_agent_arbitrate_intent(const char* query) {
    if (!query) return nullptr;
    std::string result = IntentArbiter::ResolveToolCall(query);
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
