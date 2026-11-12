package audioagent

/*
#cgo CFLAGS: -I${SRCDIR}/../../core/include
#cgo LDFLAGS: -L${SRCDIR}/../../core/build -laudio_agent -Wl,-rpath,${SRCDIR}/../../core/build
#include "audio_agent.h"
#include <stdlib.h>

extern void goOnStateChangedBridge(audio_agent_state_t new_state, audio_agent_state_t old_state, void* user_data);
extern void goOnPipelineSentenceBridge(char* sentence, void* user_data);
extern void goOnPipelineToolBridge(char* tool_json, void* user_data);
extern void goOnBargeInBridge(char* spoken_so_far, void* user_data);
extern void goOnPersonaSwitchedBridge(char* persona_id, char* persona_name, void* user_data);

static inline audio_agent_pipeline_t* create_pipeline_bridge(const char* sys_prompt, void* user_data) {
    audio_agent_callbacks_t cbs;
    cbs.on_state_changed = (audio_agent_state_cb)goOnStateChangedBridge;
    cbs.on_tts_sentence = (audio_agent_sentence_cb)goOnPipelineSentenceBridge;
    cbs.on_tool_call = (audio_agent_tool_cb)goOnPipelineToolBridge;
    cbs.on_barge_in = (audio_agent_barge_in_cb)goOnBargeInBridge;
    cbs.on_persona_switched = (audio_agent_persona_cb)goOnPersonaSwitchedBridge;
    cbs.user_data = user_data;
    return audio_agent_pipeline_create(sys_prompt, &cbs);
}
*/
import "C"
import (
	"sync"
	"unsafe"
)

var (
	pipelineRegistry sync.Map
	pipelineSeq      uintptr
	pipelineSeqMu    sync.Mutex
)

func nextPipelineID() uintptr {
	pipelineSeqMu.Lock()
	defer pipelineSeqMu.Unlock()
	pipelineSeq++
	return pipelineSeq
}

//export goOnStateChangedBridge
func goOnStateChangedBridge(newState, oldState C.audio_agent_state_t, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := pipelineRegistry.Load(id); ok {
		p := val.(*Pipeline)
		if p.cbs.OnStateChanged != nil {
			p.cbs.OnStateChanged(State(newState), State(oldState))
		}
	}
}

//export goOnPipelineSentenceBridge
func goOnPipelineSentenceBridge(sentence *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := pipelineRegistry.Load(id); ok {
		p := val.(*Pipeline)
		if p.cbs.OnTTSSentence != nil {
			p.cbs.OnTTSSentence(C.GoString(sentence))
		}
	}
}

//export goOnPipelineToolBridge
func goOnPipelineToolBridge(toolJSON *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := pipelineRegistry.Load(id); ok {
		p := val.(*Pipeline)
		if p.cbs.OnToolCall != nil {
			p.cbs.OnToolCall(C.GoString(toolJSON))
		}
	}
}

//export goOnBargeInBridge
func goOnBargeInBridge(spokenSoFar *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := pipelineRegistry.Load(id); ok {
		p := val.(*Pipeline)
		if p.cbs.OnBargeIn != nil {
			p.cbs.OnBargeIn(C.GoString(spokenSoFar))
		}
	}
}

//export goOnPersonaSwitchedBridge
func goOnPersonaSwitchedBridge(personaID, personaName *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := pipelineRegistry.Load(id); ok {
		p := val.(*Pipeline)
		if p.cbs.OnPersonaSwitched != nil {
			p.cbs.OnPersonaSwitched(C.GoString(personaID), C.GoString(personaName))
		}
	}
}

// PipelineCallbacks defines observation hooks for the voice assistant harness.
type PipelineCallbacks struct {
	OnStateChanged    func(newState, oldState State)
	OnTTSSentence     func(sentence string)
	OnToolCall        func(toolJSON string)
	OnBargeIn         func(spokenSoFar string)
	OnPersonaSwitched func(personaID, personaName string)
}

// Pipeline wraps the high-reliability ISO C++17 AudioAgentPipeline harness.
type Pipeline struct {
	handle *C.audio_agent_pipeline_t
	id     uintptr
	cbs    PipelineCallbacks
	mu     sync.Mutex
}

// NewPipeline creates a new native voice assistant harness pipeline instance.
func NewPipeline(systemPrompt string, cbs PipelineCallbacks) *Pipeline {
	id := nextPipelineID()
	p := &Pipeline{
		id:  id,
		cbs: cbs,
	}
	pipelineRegistry.Store(id, p)

	var cPrompt *C.char
	if systemPrompt != "" {
		cPrompt = C.CString(systemPrompt)
		defer C.free(unsafe.Pointer(cPrompt))
	}

	p.handle = C.create_pipeline_bridge(cPrompt, unsafe.Pointer(id))
	return p
}

