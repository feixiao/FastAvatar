package splitter

import (
	"reflect"
	"testing"
)

func TestSentenceSplitter(t *testing.T) {
	s := NewSentenceSplitter()

	tokens := []string{"你", "好", "呀", "！", "今", "天", "天", "气", "真", "不", "错", "。"}
	var sentences []string
	for _, tok := range tokens {
		sentences = append(sentences, s.Push(tok)...)
	}
	sentences = append(sentences, s.Flush()...)

	expected := []string{"你好呀！", "今天天气真不错。"}
	if !reflect.DeepEqual(sentences, expected) {
		t.Fatalf("expected %v, got %v", expected, sentences)
	}
}

func TestCleanSpokenText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Markdown bold and emojis",
			input:    "大肥鱼：**哇！** 你终于把我叫醒啦！我是大肥鱼 🐋✨",
			expected: "哇！ 你终于把我叫醒啦！我是大肥鱼",
		},
		{
			name:     "Bracketed emote cues",
			input:    "回复: 今天想吃白饭配鱼片（咽口水），你呢？(微笑)",
			expected: "今天想吃白饭配鱼片，你呢？",
		},
		{
			name:     "Markdown headings and backticks",
			input:    "### 鲸鱼娘：`冰奶茶`超好喝的！",
			expected: "冰奶茶超好喝的！",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := CleanSpokenText(tc.input)
			if got != tc.expected {
				t.Errorf("CleanSpokenText(%q) = %q; want %q", tc.input, got, tc.expected)
			}
		})
	}
}
