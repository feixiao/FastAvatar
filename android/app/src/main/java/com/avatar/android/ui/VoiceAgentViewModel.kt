package com.avatar.android.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.audio.AudioPrivacyGuard
import com.avatar.android.core.audio.SpeechRecognitionHelper
import com.avatar.android.core.cloud.CloudClient
import com.avatar.android.core.config.AppSettings
import com.avatar.android.core.dispatcher.RouteTarget
import com.avatar.android.core.memory.MemoryItem
import com.avatar.android.core.memory.MemoryVault
import com.avatar.android.core.memory.PendingMemory
import com.avatar.android.core.mockvehicle.AuditLogEntry
import com.avatar.android.core.mockvehicle.MockVehicleEngine
import com.avatar.android.core.state.SessionState
import com.avatar.android.core.state.SessionStateMachine
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch

data class MessageItem(
    val id: String,
    val sender: MessageSender,
    val text: String,
    val routeTarget: RouteTarget? = null,
    val latencyMs: Long? = null,
    val isInterrupted: Boolean = false,
    val timestamp: Long = System.currentTimeMillis()
)

enum class MessageSender {
    USER,
    ASSISTANT,
    SYSTEM
}

class VoiceAgentViewModel(
    val stateMachine: SessionStateMachine,
    val memoryVault: MemoryVault,
    val vehicleEngine: MockVehicleEngine,
    val playbackController: AudioPlaybackController,
    val privacyGuard: AudioPrivacyGuard,
    val cloudClient: CloudClient,
    val deviceId: String,
    val appSettings: AppSettings? = null,
    val speechRecognitionHelper: SpeechRecognitionHelper? = null
) : ViewModel() {

    val sessionState: StateFlow<SessionState> = stateMachine.state

    private val _messages = MutableStateFlow<List<MessageItem>>(emptyList())
    val messages: StateFlow<List<MessageItem>> = _messages.asStateFlow()

    private val _isCloudConsented = MutableStateFlow(stateMachine.isCloudConsented)
    val isCloudConsented: StateFlow<Boolean> = _isCloudConsented.asStateFlow()

    private val _isOnline = MutableStateFlow(stateMachine.isOnline)
    val isOnline: StateFlow<Boolean> = _isOnline.asStateFlow()

    private val _pendingMemory = MutableStateFlow<PendingMemory?>(null)
    val pendingMemory: StateFlow<PendingMemory?> = _pendingMemory.asStateFlow()

    private val _memories = MutableStateFlow<List<MemoryItem>>(emptyList())
    val memories: StateFlow<List<MemoryItem>> = _memories.asStateFlow()

    private val _auditLogs = MutableStateFlow<List<AuditLogEntry>>(emptyList())
    val auditLogs: StateFlow<List<AuditLogEntry>> = _auditLogs.asStateFlow()

    private val _activePersona = MutableStateFlow(stateMachine.activePersona)
    val activePersona: StateFlow<String> = _activePersona.asStateFlow()

    val isAudioMuted: Boolean
        get() = cloudClient.isAudioMuted

    val isPlaying: StateFlow<Boolean> = playbackController.isPlaying

    val isVoiceListening: StateFlow<Boolean> =
        speechRecognitionHelper?.isListening ?: MutableStateFlow(false).asStateFlow()

    val partialSpeechText: StateFlow<String> =
        speechRecognitionHelper?.partialTranscript ?: MutableStateFlow("").asStateFlow()

    val speechVolume: StateFlow<Float> =
        speechRecognitionHelper?.speechVolume ?: MutableStateFlow(0f).asStateFlow()

    init {
        refreshMemories()
        refreshAuditLogs()

        viewModelScope.launch {
            stateMachine.turnResults.collect { turn ->
                val assistantMsg = MessageItem(
                    id = turn.turnId,
                    sender = MessageSender.ASSISTANT,
                    text = turn.responseText,
                    routeTarget = turn.routeTarget,
                    latencyMs = turn.latencyMs,
                    isInterrupted = turn.isInterrupted
                )
                _messages.value = _messages.value + assistantMsg
                _pendingMemory.value = memoryVault.pendingMemory
                _activePersona.value = stateMachine.activePersona
                refreshMemories()
                refreshAuditLogs()
            }
        }
    }

    fun startSession(hasMicPermission: Boolean) {
        stateMachine.startSession(hasMicPermission)
    }

    fun stopSession() {
        stopVoiceInput()
        stateMachine.stopSession()
    }

    fun submitUtterance(text: String) {
        if (text.isBlank()) return

        if (stateMachine.state.value == SessionState.STOPPED) {
            stateMachine.startSession(hasMicPermission = true)
        }

        val userMsg = MessageItem(
            id = "user_${System.currentTimeMillis()}",
            sender = MessageSender.USER,
            text = text
        )
        _messages.value = _messages.value + userMsg
        stateMachine.submitUtterance(text)
    }

    fun startVoiceInput(hasMicPermission: Boolean, onRequestPermission: () -> Unit) {
        if (!hasMicPermission) {
            onRequestPermission()
            return
        }

        if (stateMachine.state.value == SessionState.STOPPED) {
            stateMachine.startSession(true)
        }

        // If assistant is currently speaking, barge-in first
        if (stateMachine.state.value == SessionState.SPEAKING || isPlaying.value) {
            bargeIn()
        }

        speechRecognitionHelper?.startListening()
    }

    fun stopVoiceInput() {
        speechRecognitionHelper?.stopListening()
    }

    fun petAvatar() {
        if (stateMachine.state.value == SessionState.STOPPED) {
            stateMachine.startSession(hasMicPermission = true)
        }
        submitUtterance("摸摸头")
    }

    fun bargeIn() {
        val latencyMs = stateMachine.interrupt(reason = "user_barge_in")
        val interruptMsg = MessageItem(
            id = "interrupt_${System.currentTimeMillis()}",
            sender = MessageSender.SYSTEM,
            text = "⚡ 全双工插话打断: 本地播报已闭音 (打断耗时: ${latencyMs}ms，音轨已重置锁定)",
            isInterrupted = true
        )
        _messages.value = _messages.value + interruptMsg
    }

    fun setCloudConsent(consented: Boolean) {
        _isCloudConsented.value = consented
        stateMachine.isCloudConsented = consented
        appSettings?.isCloudConsented = consented
    }

    fun setOnline(online: Boolean) {
        _isOnline.value = online
        stateMachine.isOnline = online
    }

    fun updateSettings(
        serverUrl: String,
        isCloudConsented: Boolean,
        autoTts: Boolean,
        speechRate: Float,
        activePersona: String,
        customLlmUrl: String,
        customLlmKey: String,
        customLlmModel: String
    ) {
        appSettings?.let {
            it.serverUrl = serverUrl
            it.isCloudConsented = isCloudConsented
            it.isAutoTtsEnabled = autoTts
            it.speechRate = speechRate
            it.activePersona = activePersona
            it.customLlmUrl = customLlmUrl
            it.customLlmKey = customLlmKey
            it.customLlmModel = customLlmModel
        }

        cloudClient.serverUrl = serverUrl
        setCloudConsent(isCloudConsented)

        stateMachine.activePersona = activePersona
        stateMachine.customLlmUrl = customLlmUrl
        stateMachine.customLlmKey = customLlmKey
        stateMachine.customLlmModel = customLlmModel
        _activePersona.value = activePersona

        playbackController.synthesizer?.setSpeechRate(speechRate)
    }

    fun confirmStagedMemory() {
        val saved = memoryVault.confirmPendingMemory()
        _pendingMemory.value = null
        if (saved != null) {
            val sysMsg = MessageItem(
                id = "mem_saved_${System.currentTimeMillis()}",
                sender = MessageSender.SYSTEM,
                text = "✅ 用户已确认: 记忆成功持久化到本设备私有存储 [${saved.content}]"
            )
            _messages.value = _messages.value + sysMsg
            refreshMemories()
        }
    }

    fun rejectStagedMemory() {
        memoryVault.rejectPendingMemory()
        _pendingMemory.value = null
        val sysMsg = MessageItem(
            id = "mem_reject_${System.currentTimeMillis()}",
            sender = MessageSender.SYSTEM,
            text = "❌ 用户已放弃该记忆保存"
        )
        _messages.value = _messages.value + sysMsg
    }

    fun deleteMemory(id: String) {
        memoryVault.delete(id)
        refreshMemories()
    }

    fun clearAllMemories() {
        memoryVault.clearAll()
        refreshMemories()
    }

    fun clearAuditLogs() {
        vehicleEngine.clearAuditLogs()
        refreshAuditLogs()
    }

    fun refreshMemories() {
        _memories.value = memoryVault.getAll()
    }

    fun refreshAuditLogs() {
        _auditLogs.value = vehicleEngine.getAuditLogs()
    }

    fun clearMessages() {
        _messages.value = emptyList()
    }
}
