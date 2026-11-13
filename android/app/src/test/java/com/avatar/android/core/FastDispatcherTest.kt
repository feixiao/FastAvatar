package com.avatar.android.core

import com.avatar.android.core.dispatcher.FastDispatcher
import com.avatar.android.core.dispatcher.RouteTarget
import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class FastDispatcherTest {

    private lateinit var dispatcher: FastDispatcher

    @Before
    fun setUp() {
        dispatcher = FastDispatcher()
    }

    @Test
    fun testMemoryOralCommands_RoutedToLocalMemory() {
        val commands = listOf(
            "记住我喜欢喝美式咖啡",
            "帮我记一下明天下午三点要开会",
            "记一下我妈妈的生日是五月二号",
            "忘掉美式咖啡",
            "忘了我的生日",
            "清除记忆"
        )

        for (cmd in commands) {
            val res = dispatcher.evaluate(cmd, isCloudConsented = true, isOnline = true)
            assertEquals("Expected LOCAL_MEMORY for '$cmd'", RouteTarget.LOCAL_MEMORY, res.target)
            assertEquals("memory.command", res.intentName)
        }
    }

    @Test
    fun testPersonaSwitchCommands_RoutedToLocalPersona() {
        val cmd = "切换成高冷猫娘人设"
        val res = dispatcher.evaluate(cmd, isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.LOCAL_PERSONA, res.target)
        assertEquals("persona.switch", res.intentName)
    }

    @Test
    fun testOpenEndedDialog_CloudConsentedAndOnline_RoutedToCloudChat() {
        val query = "为什么海水是蓝色的？"
        val res = dispatcher.evaluate(query, isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.CLOUD_CHAT, res.target)
        assertEquals("dialog.open_ended", res.intentName)
    }

    @Test
    fun testOpenEndedDialog_NotConsented_RoutedToLocalOnly() {
        val query = "为什么海水是蓝色的？"
        val res = dispatcher.evaluate(query, isCloudConsented = false, isOnline = true)
        assertEquals(RouteTarget.LOCAL_ONLY_PROMPT, res.target)
    }

    @Test
    fun testOpenEndedDialog_Offline_RoutedToOfflinePrompt() {
        val query = "为什么海水是蓝色的？"
        val res = dispatcher.evaluate(query, isCloudConsented = true, isOnline = false)
        assertEquals(RouteTarget.OFFLINE_PROMPT, res.target)
    }


    @Test
    fun testPetActionCommands_RoutedToLocalPetAction() {
        val resSit = dispatcher.evaluate("蹲下", isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.LOCAL_PET_ACTION, resSit.target)
        assertEquals("sit", resSit.slots["action"])

        val resJump = dispatcher.evaluate("跳一下", isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.LOCAL_PET_ACTION, resJump.target)
        assertEquals("jump", resJump.slots["action"])
    }

    @Test
    fun testTinyClassifierAndDSTContextInheritance() {
        // 1. 短句数理公式 -> need_cloud = true
        val mathRes = dispatcher.evaluate("求导 x^3·lnx", isCloudConsented = true, isOnline = true)
        assertEquals(true, mathRes.slots["need_cloud"])
        assertEquals("math", mathRes.slots["primary_domain"])

        // 2. 长句生活闲聊 -> need_cloud = false (杜绝字数一刀切)
        val storyRes = dispatcher.evaluate(
            "今天我和朋友去公园散步看到一只流浪小猫好开心",
            isCloudConsented = true,
            isOnline = true
        )
        assertEquals(false, storyRes.slots["need_cloud"])
        assertEquals("chat", storyRes.slots["primary_domain"])

        // 3. 多轮 DST 上下文继承: 上一轮云端写代码 -> 本轮短代词追问 "第三行是什么意思" 继承云端
        dispatcher.updateContext("CLOUD_CHAT", "code")
        val followupRes = dispatcher.evaluate("第三行是什么意思", isCloudConsented = true, isOnline = true)
        assertEquals(true, followupRes.slots["need_cloud"])
        assertEquals("context_followup", followupRes.slots["primary_domain"])
    }
    @Test
    fun testDispatcherEvaluationLatency_Under5ms() {
        val query = "把空调调到22.5度"
        // Warmup
        repeat(50) { dispatcher.evaluate(query, isCloudConsented = true, isOnline = true) }

        val start = System.nanoTime()
        val res = dispatcher.evaluate(query, isCloudConsented = true, isOnline = true)
        val elapsedNano = System.nanoTime() - start
        val elapsedMs = elapsedNano / 1_000_000.0

        assertTrue("Dispatcher latency should be < 5ms, actual: $elapsedMs ms", elapsedMs < 5.0)
    }
}
