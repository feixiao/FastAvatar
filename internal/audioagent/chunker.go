package audioagent

/*
#cgo CFLAGS: -I${SRCDIR}/../../core/include
#cgo LDFLAGS: -L${SRCDIR}/../../core/build -laudio_agent -Wl,-rpath,${SRCDIR}/../../core/build
#include "audio_agent.h"
#include <stdlib.h>

extern void goOnSentenceBridge(char* sentence, void* user_data);
extern void goOnToolBridge(char* tool_json, void* user_data);

static inline audio_agent_chunker_t* create_chunker_bridge(void* user_data) {
    return audio_agent_chunker_create(
        (audio_agent_sentence_cb)goOnSentenceBridge,
        (audio_agent_tool_cb)goOnToolBridge,
        user_data
    );
}
*/
import "C"
import (
	"sync"
	"unsafe"
)

var (
	chunkerRegistry sync.Map
	chunkerSeq      uintptr
	chunkerSeqMu    sync.Mutex
)

func nextChunkerID() uintptr {
	chunkerSeqMu.Lock()
	defer chunkerSeqMu.Unlock()
	chunkerSeq++
	return chunkerSeq
}

//export goOnSentenceBridge
func goOnSentenceBridge(sentence *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := chunkerRegistry.Load(id); ok {
		ch := val.(*SentenceChunker)
		if ch.onSentence != nil {
			ch.onSentence(C.GoString(sentence))
		}
	}
}

//export goOnToolBridge
func goOnToolBridge(toolJSON *C.char, userData unsafe.Pointer) {
	id := uintptr(userData)
	if val, ok := chunkerRegistry.Load(id); ok {
		ch := val.(*SentenceChunker)
		if ch.onTool != nil {
			ch.onTool(C.GoString(toolJSON))
		}
	}
}

// SentenceChunker wraps the native C++ StreamingSentenceChunker.
type SentenceChunker struct {
	handle     *C.audio_agent_chunker_t
	id         uintptr
	onSentence func(string)
	onTool     func(string)
	mu         sync.Mutex
}

// NewSentenceChunker creates a new native streaming sentence chunker.
func NewSentenceChunker(onSentence func(string), onTool func(string)) *SentenceChunker {
	id := nextChunkerID()
	sc := &SentenceChunker{
		id:         id,
		onSentence: onSentence,
		onTool:     onTool,
	}

	chunkerRegistry.Store(id, sc)
	sc.handle = C.create_chunker_bridge(unsafe.Pointer(id))
	return sc
}

// Push feeds streaming tokens into the native chunker.
func (c *SentenceChunker) Push(token string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil || token == "" {
		return
	}

	cToken := C.CString(token)
	defer C.free(unsafe.Pointer(cToken))

	C.audio_agent_chunker_push(c.handle, cToken, C.size_t(len(token)))
}

// Flush forces any remaining text in the buffer to be emitted.
func (c *SentenceChunker) Flush() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil {
		return
	}
	C.audio_agent_chunker_flush(c.handle)
}

// Reset resets the chunker state for a new turn.
func (c *SentenceChunker) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle == nil {
		return
	}
	C.audio_agent_chunker_reset(c.handle)
}

// Close destroys the underlying native C++ chunker handle.
func (c *SentenceChunker) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.handle != nil {
		C.audio_agent_chunker_destroy(c.handle)
		c.handle = nil
		chunkerRegistry.Delete(c.id)
	}
}
