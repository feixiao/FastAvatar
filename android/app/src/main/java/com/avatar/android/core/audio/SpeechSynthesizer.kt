package com.avatar.android.core.audio

/**
 * 语音合成与播报抽象接口
 *
 * 允许在单元测试环境中无依赖模拟，同时在真实 Android 设备上调用系统原生 TextToSpeech。
 */
interface SpeechSynthesizer {
    val isAvailable: Boolean

    /**
     * 开始合成并播报语音
     * @param turnId 当前回合 ID
     * @param text 待朗读文本
     * @param onStart 播报实际开始回调
     * @param onDone 播报正常播放完毕回调
     * @param onError 播报异常或被中断回调
     */
    fun speak(
        turnId: String,
        text: String,
        onStart: (() -> Unit)? = null,
        onDone: (() -> Unit)? = null,
        onError: ((String) -> Unit)? = null
    )

    /**
     * 合成并播报分句片段 (用于低延迟流式切句)
     * @param turnId 当前回合 ID
     * @param utteranceId 当前分句片段的唯一 ID
     * @param text 待朗读文本
     * @param isQueueAdd 是否加入播放队列 (false 为立即清空队列排他播放)
     */
    fun speakChunk(
        turnId: String,
        utteranceId: String,
        text: String,
        isQueueAdd: Boolean = false,
        onStart: (() -> Unit)? = null,
        onDone: (() -> Unit)? = null,
        onError: ((String) -> Unit)? = null
    ) {
        speak(turnId, text, onStart, onDone, onError)
    }

    /**
     * 极速停止当前播报 (< 80ms)
     */
    fun stop()

    /**
     * 调整语速 (0.5f ~ 2.0f, 默认 1.0f)
     */
    fun setSpeechRate(rate: Float)

    /**
     * 调整音调 (0.5f ~ 2.0f, 默认 1.0f)
     */
    fun setPitch(pitch: Float)

    /**
     * 释放底层资源
     */
    fun release()
}
