package com.avatar.android.core.audio

import java.util.regex.Pattern

/**
 * 流式标点符号切句器 (Sentence Chunker)
 *
 * 核心目标：
 * 1. 消灭首音延迟 (TTFA)：首句只要满 minFirstLength (默认 3 字符，如"好的，") 遇标点即切分投递 TTS；
 * 2. 保证后续韵律：后续句子满 minNextLength (默认 8 字符) 遇标点再切分，保证语音停顿自然连贯；
 * 3. 拦截 Tool Call：自动识别并剥离 <|tool_call|>...<|/tool_call|>，避免 TTS 念出 JSON 字符串；
 * 4. 清洗 Markdown：去除加粗、标题、列表等口语无关符号，防止 TTS 读出"星号"、"井号"；
 * 5. 保护数字：防止将 3.14 等浮点数的小数点误识别为句子断点。
 */
class SentenceChunker(
    private val minFirstLength: Int = 3,
    private val minNextLength: Int = 8,
    private val onToolCall: ((String) -> Unit)? = null
) {
    private val buffer = StringBuilder()
    private var isFirstChunk = true

    companion object {
        private val STRONG_PUNCTS = hashSetOf('。', '！', '？', '!', '?', '\n')
        private val WEAK_PUNCTS = hashSetOf('，', '；', '、', ',', ';', ':', '：')
        private val MARKDOWN_PATTERN = Pattern.compile("(\\*\\*|\\*|`|###|##|#)")
        private const val TOOL_CALL_START = "<|tool_call|>"
        private const val TOOL_CALL_END = "<|/tool_call|>"
    }

    /**
     * 接收流式 Token 片段，返回所有已就绪的待朗读短句
     */
    @Synchronized
    fun push(token: String): List<String> {
        val cleanedToken = cleanMarkdown(token)
        buffer.append(cleanedToken)

        val sentences = mutableListOf<String>()

        while (true) {
            val bufStr = buffer.toString()

            // 1. 检查是否存在工具调用标签
            val startIdx = bufStr.indexOf(TOOL_CALL_START)
            if (startIdx != -1) {
                val endIdx = bufStr.indexOf(TOOL_CALL_END, startIdx + TOOL_CALL_START.length)
                if (endIdx != -1) {
                    val toolContent = bufStr.substring(startIdx + TOOL_CALL_START.length, endIdx).trim()
                    if (toolContent.isNotEmpty()) {
                        onToolCall?.invoke(toolContent)
                    }
                    val before = bufStr.substring(0, startIdx)
                    val after = bufStr.substring(endIdx + TOOL_CALL_END.length)
                    buffer.setLength(0)
                    buffer.append(before).append(after)
                    continue
                }
            }

            // 2. 查找可切分断点
            val currentStr = buffer.toString()
            val searchLen = if (startIdx != -1) startIdx else currentStr.length
            var splitIdx = -1

            for (i in 0 until searchLen) {
                val ch = currentStr[i]
                val currLen = i + 1
                val threshold = if (isFirstChunk || STRONG_PUNCTS.contains(ch)) {
                    minFirstLength
                } else {
                    minNextLength
                }

                if (currLen >= threshold) {
                    if (STRONG_PUNCTS.contains(ch)) {
                        // 浮点数保护：如果 '.' 前后都是数字，则不是句号断点
                        if (ch == '.' && i > 0 && i + 1 < searchLen &&
                            currentStr[i - 1].isDigit() && currentStr[i + 1].isDigit()
                        ) {
                            continue
                        }
                        splitIdx = i
                        break
                    } else if (WEAK_PUNCTS.contains(ch)) {
                        splitIdx = i
                        break
                    }
                }
            }

            if (splitIdx != -1) {
                val chunk = currentStr.substring(0, splitIdx + 1).trim()
                val remaining = currentStr.substring(splitIdx + 1)
                buffer.setLength(0)
                buffer.append(remaining)
                if (chunk.isNotEmpty()) {
                    sentences.add(chunk)
                    isFirstChunk = false
                }
            } else {
                break
            }
        }

        return sentences
    }

    /**
     * 流结束时清空并返回剩余未结算文本
     */
    @Synchronized
    fun flush(): List<String> {
        val sentences = mutableListOf<String>()
        var remaining = buffer.toString()
        // 清理可能未闭合或遗留的工具调用标签
        remaining = remaining.replace(Regex("<\\|tool_call\\|>.*?<\\|/tool_call\\|>", RegexOption.DOT_MATCHES_ALL), "")
        remaining = remaining.replace(Regex("<\\|/?tool_call\\|?>?"), "")
        remaining = cleanMarkdown(remaining).trim()

        if (remaining.isNotEmpty()) {
            sentences.add(remaining)
        }
        reset()
        return sentences
    }

    @Synchronized
    fun reset() {
        buffer.setLength(0)
        isFirstChunk = true
    }

    /**
     * 便捷方法：将完整文本一次性切分为流式短句
     */
    fun splitAll(text: String): List<String> {
        reset()
        val list = mutableListOf<String>()
        list.addAll(push(text))
        list.addAll(flush())
        return list
    }

    private fun cleanMarkdown(text: String): String {
        return MARKDOWN_PATTERN.matcher(text).replaceAll("")
    }
}
