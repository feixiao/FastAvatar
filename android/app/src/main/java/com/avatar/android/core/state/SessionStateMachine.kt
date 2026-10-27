package com.avatar.android.core.state

import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.diagnostics.DiagnosticLogger
import com.avatar.android.core.dispatcher.FastDispatcher
import com.avatar.android.core.dispatcher.RouteTarget
import com.avatar.android.core.memory.MemoryVault
import com.avatar.android.core.mockvehicle.MockVehicleEngine
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import java.util.UUID

data class TurnResult(
    val turnId: String,
    val userText: String,
    val routeTarget: RouteTarget,
    val responseText: String,
    val latencyMs: Long,
    val isInterrupted: Boolean = false
)

class SessionStateMachine(
    val dispatcher: FastDispatcher,
    val vehicleEngine: MockVehicleEngine,
    val memoryVault: MemoryVault,
    val cloudClient: CloudClient,
    val playbackController: AudioPlaybackController,
    val diagnosticLogger: DiagnosticLogger,
    private val scope: CoroutineScope = CoroutineScope(Dispatchers.Default + SupervisorJob())
) {
    private val _state = MutableStateFlow(SessionState.STOPPED)
    val state: StateFlow<SessionState> = _state.asStateFlow()

    private val _turnResults = MutableSharedFlow<TurnResult>()
    val turnResults: SharedFlow<TurnResult> = _turnResults.asSharedFlow()

    var isCloudConsented: Boolean = false
    var isOnline: Boolean = true

    @Volatile
    private var activeTurnJob: Job? = null
    @Volatile
    var activeTurnId: String? = null
        private set

    fun startSession(hasMicPermission: Boolean): Boolean {
        if (_state.value != SessionState.STOPPED && _state.value != SessionState.PAUSED) {
            return false
        }

        _state.value = SessionState.PREPARING

        if (!hasMicPermission) {
            _state.value = SessionState.PAUSED
            diagnosticLogger.logEvent("client_session_interrupted", mapOf("reason" to "mic_permission_missing"))
            return false
        }

        _state.value = SessionState.LISTENING
        diagnosticLogger.logEvent("client_session_started", mapOf(
            "platform" to "pixel_10_android_16",
            "cloud_consent" to isCloudConsented,
            "is_online" to isOnline
        ))
        return true
    }

    fun stopSession() {
        val prev = _state.value
        if (prev == SessionState.STOPPED) return

        interrupt(reason = "session_stopped")
        scope.launch {
            cloudClient.closeSession()
        }

        _state.value = SessionState.STOPPED
        diagnosticLogger.logEvent("client_session_interrupted", mapOf("reason" to "user_stopped"))
    }

    fun pauseSession(reason: String) {
        if (_state.value == SessionState.STOPPED || _state.value == SessionState.PAUSED) return

        interrupt(reason = reason)
        _state.value = SessionState.PAUSED
        diagnosticLogger.logEvent("client_session_interrupted", mapOf("reason" to reason))
    }

    fun resumeSession(hasMicPermission: Boolean): Boolean {
        if (_state.value != SessionState.PAUSED) return false
        return startSession(hasMicPermission)
    }

    /**
     * Submit user utterance into the state machine.
     */
    fun submitUtterance(text: String, simulateCloudResponse: String? = null) {
        val currentState = _state.value
        if (currentState != SessionState.LISTENING &&
            currentState != SessionState.SPEAKING &&
            currentState != SessionState.LOCAL_ONLY &&
            currentState != SessionState.OFFLINE) {
            return
        }

        // If currently speaking, auto-interrupt old turn before handling new input
        if (currentState == SessionState.SPEAKING || activeTurnJob?.isActive == true) {
            interrupt(reason = "barge_in_new_input")
        }

        val turnId = "turn_${UUID.randomUUID().toString().take(8)}"
        activeTurnId = turnId

        activeTurnJob = scope.launch {
            val startTime = System.currentTimeMillis()
            _state.value = SessionState.ROUTING

            val dispatchResult = dispatcher.evaluate(
                text = text,
                isCloudConsented = isCloudConsented,
                isOnline = isOnline
            )

            when (dispatchResult.target) {
                RouteTarget.LOCAL_MEMORY -> {
                    _state.value = SessionState.LOCAL_PROCESSING
                    val cmdResult = memoryVault.parseMemoryCommand(text)
                    val elapsed = System.currentTimeMillis() - startTime

                    diagnosticLogger.logEvent("local_memory_operation", mapOf(
                        "action" to cmdResult.action.name,
                        "success" to true
                    ))

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = cmdResult.feedback,
                        latencyMs = elapsed
                    )
                }

                RouteTarget.MOCK_VEHICLE -> {
                    _state.value = SessionState.LOCAL_PROCESSING
                    val execResult = vehicleEngine.execute(
                        intent = dispatchResult.intentName,
                        slots = dispatchResult.slots
                    )
                    val elapsed = System.currentTimeMillis() - startTime

                    diagnosticLogger.logEvent("mock_action_recorded", mapOf(
                        "intent" to execResult.entry.intent,
                        "decision" to execResult.entry.decision.name,
                        "reason" to execResult.entry.reason,
                        "simulation" to execResult.entry.simulation
                    ))

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = execResult.confirmation,
                        latencyMs = elapsed
                    )
                }

                RouteTarget.LOCAL_PERSONA -> {
                    _state.value = SessionState.LOCAL_PROCESSING
                    val response = "已为您切换至该陪伴人设体验。"
                    val elapsed = System.currentTimeMillis() - startTime

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = response,
                        latencyMs = elapsed
                    )
                }

                RouteTarget.LOCAL_ONLY_PROMPT -> {
                    _state.value = SessionState.LOCAL_ONLY
                    val response = "云端对话处理尚未启用，您可在设置中授权开启。当前仍为您提供本地功能。"
                    val elapsed = System.currentTimeMillis() - startTime

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = response,
                        latencyMs = elapsed
                    )
                }

                RouteTarget.OFFLINE_PROMPT -> {
                    _state.value = SessionState.OFFLINE
                    val response = "当前处于离线状态，云端对话暂不可用。本地记忆与车控指令仍可正常使用。"
                    val elapsed = System.currentTimeMillis() - startTime

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = response,
                        latencyMs = elapsed
                    )
                }

                RouteTarget.CLOUD_CHAT -> {
                    _state.value = SessionState.CLOUD_CONNECTING

                    // Check token / session
                    if (cloudClient.currentRoom.isEmpty()) {
                        val tokenRes = cloudClient.acquireSessionToken(isCloudConsented)
                        if (tokenRes.isFailure) {
                            _state.value = SessionState.LISTENING
                            _turnResults.emit(
                                TurnResult(
                                    turnId = turnId,
                                    userText = text,
                                    routeTarget = dispatchResult.target,
                                    responseText = "云端服务连接失败，请检查网络后重试。",
                                    latencyMs = System.currentTimeMillis() - startTime
                                )
                            )
                            return@launch
                        }
                    }

                    _state.value = SessionState.CLOUD_TURN
                    cloudClient.startTurn(turnId)

                    // Cloud response synthesis (or custom simulated payload)
                    val cloudReply = simulateCloudResponse
                        ?: "海水呈现蓝色主要是因为水分子对波长较长的红光吸收较强，而对波长较短的蓝光发生强烈散射。"

                    val elapsed = System.currentTimeMillis() - startTime
                    diagnosticLogger.logEvent("client_turn_completed", mapOf(
                        "route" to "CLOUD_CHAT",
                        "latency_ms" to elapsed,
                        "result" to "success"
                    ))

                    cloudClient.finishTurn(turnId)

                    playAndEmitResult(
                        turnId = turnId,
                        userText = text,
                        target = dispatchResult.target,
                        response = cloudReply,
                        latencyMs = elapsed
                    )
                }
            }
        }
    }

    private suspend fun playAndEmitResult(
        turnId: String,
        userText: String,
        target: RouteTarget,
        response: String,
        latencyMs: Long
    ) {
        _state.value = SessionState.SPEAKING
        playbackController.startPlayback(turnId, response)

        _turnResults.emit(
            TurnResult(
                turnId = turnId,
                userText = userText,
                routeTarget = target,
                responseText = response,
                latencyMs = latencyMs
            )
        )

        // Mark naturally completed unless interrupted
        if (playbackController.currentTurnId == turnId) {
            playbackController.finishPlayback()
            _state.value = SessionState.LISTENING
        }
    }

    /**
     * Barge-in interruption (< 80ms local stop & cloud interruption).
     */
    fun interrupt(reason: String = "user_barge_in"): Long {
        val currentTid = activeTurnId
        activeTurnJob?.cancel()
        activeTurnJob = null
        activeTurnId = null

        val stopLatency = playbackController.stopPlayback(reason = reason)

        currentTid?.let { tid ->
            scope.launch {
                cloudClient.interruptActiveTurn(tid, reason)
            }
            diagnosticLogger.logEvent("client_turn_cancelled", mapOf(
                "turn_id" to tid,
                "reason" to reason,
                "stop_latency_ms" to stopLatency
            ))
        }

        if (_state.value != SessionState.STOPPED && _state.value != SessionState.PAUSED) {
            _state.value = SessionState.LISTENING
        }

        return stopLatency
    }
}
