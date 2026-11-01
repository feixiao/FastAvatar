package com.ai.audioagent.tools

import com.ai.audioagent.tools.impl.WeatherQueryTool
import kotlinx.coroutines.runBlocking
import org.json.JSONObject
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test

class ToolSystemTest {

    @Before
    fun setUp() {
        ToolRegistry.clear()
    }

    @Test
    fun testToolRegistryRegistrationAndLookup() {
        val dummyTool = object : AgentTool {
            override val name: String = "test_action"
            override val description: String = "Test Description"
            override val parametersSchema: JSONObject = JSONObject().apply {
                put("type", "object")
            }
            override suspend fun execute(params: JSONObject): ToolExecutionResult {
                return ToolExecutionResult(
                    action = name,
                    success = true,
                    summary = "Executed successfully",
                    rawJson = params.toString()
                )
            }
        }

        ToolRegistry.register(dummyTool)
        assertEquals(1, ToolRegistry.getAllTools().size)

        val retrieved = ToolRegistry.get("test_action")
        assertNotNull(retrieved)
        assertEquals("test_action", retrieved?.name)
        assertEquals("Test Description", retrieved?.description)

        val schemaJson = ToolRegistry.exportToolSchemasForPrompt()
        assertTrue(schemaJson.contains("test_action"))
        assertTrue(schemaJson.contains("Test Description"))
    }

    @Test
    fun testWeatherQueryToolExecution() = runBlocking {
        val weatherTool = object : AgentTool {
            override val name: String = "query_weather"
            override val description: String = "查询天气"
            override val parametersSchema: JSONObject = JSONObject()
            override suspend fun execute(params: JSONObject): ToolExecutionResult {
                val city = params.optString("city", "杭州")
                return ToolExecutionResult(
                    action = name,
                    success = true,
                    summary = "🌤️ ${city}当前天气晴朗，气温22℃",
                    rawJson = params.toString(),
                    channel = "云端 OpenAPI 天气网关",
                    extras = mapOf("city" to city, "temp" to 22)
                )
            }
        }

        val params = JSONObject().apply {
            put("city", "上海")
        }

        val result = weatherTool.execute(params)
        assertTrue(result.success)
        assertEquals("query_weather", result.action)
        assertEquals("云端 OpenAPI 天气网关", result.channel)
        assertEquals("上海", result.extras["city"])
        assertTrue(result.summary.contains("上海当前天气晴朗"))
    }

    @Test
    fun testSystemAlarmToolSchemaAndMetadata() {
        val alarmTool = com.ai.audioagent.tools.impl.SystemAlarmTool(
            context = android.content.ContextWrapper(null)
        )
        assertEquals("set_alarm", alarmTool.name)
        assertEquals(RiskLevel.MEDIUM, alarmTool.riskLevel)

        val schema = alarmTool.parametersSchema
        val props = schema.optJSONObject("properties")
        assertNotNull(props)
        assertTrue(props!!.has("time"))
        val required = schema.optJSONArray("required")
        assertNotNull(required)
        assertEquals("time", required!!.getString(0))
    }

    @Test
    fun testMusicPlaybackToolSchemaAndMetadata() {
        val musicTool = com.ai.audioagent.tools.impl.MusicPlaybackTool(
            context = android.content.ContextWrapper(null)
        )
        assertEquals("play_music", musicTool.name)
        assertEquals(RiskLevel.LOW, musicTool.riskLevel)

        val schema = musicTool.parametersSchema
        val props = schema.optJSONObject("properties")
        assertNotNull(props)
        assertTrue(props!!.has("query"))
        assertTrue(props.has("artist"))
        assertTrue(props.has("command"))
        val required = schema.optJSONArray("required")
        assertNotNull(required)
        assertEquals("query", required!!.getString(0))
    }

    @Test
    fun testMusicPlaybackToolExecutionFallback() = runBlocking {
        val musicTool = com.ai.audioagent.tools.impl.MusicPlaybackTool(
            context = android.content.ContextWrapper(null)
        )
        val params = JSONObject().apply {
            put("query", "晴天")
            put("artist", "周杰伦")
            put("command", "play")
        }
        val result = musicTool.execute(params)
        assertTrue(result.success)
        assertEquals("play_music", result.action)
        assertTrue(result.summary.contains("晴天"))
    }

    @Test
    fun testMusicPlaybackToolControlCommands() = runBlocking {
        val musicTool = com.ai.audioagent.tools.impl.MusicPlaybackTool(
            context = android.content.ContextWrapper(null)
        )

        // 1. Test pause command
        val pauseParams = JSONObject().apply {
            put("query", "晴天")
            put("command", "pause")
        }
        val pauseResult = musicTool.execute(pauseParams)
        assertTrue(pauseResult.success)
        assertEquals("play_music", pauseResult.action)
        assertTrue(pauseResult.summary.contains("暂停播放"))
        assertEquals(android.view.KeyEvent.KEYCODE_MEDIA_PAUSE, pauseResult.extras["keyCode"])

        // 2. Test next command
        val nextParams = JSONObject().apply {
            put("query", "")
            put("command", "next")
        }
        val nextResult = musicTool.execute(nextParams)
        assertTrue(nextResult.success)
        assertTrue(nextResult.summary.contains("切到下一首"))
        assertEquals(android.view.KeyEvent.KEYCODE_MEDIA_NEXT, nextResult.extras["keyCode"])

        // 3. Test resume command
        val resumeParams = JSONObject().apply {
            put("query", "")
            put("command", "resume")
        }
        val resumeResult = musicTool.execute(resumeParams)
        assertTrue(resumeResult.success)
        assertTrue(resumeResult.summary.contains("恢复播放"))
        assertEquals(android.view.KeyEvent.KEYCODE_MEDIA_PLAY, resumeResult.extras["keyCode"])
    }
}
