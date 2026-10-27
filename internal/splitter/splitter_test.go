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
