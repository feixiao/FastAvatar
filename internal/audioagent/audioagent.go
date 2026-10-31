package audioagent

/*
#cgo CFLAGS: -I${SRCDIR}/../../core/include
#cgo LDFLAGS: -L${SRCDIR}/../../core/build -laudio_agent -Wl,-rpath,${SRCDIR}/../../core/build
#include "audio_agent.h"
#include <stdlib.h>
*/
import "C"
import (
	"unsafe"
)

// ArbitrateIntent parses user query with the zero-hallucination deterministic IntentArbiter.
// Returns JSON tool call payload if matched, or empty string.
func ArbitrateIntent(query string) string {
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	cResult := C.audio_agent_arbitrate_intent(cQuery)
	if cResult == nil {
		return ""
	}
	defer C.audio_agent_free_string(cResult)

	return C.GoString(cResult)
}

// State represents the 4-state audio agent FSM.
type State int

const (
	StateIdle      State = State(C.AUDIO_AGENT_STATE_IDLE)
	StateListening State = State(C.AUDIO_AGENT_STATE_LISTENING)
	StateThinking  State = State(C.AUDIO_AGENT_STATE_THINKING)
	StateSpeaking  State = State(C.AUDIO_AGENT_STATE_SPEAKING)
)

func (s State) String() string {
	switch s {
	case StateListening:
		return "LISTENING"
	case StateThinking:
		return "THINKING"
	case StateSpeaking:
		return "SPEAKING"
	default:
		return "IDLE"
	}
}
