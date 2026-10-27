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
    fun testMockVehicleCommands_RoutedToMockVehicle() {
        val climateCmd = "把空调调到24度"
        val res1 = dispatcher.evaluate(climateCmd, isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.MOCK_VEHICLE, res1.target)
        assertEquals("climate.set_temperature", res1.intentName)
        assertEquals(24.0, res1.slots["target_temperature"])

        val windowCmd = "打开副驾车窗"
        val res2 = dispatcher.evaluate(windowCmd, isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.MOCK_VEHICLE, res2.target)
        assertEquals("window.control", res2.intentName)

        val modeCmd = "切换到运动模式"
        val res3 = dispatcher.evaluate(modeCmd, isCloudConsented = true, isOnline = true)
        assertEquals(RouteTarget.MOCK_VEHICLE, res3.target)
        assertEquals("drive.switch_mode", res3.intentName)
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
