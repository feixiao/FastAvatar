package com.avatar.android.ui

import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.avatar.android.core.audio.AudioPlaybackController
import com.avatar.android.core.audio.AudioPrivacyGuard
import com.avatar.android.core.cloud.CloudClient
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
    val deviceId: String
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

    val isAudioMuted: Boolean
        get() = cloudClient.isAudioMuted

    val isPlaying: StateFlow<Boolean> = playbackController.isPlaying

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
                refreshMemories()
                refreshAuditLogs()
            }
        }
    }

    fun startSession(hasMicPermission: Boolean) {
        stateMachine.startSession(hasMicPermission)
    }

    fun stopSession() {
        stateMachine.stopSession()
    }

    fun submitUtterance(text: String) {
        val userMsg = MessageItem(
            id = "user_${System.currentTimeMillis()}",
            sender = MessageSender.USER,
            text = text
        )
        _messages.value = _messages.value + userMsg
        stateMachine.submitUtterance(text)
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
    }

    fun setOnline(online: Boolean) {
        _isOnline.value = online
        stateMachine.isOnline = online
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
