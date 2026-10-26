package engine

import (
	"strings"
	"unicode"
)

type SentenceSplitter struct {
	buffer         strings.Builder
	isFirstChunk   bool
	minFirstLength int
	minNextLength  int
}

func NewSentenceSplitter() *SentenceSplitter {
	return &SentenceSplitter{
		isFirstChunk:   true,
		minFirstLength: 4,
		minNextLength:  6,
	}
}

func (s *SentenceSplitter) Reset() {
	s.buffer.Reset()
	s.isFirstChunk = true
}

var strongPunct = map[rune]bool{
	'。': true, '！': true, '？': true, '\n': true,
	'!': true, '?': true,
}

var weakPunct = map[rune]bool{
	'，': true, '；': true, '、': true, '…': true,
	',': true, ';': true, ':': true, '：': true,
}

// Push 接收流式 Token 并返回所有已经切割出来的短句列表
func (s *SentenceSplitter) Push(token string) []string {
	var sentences []string
	runes := []rune(token)

	for i, r := range runes {
		s.buffer.WriteRune(r)

		bufStr := s.buffer.String()
		bufRunes := []rune(bufStr)
		bufLen := len(bufRunes)

		threshold := s.minNextLength
		if s.isFirstChunk || strongPunct[r] {
			threshold = s.minFirstLength
		}

		if bufLen < threshold {
			continue
		}

		isSplitPoint := false

		if strongPunct[r] {
			// 保护浮点数如 3.14
			if r == '.' && i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
				isSplitPoint = false
			} else {
				isSplitPoint = true
			}
		} else if weakPunct[r] {
			isSplitPoint = true
		}

		if isSplitPoint {
			chunk := strings.TrimSpace(bufStr)
			if len([]rune(chunk)) > 0 {
				sentences = append(sentences, chunk)
				s.isFirstChunk = false
				s.buffer.Reset()
			}
		}
	}

	return sentences
}

// Flush 清空剩余未结算的文本
func (s *SentenceSplitter) Flush() []string {
	var sentences []string
	remaining := strings.TrimSpace(s.buffer.String())
	if len([]rune(remaining)) > 0 {
		sentences = append(sentences, remaining)
	}
	s.Reset()
	return sentences
}
