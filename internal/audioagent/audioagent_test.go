package audioagent

import (
	"strings"
	"testing"
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