// Start boots the background threads and audio IO of the harness.
func (p *Pipeline) Start() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return false
	}
	return bool(C.audio_agent_pipeline_start(p.handle))
}

// Stop stops all background worker loops.
func (p *Pipeline) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_stop(p.handle)
	}
}

// Close destroys the underlying native C++ pipeline.
func (p *Pipeline) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_destroy(p.handle)
		p.handle = nil
		pipelineRegistry.Delete(p.id)
	}
}

// SubmitUserQuery submits an oral transcription or textual command to the harness.
func (p *Pipeline) SubmitUserQuery(query string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || query == "" {
		return
	}
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	C.audio_agent_pipeline_submit_query(p.handle, cQuery)
}

// TriggerBargeIn triggers instant (<5ms) speech cancellation and KV cache rollback.
func (p *Pipeline) TriggerBargeIn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_trigger_barge_in(p.handle)
	}
}

// GetState returns the current FSM state (IDLE, LISTENING, THINKING, SPEAKING).
func (p *Pipeline) GetState() State {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return StateIdle
	}
	return State(C.audio_agent_pipeline_get_state(p.handle))
}

// PushMicPCM pushes microphone PCM frames (16kHz float) into the pipeline (Mode A).
func (p *Pipeline) PushMicPCM(pcm []float32) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || len(pcm) == 0 {
		return 0
	}
	count := C.size_t(len(pcm))
	written := C.audio_agent_pipeline_push_mic_pcm(p.handle, (*C.float)(&pcm[0]), count)
	return int(written)
}

// PullSpeakerPCM pulls speaker PCM frames (24kHz float) from the pipeline (Mode A).
func (p *Pipeline) PullSpeakerPCM(dest []float32) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || len(dest) == 0 {
		return 0
	}
	count := C.size_t(len(dest))
	read := C.audio_agent_pipeline_pull_speaker_pcm(p.handle, (*C.float)(&dest[0]), count)
	return int(read)
}

// SpeakerAvailable returns number of speaker samples ready for playback.
func (p *Pipeline) SpeakerAvailable() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return 0
	}
	return int(C.audio_agent_pipeline_speaker_available(p.handle))
}

// StartCloudTurn initiates a cloud-driven cognition turn (Slow Loop B).
func (p *Pipeline) StartCloudTurn(query string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || query == "" {
		return
	}
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))
	C.audio_agent_pipeline_start_cloud_turn(p.handle, cQuery)
}

// PushCloudToken pushes an incremental token chunk from the cloud SSE stream.
func (p *Pipeline) PushCloudToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || token == "" {
		return
	}
	cTok := C.CString(token)
	defer C.free(unsafe.Pointer(cTok))
	C.audio_agent_pipeline_push_cloud_token(p.handle, cTok, C.size_t(len(token)))
}

// FinishCloudTurn finishes the cloud token stream and flushes sentences to TTS.
func (p *Pipeline) FinishCloudTurn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_finish_cloud_turn(p.handle)
	}
}

// AbortCloudTurn aborts the current cloud turn.
func (p *Pipeline) AbortCloudTurn() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_abort_cloud_turn(p.handle)
	}
}

// SetPersona switches the companion persona slot ("butler", "companion", "roaster").
func (p *Pipeline) SetPersona(personaID string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil || personaID == "" {
		return false
	}
	cPid := C.CString(personaID)
	defer C.free(unsafe.Pointer(cPid))
	return bool(C.audio_agent_pipeline_set_persona(p.handle, cPid))
}

// GetPersona returns active persona identifier.
func (p *Pipeline) GetPersona() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle == nil {
		return ""
	}
	cPid := C.audio_agent_pipeline_get_persona(p.handle)
	if cPid == nil {
		return ""
	}
	defer C.audio_agent_free_string(cPid)
	return C.GoString(cPid)
}

// GetTimeGreeting returns natural time-of-day greeting (e.g. "下午 (15:30)").
func (p *Pipeline) GetTimeGreeting() string {
	cGreeting := C.audio_agent_pipeline_get_time_greeting()
	if cGreeting == nil {
		return ""
	}
	defer C.audio_agent_free_string(cGreeting)
	return C.GoString(cGreeting)
}

// EnableHardwareAudio toggles native hardware audio capture/playback (AudioQueue/Oboe).
func (p *Pipeline) EnableHardwareAudio(enable bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.handle != nil {
		C.audio_agent_pipeline_enable_hardware_audio(p.handle, C.bool(enable))
	}
}
