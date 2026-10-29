package com.avatar.android.core

import com.avatar.android.core.audio.SentenceChunker
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Test

class SentenceChunkerTest {

    @Test
    fun testFirstChunk_ImmediateSplitOnShortPunctuation() {
        val chunker = SentenceChunker(minFirstLength = 3, minNextLength = 8)
        val fullText = "好的，马上为您调至二十四度。今天天气较热，建议开启自动内循环模式。"
        val chunks = chunker.splitAll(fullText)

        assertEquals(3, chunks.size)
        assertEquals("好的，", chunks[0])
        assertEquals("马上为您调至二十四度。", chunks[1])
        assertEquals("今天天气较热，建议开启自动内循环模式。", chunks[2])
    }

    @Test
    fun testStreamingTokenPush_EmitsIncrementally() {
        val chunker = SentenceChunker(minFirstLength = 3, minNextLength = 8)
        val tokens = listOf("好", "的", "，", "马", "上", "出", "发", "。")

        val emitted = mutableListOf<String>()
        for ((idx, tok) in tokens.withIndex()) {
            val res = chunker.push(tok)
            emitted.addAll(res)
            if (idx == 2) {
                // At "，", first short sentence must already be emitted
                assertEquals(1, emitted.size)
                assertEquals("好的，", emitted[0])
            }
        }
        emitted.addAll(chunker.flush())

        assertEquals(2, emitted.size)
        assertEquals("好的，", emitted[0])
        assertEquals("马上出发。", emitted[1])
    }

    @Test
    fun testToolCallInterception_StrippedFromSpeech() {
        val capturedTools = mutableListOf<String>()
        val chunker = SentenceChunker(
            minFirstLength = 3,
            minNextLength = 8,
            onToolCall = { capturedTools.add(it) }
        )

        val textWithTool = "好的，马上出发。<|tool_call|>{\"action\":\"start_nav\",\"dest\":\"西湖\"}<|/tool_call|>正在为您规划路线。"
        val chunks = chunker.splitAll(textWithTool)

        assertEquals(1, capturedTools.size)
        assertEquals("{\"action\":\"start_nav\",\"dest\":\"西湖\"}", capturedTools[0])

        for (c in chunks) {
            assertFalse("Speech chunks must not contain tool_call tags", c.contains("tool_call"))
            assertFalse("Speech chunks must not contain raw JSON", c.contains("start_nav"))
        }

        assertTrue(chunks.contains("好的，"))
        assertTrue(chunks.contains("马上出发。"))
        assertTrue(chunks.contains("正在为您规划路线。"))
    }

    @Test
    fun testMarkdownCleaning_StripsBoldAndHeaders() {
        val chunker = SentenceChunker()
        val text = "**好的**，### 这是标题。`code`"
        val chunks = chunker.splitAll(text)

        for (c in chunks) {
            assertFalse(c.contains("**"))
            assertFalse(c.contains("###"))
            assertFalse(c.contains("`"))
        }
    }

    @Test
    fun testFloatNumberProtection_DoesNotSplitOnDecimalPoint() {
        val chunker = SentenceChunker(minFirstLength = 3, minNextLength = 8)
        val text = "当前温度是24.5度，稍微有点偏高。请注意防暑！"
        val chunks = chunker.splitAll(text)

        assertEquals(2, chunks.size)
        assertEquals("当前温度是24.5度，稍微有点偏高。", chunks[0])
        assertEquals("请注意防暑！", chunks[1])
    }
}
