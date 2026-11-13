package audioagent

import (
	"strings"
	"testing"
	"time"
)

func TestArbitrateIntent_VehicleAndSystemCommands(t *testing.T) {
	tests := []struct {
		query       string
		expectMatch bool
		actionName  string
	}{
		{"播放周杰伦的稻香", true, "play_music"},
		{"暂停播放", true, "play_music"},
		{"帮我定一个明天早晨七点的闹钟", true, "set_alarm"},
		{"为什么天空是蓝色的", false, ""},
		{"给我讲个好笑的笑话吧", false, ""},
	}

	for _, tt := range tests {
		res := ArbitrateIntent(tt.query)
		if tt.expectMatch {
			if res == "" {
				t.Errorf("expected match for query %q, got empty", tt.query)
			} else if !strings.Contains(res, tt.actionName) {
				t.Errorf("query %q: expected action %q in %s", tt.query, tt.actionName, res)
			}
		} else {
			if res != "" {
				t.Errorf("expected empty for query %q, got %s", tt.query, res)
			}
		}
	}
}

func TestDispatchIntent_MultiDomainCPlusPlus(t *testing.T) {
	// 1. Action: 蹲下
	sitJson := DispatchIntent("蹲下")
	if !strings.Contains(sitJson, `"action":"sit"`) {
		t.Fatalf("expected sit action in JSON, got %s", sitJson)
	}

	// 2. Action: 跳一下
	jumpJson := DispatchIntent("跳一下")
	if !strings.Contains(jumpJson, `"action":"jump"`) {
		t.Fatalf("expected jump action in JSON, got %s", jumpJson)
	}

	// 3. Action: 向右跑 (带位移)
	moveJson := DispatchIntent("向右跑两步")
	if !strings.Contains(moveJson, `"action":"move_right"`) || !strings.Contains(moveJson, `"direction":"right"`) {
		t.Fatalf("expected move_right with right direction, got %s", moveJson)
	}

	// 4. Memory: 记住
	memJson := DispatchIntent("记住我明天要开会")
	if !strings.Contains(memJson, `"domain":"MEMORY_COMMAND"`) {
		t.Fatalf("expected MEMORY_COMMAND in JSON, got %s", memJson)
	}

	// 5. Persona: 切换成伴侣
	personaJson := DispatchIntent("切换成伴侣人设")
	if !strings.Contains(personaJson, `"domain":"PERSONA_SWITCH"`) {
		t.Fatalf("expected PERSONA_SWITCH in JSON, got %s", personaJson)
	}

	// 6. Open-ended question (Anti-conflict)
	queryJson := DispatchIntent("为什么蹲下会觉得累")
	if !strings.Contains(queryJson, `"domain":"OPEN_ENDED"`) {
		t.Fatalf("expected OPEN_ENDED in JSON, got %s", queryJson)
	}
}

func TestStateString(t *testing.T) {
	if StateIdle.String() != "IDLE" {
		t.Errorf("expected IDLE, got %s", StateIdle.String())
	}
	if StateSpeaking.String() != "SPEAKING" {
		t.Errorf("expected SPEAKING, got %s", StateSpeaking.String())
	}
}

func TestSentenceChunker_StreamingAndToolStripping(t *testing.T) {
	var sentences []string
	var tools []string

	chunker := NewSentenceChunker(func(s string) {
		sentences = append(sentences, s)
	}, func(tool string) {
		tools = append(tools, tool)
	})
	defer chunker.Close()

	// Push tokens with tool call: "好的，马上为您设定闹钟。<|tool_call|>{"action":"set_alarm","time":"08:00"}<|/tool_call|>已设置完成。"
	tokens := []string{"好", "的", "，", "马", "上", "为", "您", "设", "定", "。"}
	for _, tok := range tokens {
		chunker.Push(tok)
	}

	chunker.Push("<|tool_call|>{\"action\":\"set_alarm\",\"time\":\"08:00\"}<|/tool_call|>")
	chunker.Push("已调节完成。")
	chunker.Flush()

	if len(sentences) == 0 {
		t.Fatal("expected at least one sentence emitted")
	}
	t.Logf("Emitted sentences: %v", sentences)

	for _, s := range sentences {
		if strings.Contains(s, "tool_call") {
			t.Errorf("sentence %q contains raw tool_call tags", s)
		}
	}

	if len(tools) != 1 {
		t.Fatalf("expected 1 tool call dispatched, got %d", len(tools))
	}
	if !strings.Contains(tools[0], "set_alarm") {
		t.Errorf("expected set_alarm in tool call, got %s", tools[0])
	}
}

