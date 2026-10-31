#ifndef AUDIO_AGENT_C_API_H
#define AUDIO_AGENT_C_API_H

#include <stdint.h>
#include <stddef.h>
#include <stdbool.h>

#ifdef __cplusplus
extern "C" {
#endif

#if defined(_WIN32)
  #ifdef AUDIO_AGENT_BUILD_SHARED
    #define AUDIO_AGENT_API __declspec(dllexport)
  #else
    #define AUDIO_AGENT_API __declspec(dllimport)
  #endif
#else
  #define AUDIO_AGENT_API __attribute__((visibility("default")))
#endif

/**
 * @brief Finite State Machine states for Audio Agent
 */
typedef enum {
    AUDIO_AGENT_STATE_IDLE = 0,
    AUDIO_AGENT_STATE_LISTENING = 1,
    AUDIO_AGENT_STATE_THINKING = 2,
    AUDIO_AGENT_STATE_SPEAKING = 3
} audio_agent_state_t;

// Opaque handles
typedef struct audio_agent_pipeline_t audio_agent_pipeline_t;
typedef struct audio_agent_chunker_t audio_agent_chunker_t;

// Callback signatures
typedef void (*audio_agent_state_cb)(audio_agent_state_t new_state, audio_agent_state_t old_state, void* user_data);
typedef void (*audio_agent_sentence_cb)(const char* sentence, void* user_data);
typedef void (*audio_agent_tool_cb)(const char* tool_json, void* user_data);
typedef void (*audio_agent_barge_in_cb)(const char* spoken_so_far, void* user_data);

typedef struct {
    audio_agent_state_cb on_state_changed;
    audio_agent_sentence_cb on_tts_sentence;
    audio_agent_tool_cb on_tool_call;
    audio_agent_barge_in_cb on_barge_in;
    void* user_data;
} audio_agent_callbacks_t;

// -------------------------------------------------------------
// Pipeline Lifecycle & Control (Full-Duplex Engine)
// -------------------------------------------------------------

AUDIO_AGENT_API audio_agent_pipeline_t* audio_agent_pipeline_create(
    const char* system_prompt,
    const audio_agent_callbacks_t* callbacks
);

AUDIO_AGENT_API bool audio_agent_pipeline_start(audio_agent_pipeline_t* handle);
AUDIO_AGENT_API void audio_agent_pipeline_stop(audio_agent_pipeline_t* handle);
AUDIO_AGENT_API void audio_agent_pipeline_destroy(audio_agent_pipeline_t* handle);

AUDIO_AGENT_API void audio_agent_pipeline_submit_query(audio_agent_pipeline_t* handle, const char* query);
AUDIO_AGENT_API void audio_agent_pipeline_trigger_barge_in(audio_agent_pipeline_t* handle);
AUDIO_AGENT_API audio_agent_state_t audio_agent_pipeline_get_state(const audio_agent_pipeline_t* handle);

// -------------------------------------------------------------
// Fast-Path Intent Arbiter (Zero-Hallucination Deterministic Router)
// -------------------------------------------------------------

AUDIO_AGENT_API char* audio_agent_arbitrate_intent(const char* query);
AUDIO_AGENT_API void audio_agent_free_string(char* str);

// -------------------------------------------------------------
// Standalone Streaming Sentence Chunker (<6B TTFT & Tool Stripping)
// -------------------------------------------------------------

AUDIO_AGENT_API audio_agent_chunker_t* audio_agent_chunker_create(
    audio_agent_sentence_cb on_sentence,
    audio_agent_tool_cb on_tool,
    void* user_data
);

AUDIO_AGENT_API void audio_agent_chunker_push(audio_agent_chunker_t* chunker, const char* token, size_t len);
AUDIO_AGENT_API void audio_agent_chunker_flush(audio_agent_chunker_t* chunker);
AUDIO_AGENT_API void audio_agent_chunker_reset(audio_agent_chunker_t* chunker);
AUDIO_AGENT_API void audio_agent_chunker_destroy(audio_agent_chunker_t* chunker);

#ifdef __cplusplus
}
#endif

#endif // AUDIO_AGENT_C_API_H
