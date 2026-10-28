package com.avatar.android.core.audio

import android.content.Context
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.speech.tts.TextToSpeech
import android.speech.tts.UtteranceProgressListener
import java.util.Locale
import java.util.concurrent.ConcurrentHashMap

class AndroidSpeechSynthesizer(
    context: Context,
    private val defaultLanguage: Locale = Locale.CHINESE
) : SpeechSynthesizer, TextToSpeech.OnInitListener {

    private val mainHandler = Handler(Looper.getMainLooper())
    private var tts: TextToSpeech? = TextToSpeech(context.applicationContext, this)

    @Volatile
    private var isInitialized = false

    @Volatile
    private var currentTurnId: String? = null

    private var speechRate: Float = 1.0f
    private var speechPitch: Float = 1.0f

    private val startCallbacks = ConcurrentHashMap<String, () -> Unit>()
    private val doneCallbacks = ConcurrentHashMap<String, () -> Unit>()
    private val errorCallbacks = ConcurrentHashMap<String, (String) -> Unit>()

    override val isAvailable: Boolean
        get() = isInitialized && tts != null

    override fun onInit(status: Int) {
        if (status == TextToSpeech.SUCCESS) {
            tts?.let { engine ->
                val result = engine.setLanguage(defaultLanguage)
                if (result == TextToSpeech.LANG_MISSING_DATA || result == TextToSpeech.LANG_NOT_SUPPORTED) {
                    // 降级使用系统默认语言
                    engine.language = Locale.getDefault()
                }
                engine.setSpeechRate(speechRate)
                engine.setPitch(speechPitch)
                engine.setOnUtteranceProgressListener(object : UtteranceProgressListener() {
                    override fun onStart(utteranceId: String?) {
                        utteranceId?.let { uid ->
                            val cb = startCallbacks.remove(uid)
                            cb?.let { mainHandler.post(it) }
                        }
                    }

                    override fun onDone(utteranceId: String?) {
                        utteranceId?.let { uid ->
                            val cb = doneCallbacks.remove(uid)
                            errorCallbacks.remove(uid)
                            cb?.let { mainHandler.post(it) }
                        }
                    }

                    @Deprecated("Deprecated in Java")
                    override fun onError(utteranceId: String?) {
                        utteranceId?.let { uid ->
                            val cb = errorCallbacks.remove(uid)
                            doneCallbacks.remove(uid)
                            cb?.let { mainHandler.post { it("TTS_ERROR_UNKNOWN") } }
                        }
                    }

                    override fun onError(utteranceId: String?, errorCode: Int) {
                        utteranceId?.let { uid ->
                            val cb = errorCallbacks.remove(uid)
                            doneCallbacks.remove(uid)
                            cb?.let { mainHandler.post { it("TTS_ERROR_$errorCode") } }
                        }
                    }
                })
                isInitialized = true
            }
        } else {
            isInitialized = false
        }
    }

    override fun speak(
        turnId: String,
        text: String,
        onStart: (() -> Unit)?,
        onDone: (() -> Unit)?,
        onError: ((String) -> Unit)?
    ) {
        currentTurnId = turnId
        if (onStart != null) startCallbacks[turnId] = onStart
        if (onDone != null) doneCallbacks[turnId] = onDone
        if (onError != null) errorCallbacks[turnId] = onError

        if (!isInitialized || tts == null) {
            // TTS 尚未就绪，快速降级触发 onDone 避免状态卡死
            mainHandler.post {
                onDone?.invoke()
            }
            return
        }

        val params = Bundle()
        tts?.speak(text, TextToSpeech.QUEUE_FLUSH, params, turnId)
    }

    override fun stop() {
        currentTurnId = null
        startCallbacks.clear()
        doneCallbacks.clear()
        errorCallbacks.clear()
        tts?.stop()
    }

    override fun setSpeechRate(rate: Float) {
        this.speechRate = rate.coerceIn(0.5f, 2.5f)
        if (isInitialized) {
            tts?.setSpeechRate(this.speechRate)
        }
    }

    override fun setPitch(pitch: Float) {
        this.speechPitch = pitch.coerceIn(0.5f, 2.0f)
        if (isInitialized) {
            tts?.setPitch(this.speechPitch)
        }
    }

    override fun release() {
        stop()
        tts?.shutdown()
        tts = null
        isInitialized = false
    }
}