func TestPipeline_LifecycleAndState(t *testing.T) {
	var stateChanges []State
	pipeline := NewPipeline("你是跨平台语音助手", PipelineCallbacks{
		OnStateChanged: func(newState, oldState State) {
			stateChanges = append(stateChanges, newState)
		},
	})
	defer pipeline.Close()

	if !pipeline.Start() {
		t.Fatal("failed to start pipeline")
	}

	if pipeline.GetState() != StateIdle {
		t.Errorf("expected IDLE initial state, got %v", pipeline.GetState())
	}

	pipeline.Stop()
}

func TestPipeline_PersonaManagementAndTime(t *testing.T) {
	pipeline := NewPipeline("", PipelineCallbacks{})
	defer pipeline.Close()

	// Initial persona
	p := pipeline.GetPersona()
	if p == "" {
		t.Fatal("expected non-empty persona ID")
	}

	// Switch to roaster
	if !pipeline.SetPersona("roaster") {
		t.Fatal("failed to switch to roaster persona")
	}
	if pipeline.GetPersona() != "roaster" {
		t.Errorf("expected roaster, got %s", pipeline.GetPersona())
	}

	// Switch to butler
	if !pipeline.SetPersona("butler") {
		t.Fatal("failed to switch to butler persona")
	}
	if pipeline.GetPersona() != "butler" {
		t.Errorf("expected butler, got %s", pipeline.GetPersona())
	}

	// Time greeting
	greeting := pipeline.GetTimeGreeting()
	if greeting == "" {
		t.Error("expected non-empty time greeting")
	}
	t.Logf("Time greeting: %s", greeting)
}

func TestPipeline_AudioPushPullModeA(t *testing.T) {
	pipeline := NewPipeline("", PipelineCallbacks{})
	defer pipeline.Close()
	pipeline.Start()
	defer pipeline.Stop()

	// Push 512 frames of 16kHz float mic PCM
	micData := make([]float32, 512)
	for i := range micData {
		micData[i] = 0.42
	}
	written := pipeline.PushMicPCM(micData)
	if written != 512 {
		t.Errorf("expected 512 frames written, got %d", written)
	}

	// Pull speaker PCM
	spkDest := make([]float32, 480)
	_ = pipeline.PullSpeakerPCM(spkDest)

	// Toggle hardware audio
	pipeline.EnableHardwareAudio(false)
}

func TestPipeline_CloudStreamingTurn(t *testing.T) {
	var sentences []string
	var tools []string

	pipeline := NewPipeline("", PipelineCallbacks{
		OnTTSSentence: func(s string) {
			sentences = append(sentences, s)
		},
		OnToolCall: func(toolJSON string) {
			tools = append(tools, toolJSON)
		},
	})
	defer pipeline.Close()
	pipeline.Start()
	defer pipeline.Stop()

	// Start cloud turn
	pipeline.StartCloudTurn("请帮我查一下杭州天气")
	if pipeline.GetState() != StateThinking {
		t.Errorf("expected THINKING, got %v", pipeline.GetState())
	}

	// Push streaming tokens
	tokens := []string{"好", "的", "，", "正", "在", "为", "您", "查", "询", "。"}
	for _, tok := range tokens {
		pipeline.PushCloudToken(tok)
	}
	pipeline.PushCloudToken("<|tool_call|>{\"action\":\"query_weather\",\"city\":\"杭州\"}<|/tool_call|>")
	pipeline.PushCloudToken("已完成查询。")
	pipeline.FinishCloudTurn()

	// Allow worker thread to process
	for range 20 {
		if len(sentences) > 0 && len(tools) > 0 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	if len(sentences) == 0 {
		t.Fatal("expected sentences emitted from cloud turn")
	}
	if len(tools) == 0 {
		t.Fatal("expected tool call dispatched from cloud turn")
	}
	if !strings.Contains(tools[0], "query_weather") {
		t.Errorf("expected query_weather in tool call, got %s", tools[0])
	}
}

