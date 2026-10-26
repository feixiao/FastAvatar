package engine

import (
	"reflect"
	"testing"
)

func TestSentenceSplitter(t *testing.T) {
	splitter := NewSentenceSplitter()

	tokens := []string{"你", "好", "呀", "！", "今", "天", "天", "气", "真", "不", "错", "，", "我", "们", "去", "散", "步", "吧", "。"}
	var got []string

	for _, tok := range tokens {
		chunks := splitter.Push(tok)
		got = append(got, chunks...)
	}
	got = append(got, splitter.Flush()...)

	expected := []string{
		"你好呀！",
		"今天天气真不错，",
		"我们去散步吧。",
	}

	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}

func TestSentenceSplitterFlush(t *testing.T) {
	splitter := NewSentenceSplitter()
	tokens := []string{"这", "句", "话", "没", "有", "标", "点"}
	var got []string
	for _, tok := range tokens {
		got = append(got, splitter.Push(tok)...)
	}
	got = append(got, splitter.Flush()...)

	expected := []string{"这句话没有标点"}
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("expected %v, got %v", expected, got)
	}
}
