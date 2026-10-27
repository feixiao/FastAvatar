package com.avatar.android.core.audio

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

interface AudioPlaybackListener {
    fun onPlaybackStarted(turnId: String)
    fun onPlaybackStopped(turnId: String, reason: String)
    fun onPlaybackCompleted(turnId: String)
}

class AudioPlaybackController {

    private val _isPlaying = MutableStateFlow(false)
    val isPlaying: StateFlow<Boolean> = _isPlaying.asStateFlow()

    @Volatile
    var currentTurnId: String? = null
        private set

    private val listeners = mutableListOf<AudioPlaybackListener>()
    private val lock = Any()

    fun addListener(listener: AudioPlaybackListener) {
        synchronized(lock) { listeners.add(listener) }
    }

    fun removeListener(listener: AudioPlaybackListener) {
        synchronized(lock) { listeners.remove(listener) }
    }

    /**
     * Start playing audio response.
     */
    fun startPlayback(turnId: String, text: String) {
        synchronized(lock) {
            currentTurnId = turnId
            _isPlaying.value = true
            listeners.forEach { it.onPlaybackStarted(turnId) }
        }
    }

    /**
     * Stop playback immediately (< 80ms latency target per PRD Section 2.2).
     */
    fun stopPlayback(reason: String = "stop"): Long {
        val startNano = System.nanoTime()
        val turnId: String?
        synchronized(lock) {
            turnId = currentTurnId
            currentTurnId = null
            _isPlaying.value = false
            turnId?.let { tid ->
                listeners.forEach { it.onPlaybackStopped(tid, reason) }
            }
        }
        return (System.nanoTime() - startNano) / 1_000_000
    }

    /**
     * Mark playback as naturally completed.
     */
    fun finishPlayback() {
        val turnId: String?
        synchronized(lock) {
            turnId = currentTurnId
            currentTurnId = null
            _isPlaying.value = false
            turnId?.let { tid ->
                listeners.forEach { it.onPlaybackCompleted(tid) }
            }
        }
    }
}
