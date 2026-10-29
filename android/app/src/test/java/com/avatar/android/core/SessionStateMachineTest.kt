package com.avatar.android.core

import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.diagnostics.DiagnosticLogger
import com.avatar.android.core.dispatcher.FastDispatcher
import com.avatar.android.core.dispatcher.RouteTarget
import com.avatar.android.core.memory.MemoryVault
import com.avatar.android.core.mockvehicle.MockVehicleEngine
import com.avatar.android.core.state.SessionState
import com.avatar.android.core.state.SessionStateMachine
import com.avatar.android.core.state.TurnResult
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.TestScope
import kotlinx.coroutines.test.UnconfinedTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.After
import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.io.File

@OptIn(ExperimentalCoroutinesApi::class)
class SessionStateMachineTest {

    private lateinit var tempDir: File
    private lateinit var dispatcher: FastDispatcher
    private lateinit var vehicleEngine: MockVehicleEngine
    private lateinit var memoryVault: MemoryVault
    private lateinit var cloudClient: CloudClient
    private lateinit var playbackController: AudioPlaybackController
    private lateinit var diagnosticLogger: DiagnosticLogger
    private lateinit var stateMachine: SessionStateMachine

    private val testDispatcher = UnconfinedTestDispatcher()
    private val testScope = TestScope(testDispatcher)

    @Before
    fun setUp() {
        tempDir = File.createTempFile("avatar_sm_test", "_dir")
        tempDir.delete()
        tempDir.mkdirs()

        val logFile = File(tempDir, "mock_vehicle.log")
        val memFile = File(tempDir, "memory.json")
        val diagFile = File(tempDir, "diagnostics.log")

        dispatcher = FastDispatcher()
        vehicleEngine = MockVehicleEngine(logFile)
        memoryVault = MemoryVault(memFile, "test_device")
        playbackController = AudioPlaybackController()
        diagnosticLogger = DiagnosticLogger(diagFile)
        cloudClient = CloudClient("http://localhost:8088", "user_1", "dev_1", "pixel_10")

        stateMachine = SessionStateMachine(
            dispatcher = dispatcher,
            vehicleEngine = vehicleEngine,
            memoryVault = memoryVault,
            cloudClient = cloudClient,
            playbackController = playbackController,
            diagnosticLogger = diagnosticLogger,
            scope = testScope
        )
    }

    @After
    fun tearDown() {
        tempDir.deleteRecursively()
    }

    @Test
    fun testStartSession_WithoutPermission_TransitionsToPaused() {
        val started = stateMachine.startSession(hasMicPermission = false)
        assertFalse(started)
        assertEquals(SessionState.PAUSED, stateMachine.state.value)
    }

    @Test
    fun testStartSession_WithPermission_TransitionsToListening() {
        val started = stateMachine.startSession(hasMicPermission = true)
        assertTrue(started)
        assertEquals(SessionState.LISTENING, stateMachine.state.value)
    }

    @Test
    fun testLocalFastPath_VehicleControl_ExecutionAndAudit() = testScope.runTest {
        stateMachine.startSession(hasMicPermission = true)
        stateMachine.submitUtterance("把空调调到24度")

        // Execution completes and returns to LISTENING
        assertEquals(SessionState.LISTENING, stateMachine.state.value)

        // Check vehicle audit log
        val logs = vehicleEngine.getAuditLogs()
        assertEquals(1, logs.size)
        assertEquals("climate.set_temperature", logs[0].intent)
        assertEquals(24.0, logs[0].slots["target_temperature"])
        assertTrue(logs[0].simulation)
    }

    @Test
    fun testBargeInInterruption_StopsPlaybackWithin80ms() = testScope.runTest {
        stateMachine.startSession(hasMicPermission = true)
        playbackController.startPlayback("turn_123", "长回复文本正在播放中...")

        assertTrue(playbackController.isPlaying.value)

        // Simulate Barge-In
        val latencyMs = stateMachine.interrupt(reason = "user_barge_in")

        assertFalse("Playback should be stopped", playbackController.isPlaying.value)
        assertTrue("Interruption stop latency should be <= 80ms, actual: $latencyMs ms", latencyMs <= 80)
        assertEquals(SessionState.LISTENING, stateMachine.state.value)
    }

    @Test
    fun testBargeInInterruption_RollsBackTurnResultContext() = testScope.runTest {
        stateMachine.startSession(hasMicPermission = true)

        val turnResults = mutableListOf<TurnResult>()
        val job = testScope.launch {
            stateMachine.turnResults.collect {
                turnResults.add(it)
            }
        }

        // 提交自定义云端回复的长文本
        stateMachine.submitUtterance(
            text = "讲个长故事",
            simulateCloudResponse = "很久很久以前，在一个遥远的山谷里，住着一只可爱的小松鼠。它每天都会收集松果。"
        )

        // 此时已开始播放并发出首个 TurnResult (isInterrupted = false)
        assertTrue(turnResults.isNotEmpty())
        assertEquals(false, turnResults.last().isInterrupted)

        // 触发插话打断
        stateMachine.interrupt(reason = "user_barge_in")

        // 验证打断后发射了回滚后的 TurnResult (isInterrupted = true)
        val lastResult = turnResults.last()
        assertTrue("Interrupted result must have isInterrupted = true", lastResult.isInterrupted)
        assertTrue(
            "Rolled back text should contain interruption suffix",
            lastResult.responseText.contains("（被用户打断）")
        )

        job.cancel()
    }

    @Test
    fun testOfflineDegradation_LocalPathStillAvailable() = testScope.runTest {
        stateMachine.isOnline = false
        stateMachine.startSession(hasMicPermission = true)

        // 1. Vehicle command in offline mode works 100% locally
        stateMachine.submitUtterance("打开车窗")
        assertEquals(SessionState.LISTENING, stateMachine.state.value)
        assertEquals(1, vehicleEngine.getAuditLogs().size)

        // 2. Open-ended conversational query in offline mode prompts offline warning, without queueing
        stateMachine.submitUtterance("今天天气怎么样？")
        // After notice, returns to LISTENING
        assertEquals(SessionState.LISTENING, stateMachine.state.value)
    }

    @Test
    fun testDiagnosticLogging_ZeroContentLeak() {
        stateMachine.startSession(hasMicPermission = true)
        stateMachine.submitUtterance("把空调调到23度")
        stateMachine.interrupt(reason = "test_cancel")
        stateMachine.stopSession()

        val events = diagnosticLogger.getEvents()
        assertTrue(events.isNotEmpty())

        val eventNames = events.map { it.event }
        assertTrue(eventNames.contains("client_session_started"))
        assertTrue(eventNames.contains("mock_action_recorded"))
        assertTrue(eventNames.contains("client_turn_cancelled"))
        assertTrue(eventNames.contains("client_session_interrupted"))

        // Check data minimization: NO transcripts or audio base64
        for (event in events) {
            assertFalse(event.properties.containsKey("audio"))
            assertFalse(event.properties.containsKey("transcript"))
        }
    }
}
