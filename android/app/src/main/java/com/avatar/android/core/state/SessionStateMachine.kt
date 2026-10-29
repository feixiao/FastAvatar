package com.avatar.android.core.state

import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.audio.SentenceChunker
import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.diagnostics.DiagnosticLogger
import com.avatar.android.core.dialog.LocalCompanionEngine
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

    var activePersona: String = "companion"
    var customLlmUrl: String = ""
    var customLlmKey: String = ""
    var customLlmModel: String = ""

    @Volatile
    private var activeTurnJob: Job? = null
    @Volatile
    var activeTurnId: String? = null
        private set
    @Volatile
    var activeUserText: String = ""
        private set
    @Volatile
    var activeRouteTarget: RouteTarget? = null
        private set
    @Volatile
    var activeTurnStartTime: Long = 0
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
        if (currentState == SessionState.STOPPED) {
            startSession(hasMicPermission = true)
        }

        val refreshedState = _state.value
        if (refreshedState != SessionState.LISTENING &&
            refreshedState != SessionState.SPEAKING &&
            refreshedState != SessionState.LOCAL_ONLY &&
            refreshedState != SessionState.OFFLINE) {
            return
        }

        // If currently speaking, auto-interrupt old turn before handling new input
        if (refreshedState == SessionState.SPEAKING || activeTurnJob?.isActive == true) {
            interrupt(reason = "barge_in_new_input")
        }

        val turnId = "turn_${UUID.randomUUID().toString().take(8)}"
        activeTurnId = turnId
        activeUserText = text
        activeTurnStartTime = System.currentTimeMillis()
        activeTurnJob = scope.launch {
            val startTime = System.currentTimeMillis()
            _state.value = SessionState.ROUTING

            val dispatchResult = dispatcher.evaluate(
                text = text,
                isCloudConsented = isCloudConsented,
                isOnline = isOnline
            )

            activeRouteTarget = dispatchResult.target
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
                    val newPersona = when {
                        text.contains("猫娘") || text.contains("宠物") || text.contains("可爱") -> "pet"
                        text.contains("毒舌") || text.contains("损友") -> "roaster"
                        text.contains("幕僚") || text.contains("助理") || text.contains("顾问") || text.contains("效率") -> "jarvis"
                        else -> "companion"
                    }
                    activePersona = newPersona
                    val response = when (newPersona) {
                        "roaster" -> "已为您切换至该陪伴人设体验。做好心理准备，毒舌损友可不会再对你客客气气哦！"
                        "jarvis" -> "已为您切换至该陪伴人设体验。高效幕僚模式已就绪，直入主题拒绝冗余。"
                        "pet" -> "已为您切换至该陪伴人设体验。喵呜~ 元气宠物小精灵上线啦！"
                        else -> "已为您切换至该陪伴人设体验。"
                    }
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

                    if (simulateCloudResponse != null) {
                        _state.value = SessionState.CLOUD_TURN
                        cloudClient.startTurn(turnId)
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
                            response = simulateCloudResponse,
                            latencyMs = elapsed
                        )
                    } else if (customLlmUrl.isNotBlank()) {
                        // 流式 Token 接收 + 标点切句极速首音播放
                        try {
                            _state.value = SessionState.CLOUD_TURN
                            cloudClient.startTurn(turnId)
                            val streamChunker = SentenceChunker()
                            val fullResponse = StringBuilder()
                            var isPlaybackStarted = false

                            playbackController.startStreamingPlayback(turnId) {
                                if (activeTurnId == turnId && _state.value == SessionState.SPEAKING) {
                                    _state.value = SessionState.LISTENING
                                    activeTurnId = null
                                }
                            }

                            cloudClient.streamChatMessage(
                                turnId = turnId,
                                message = text,
                                persona = activePersona,
                                customLlmUrl = customLlmUrl,
                                customLlmKey = customLlmKey,
                                customLlmModel = customLlmModel
                            ).collect { token ->
                                fullResponse.append(token)
                                val chunks = streamChunker.push(token)
                                for (chunk in chunks) {
                                    if (!isPlaybackStarted) {
                                        _state.value = SessionState.SPEAKING
                                        isPlaybackStarted = true
                                    }
                                    playbackController.enqueueChunk(turnId, chunk)
                                }
                            }

                            val remainingChunks = streamChunker.flush()
                            for (chunk in remainingChunks) {
                                if (!isPlaybackStarted) {
                                    _state.value = SessionState.SPEAKING
                                    isPlaybackStarted = true
                                }
                                playbackController.enqueueChunk(turnId, chunk)
                            }
                            playbackController.finishStreaming(turnId)

                            val elapsed = System.currentTimeMillis() - startTime
                            diagnosticLogger.logEvent("client_turn_completed", mapOf(
                                "route" to "CLOUD_CHAT",
                                "latency_ms" to elapsed,
                                "result" to "success"
                            ))
                            cloudClient.finishTurn(turnId)

                            _turnResults.emit(
                                TurnResult(
                                    turnId = turnId,
                                    userText = text,
                                    routeTarget = dispatchResult.target,
                                    responseText = fullResponse.toString(),
                                    latencyMs = elapsed,
                                    isInterrupted = false
                                )
                            )

                            if (!playbackController.hasActiveSynthesizer && playbackController.currentTurnId == turnId) {
                                playbackController.finishPlayback()
                                _state.value = SessionState.LISTENING
                                activeTurnId = null
                            }
                        } catch (e: Exception) {
                            // 优雅降级到本地伴生闲聊引擎
                            val fallbackReply = LocalCompanionEngine.generateResponse(
                                userInput = text,
                                persona = activePersona,
                                memoryVault = memoryVault
                            )
                            val elapsed = System.currentTimeMillis() - startTime
                            playAndEmitResult(
                                turnId = turnId,
                                userText = text,
                                target = dispatchResult.target,
                                response = fallbackReply,
                                latencyMs = elapsed
                            )
                        }
                    } else {
                        // 2. 标准云端调用或本地伴生兜底
                        val remoteResult = cloudClient.sendChatMessage(
                            turnId = turnId,
                            message = text,
                            persona = activePersona,
                            customLlmUrl = customLlmUrl,
                            customLlmKey = customLlmKey,
                            customLlmModel = customLlmModel
                        )

                        val cloudReply = if (remoteResult.isSuccess) {
                            remoteResult.getOrThrow()
                        } else {
                            LocalCompanionEngine.generateResponse(
                                userInput = text,
                                persona = activePersona,
                                memoryVault = memoryVault
                            )
                        }

                        _state.value = SessionState.CLOUD_TURN
                        cloudClient.startTurn(turnId)

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
    }
    private suspend fun playAndEmitResult(
        turnId: String,
        userText: String,
        target: RouteTarget,
        response: String,
        latencyMs: Long
    ) {
        _state.value = SessionState.SPEAKING
        activeRouteTarget = target

        playbackController.startPlayback(turnId, response) {
            // Natural TTS finish callback
            if (activeTurnId == turnId && _state.value == SessionState.SPEAKING) {
                _state.value = SessionState.LISTENING
                activeTurnId = null
            }
        }

        _turnResults.emit(
            TurnResult(
                turnId = turnId,
                userText = userText,
                routeTarget = target,
                responseText = response,
                latencyMs = latencyMs,
                isInterrupted = false
            )
        )

        // Mark naturally completed immediately if no active synthesizer is playing
        if (!playbackController.hasActiveSynthesizer && playbackController.currentTurnId == turnId) {
            playbackController.finishPlayback()
            _state.value = SessionState.LISTENING
            activeTurnId = null
        }
    }

    /**
     * Barge-in interruption (< 80ms local stop & cloud interruption).
     */
    fun interrupt(reason: String = "user_barge_in"): Long {
        val currentTid = activeTurnId
        val currentUserText = activeUserText
        val currentRoute = activeRouteTarget ?: RouteTarget.CLOUD_CHAT
        val wasSpeaking = _state.value == SessionState.SPEAKING
        val wasActiveTurn = currentTid != null

        activeTurnJob?.cancel()
        activeTurnJob = null
        activeTurnId = null

        val spokenText = playbackController.getSpokenText()
        val stopLatency = playbackController.stopPlayback(reason = reason)

        if (wasActiveTurn && currentTid != null) {
            scope.launch {
                cloudClient.interruptActiveTurn(currentTid, reason)
            }
            diagnosticLogger.logEvent("client_turn_cancelled", mapOf(
                "turn_id" to currentTid,
                "reason" to reason,
                "stop_latency_ms" to stopLatency
            ))

            // 精准打断记忆回滚：实际听到的内容 == 模型记住的内容
            val rolledBackText = if (wasSpeaking && spokenText.isNotBlank()) {
                "$spokenText（被用户打断）"
            } else {
                "（被用户打断）"
            }

            scope.launch {
                _turnResults.emit(
                    TurnResult(
                        turnId = currentTid,
                        userText = currentUserText,
                        routeTarget = currentRoute,
                        responseText = rolledBackText,
                        latencyMs = stopLatency,
                        isInterrupted = true
                    )
                )
            }
        }

        if (_state.value != SessionState.STOPPED && _state.value != SessionState.PAUSED) {
            _state.value = SessionState.LISTENING
        }

        return stopLatency
    }
}
