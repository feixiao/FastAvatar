package com.avatar.android.core.audio

import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.ArrayDeque

interface AudioPlaybackListener {
    fun onPlaybackStarted(turnId: String)
    fun onPlaybackStopped(turnId: String, reason: String)
    fun onPlaybackCompleted(turnId: String)
}

/**
 * 低延迟流式音频播放与打断控制器
 *
 * 核心能力：
 * 1. 动态切句播放：利用 SentenceChunker 将长文本切分为极短首句与后续连贯句，分句送入 TTS；
 * 2. 播报内容精确溯源 (Spoken Text Accumulator)：毫秒级追踪已实际播报的分句；
 * 3. 极速打断 (<80ms)：清空待播队列，停止底层合成，保留已播内容供记忆与上下文回滚；
 * 4. 支持整段输入与流式 Token 分块输入两种模式。
 */
class AudioPlaybackController(
    val synthesizer: SpeechSynthesizer? = null,
    private val chunker: SentenceChunker = SentenceChunker()
) {

    private val _isPlaying = MutableStateFlow(false)
    val isPlaying: StateFlow<Boolean> = _isPlaying.asStateFlow()

    @Volatile
    var currentTurnId: String? = null
        private set

    val hasActiveSynthesizer: Boolean
        get() = synthesizer != null && synthesizer.isAvailable

    private val listeners = mutableListOf<AudioPlaybackListener>()
    private val lock = Any()

    private val pendingChunks = ArrayDeque<String>()
    private val _spokenChunks = mutableListOf<String>()
    @Volatile
    private var currentlySpeakingChunk: String? = null
    private var chunkCount = 0
    private var activeOnNaturalDone: (() -> Unit)? = null

    private var isStreamingMode = false
    private var isStreamingFinished = false

    fun addListener(listener: AudioPlaybackListener) {
        synchronized(lock) { listeners.add(listener) }
    }

    fun removeListener(listener: AudioPlaybackListener) {
        synchronized(lock) { listeners.remove(listener) }
    }

    /**
     * 获取当前或最近一轮回合已实际播报给用户的完整文本 (用于打断记忆回滚)
     */
    val spokenText: String
        get() = synchronized(lock) { _spokenChunks.joinToString("") }

    fun getSpokenText(): String = spokenText

    fun resetSpokenText() {
        synchronized(lock) {
            _spokenChunks.clear()
            currentlySpeakingChunk = null
        }
    }

    /**
     * 整段文本分句播放 (自动进行流式标点切分，压榨首音延迟)
     */
    fun startPlayback(
        turnId: String,
        text: String,
        onNaturalDone: (() -> Unit)? = null
    ) {
        val chunks = chunker.splitAll(text)
        if (chunks.isEmpty()) {
            onNaturalDone?.invoke()
            return
        }

        synchronized(lock) {
            currentTurnId = turnId
            _isPlaying.value = true
            isStreamingMode = false
            isStreamingFinished = true
            _spokenChunks.clear()
            currentlySpeakingChunk = null
            chunkCount = 0
            pendingChunks.clear()
            pendingChunks.addAll(chunks)
            activeOnNaturalDone = onNaturalDone
            listeners.forEach { it.onPlaybackStarted(turnId) }
        }

        if (synthesizer != null && synthesizer.isAvailable) {
            playNextChunk(turnId)
        } else {
            // 无活跃合成器 (如单元测试环境或降级静音模式)
            synchronized(lock) {
                _spokenChunks.addAll(chunks)
            }
        }
    }

    /**
     * 启动流式分块播放模式 (边接收 LLM Token 边入队播放)
     */
    fun startStreamingPlayback(
        turnId: String,
        onNaturalDone: (() -> Unit)? = null
    ) {
        synchronized(lock) {
            currentTurnId = turnId
            _isPlaying.value = true
            isStreamingMode = true
            isStreamingFinished = false
            _spokenChunks.clear()
            currentlySpeakingChunk = null
            chunkCount = 0
            pendingChunks.clear()
            activeOnNaturalDone = onNaturalDone
            chunker.reset()
            listeners.forEach { it.onPlaybackStarted(turnId) }
        }
    }

    /**
     * 接收流式切句片段入队播放
     */
    fun enqueueChunk(turnId: String, chunk: String) {
        if (chunk.isBlank()) return

        var shouldTriggerPlay = false
        synchronized(lock) {
            if (currentTurnId != turnId) return
            pendingChunks.add(chunk)
            if (currentlySpeakingChunk == null) {
                shouldTriggerPlay = true
            }
        }

        if (shouldTriggerPlay && synthesizer != null && synthesizer.isAvailable) {
            playNextChunk(turnId)
        }
    }

    /**
     * 标记流式分块输入完毕
     */
    fun finishStreaming(turnId: String) {
        var shouldFinishNow = false
        val doneCb: (() -> Unit)?
        synchronized(lock) {
            if (currentTurnId != turnId) return
            isStreamingFinished = true
            if (pendingChunks.isEmpty() && currentlySpeakingChunk == null) {
                shouldFinishNow = true
            }
            doneCb = activeOnNaturalDone
            if (shouldFinishNow) {
                activeOnNaturalDone = null
            }
        }

        if (shouldFinishNow) {
            finishPlayback()
            doneCb?.invoke()
        }
    }

    private fun playNextChunk(turnId: String) {
        val nextChunk: String?
        val chunkIndex: Int
        synchronized(lock) {
            if (currentTurnId != turnId || !_isPlaying.value) return

            if (pendingChunks.isEmpty()) {
                currentlySpeakingChunk = null
                if (isStreamingMode && !isStreamingFinished) {
                    // 等待后续流式 Token 到达
                    return
                }
                finishPlayback()
                val doneCb = activeOnNaturalDone
                activeOnNaturalDone = null
                doneCb?.invoke()
                return
            }

            nextChunk = pendingChunks.removeFirst()
            chunkIndex = chunkCount++
            currentlySpeakingChunk = nextChunk
        }

        if (nextChunk == null) return

        val utteranceId = "${turnId}_c$chunkIndex"
        synthesizer?.speakChunk(
            turnId = turnId,
            utteranceId = utteranceId,
            text = nextChunk,
            isQueueAdd = false,
            onStart = {
                // 当前分句开始出声
            },
            onDone = {
                synchronized(lock) {
                    if (currentTurnId == turnId) {
                        _spokenChunks.add(nextChunk)
                        if (currentlySpeakingChunk == nextChunk) {
                            currentlySpeakingChunk = null
                        }
                    }
                }
                playNextChunk(turnId)
            },
            onError = {
                synchronized(lock) {
                    if (currentTurnId == turnId) {
                        _spokenChunks.add(nextChunk)
                        if (currentlySpeakingChunk == nextChunk) {
                            currentlySpeakingChunk = null
                        }
                    }
                }
                playNextChunk(turnId)
            }
        )
    }

    /**
     * 极速停止当前播报 (< 80ms 达标 PRD Section 2.2)
     */
    fun stopPlayback(reason: String = "stop"): Long {
        val startNano = System.nanoTime()
        synthesizer?.stop()
        val turnId: String?
        synchronized(lock) {
            turnId = currentTurnId
            currentTurnId = null
            _isPlaying.value = false
            pendingChunks.clear()
            // 如果打断时正有一句正在发音，将其纳入已播报文本
            currentlySpeakingChunk?.let { activeChunk ->
                if (!_spokenChunks.contains(activeChunk)) {
                    _spokenChunks.add(activeChunk)
                }
                currentlySpeakingChunk = null
            }
            activeOnNaturalDone = null
            turnId?.let { tid ->
                listeners.forEach { it.onPlaybackStopped(tid, reason) }
            }
        }
        return (System.nanoTime() - startNano) / 1_000_000
    }

    /**
     * 标记播报正常自然结束
     */
    fun finishPlayback() {
        val turnId: String?
        synchronized(lock) {
            turnId = currentTurnId
            currentTurnId = null
            _isPlaying.value = false
            pendingChunks.clear()
            currentlySpeakingChunk = null
            turnId?.let { tid ->
                listeners.forEach { it.onPlaybackCompleted(tid) }
            }
        }
    }
}
