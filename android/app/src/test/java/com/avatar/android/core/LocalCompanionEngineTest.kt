package com.avatar.android.core

import com.avatar.android.core.dialog.LocalCompanionEngine
import com.avatar.android.core.memory.MemoryVault
import org.junit.After
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File

class LocalCompanionEngineTest {

    private lateinit var tempDir: File
    private lateinit var memoryVault: MemoryVault

    @Before
    fun setUp() {
        tempDir = File.createTempFile("avatar_companion_test", "_dir")
        tempDir.delete()
        tempDir.mkdirs()
        val memFile = File(tempDir, "memory.json")
        memoryVault = MemoryVault(memFile, "test_device")
    }

    @After
    fun tearDown() {
        tempDir.deleteRecursively()
    }

    @Test
    fun testGreetingResponses_ProvidesWarmCompanionReplies() {
        val greetings = listOf("你好", "早上好", "在吗", "嗨")
        for (g in greetings) {
            val reply = LocalCompanionEngine.generateResponse(g, persona = "companion", memoryVault = memoryVault)
            assertTrue("Reply should be non-empty for '$g'", reply.isNotBlank())
            assertFalse("Should not be generic static error", reply.contains("云端对话处理尚未启用"))
        }
    }

    @Test
    fun testEmotionComfort_ProvidesEmpathy() {
        val reply = LocalCompanionEngine.generateResponse("今天工作好累啊，心情好烦", persona = "companion", memoryVault = memoryVault)
        assertTrue(reply.contains("抱抱") || reply.contains("辛苦") || reply.contains("放松"))
    }

    @Test
    fun testPersonaSwitching_ProducesDifferentTones() {
        val input = "你好呀"
        val companionReply = LocalCompanionEngine.generateResponse(input, persona = "companion", memoryVault = memoryVault)
        val roasterReply = LocalCompanionEngine.generateResponse(input, persona = "roaster", memoryVault = memoryVault)
        val jarvisReply = LocalCompanionEngine.generateResponse(input, persona = "jarvis", memoryVault = memoryVault)
        val petReply = LocalCompanionEngine.generateResponse(input, persona = "pet", memoryVault = memoryVault)

        assertTrue(roasterReply.contains("理我") || roasterReply.contains("哟") || roasterReply.contains("好玩"))
        assertTrue(jarvisReply.contains("顾问") || jarvisReply.contains("分析") || jarvisReply.contains("您好"))
        assertTrue(petReply.contains("喵呜") || petReply.contains("主人"))
        assertTrue(companionReply.contains("伴侣") || companionReply.contains("嗨"))
    }

    @Test
    fun testJokeAndEntertainment_ReturnsJoke() {
        val reply = LocalCompanionEngine.generateResponse("讲个笑话开心一下", persona = "companion", memoryVault = memoryVault)
        assertTrue(reply.contains("冷笑话") || reply.contains("小鸭") || reply.contains("草莓") || reply.contains("咸鱼") || reply.contains("企鹅"))
    }

    @Test
    fun testPettingInteraction_ProducesTouchReaction() {
        val reply = LocalCompanionEngine.generateResponse("摸摸头", persona = "pet", memoryVault = memoryVault)
        assertTrue(reply.contains("喵呜") || reply.contains("手心") || reply.contains("摸摸头"))
    }
}
