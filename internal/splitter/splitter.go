package splitter

import (
	"regexp"
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

var (
	bracketPattern  = regexp.MustCompile(`（[^）]*）|\([^)]*\)|【[^】]*】|\[[^\]]*\]`)
	prefixPattern   = regexp.MustCompile(`^(大肥鱼|鲸鱼娘|回复|助手|AI|Assistant)\s*[:：]\s*`)
	markdownPattern = regexp.MustCompile(`[*_#~` + "`" + `]`)
)

// CleanSpokenText 过滤大模型输出中的 Markdown 标记、括号神态指示、角色前缀与 Emoji，确保语音引擎只朗读纯净口语文本
func CleanSpokenText(s string) string {
	if s == "" {
		return ""
	}
	s = markdownPattern.ReplaceAllString(s, "")
	s = strings.TrimSpace(s)
	s = prefixPattern.ReplaceAllString(s, "")
	s = bracketPattern.ReplaceAllString(s, "")

	var sb strings.Builder
	for _, r := range s {
		if isEmojiOrSymbol(r) {
			continue
		}
		sb.WriteRune(r)
	}
	return strings.TrimSpace(sb.String())
}

func isEmojiOrSymbol(r rune) bool {
	if (r >= 0x1F600 && r <= 0x1F64F) ||
		(r >= 0x1F300 && r <= 0x1F5FF) ||
		(r >= 0x1F680 && r <= 0x1F6FF) ||
		(r >= 0x1F700 && r <= 0x1F77F) ||
		(r >= 0x1F780 && r <= 0x1F7FF) ||
		(r >= 0x1F800 && r <= 0x1F8FF) ||
		(r >= 0x1F900 && r <= 0x1F9FF) ||
		(r >= 0x1FA00 && r <= 0x1FA6F) ||
		(r >= 0x1FA70 && r <= 0x1FAFF) ||
		(r >= 0x2600 && r <= 0x26FF) ||
		(r >= 0x2700 && r <= 0x27BF) ||
		(r >= 0xFE00 && r <= 0xFE0F) ||
		(r >= 0x1F1E6 && r <= 0x1F1FF) {
		return true
	}
	return false
}
