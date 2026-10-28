package com.avatar.android.core.audio

import android.content.Context
import android.content.Intent
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.speech.RecognitionListener
import android.speech.RecognizerIntent
import android.speech.SpeechRecognizer
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import java.util.Locale

class SpeechRecognitionHelper(
    private val context: Context,
    private val onFinalResult: (String) -> Unit
) {

    private val mainHandler = Handler(Looper.getMainLooper())
    private var recognizer: SpeechRecognizer? = null

    private val _isListening = MutableStateFlow(false)
    val isListening: StateFlow<Boolean> = _isListening.asStateFlow()

    private val _partialTranscript = MutableStateFlow("")
    val partialTranscript: StateFlow<String> = _partialTranscript.asStateFlow()

    private val _speechVolume = MutableStateFlow(0f)
    val speechVolume: StateFlow<Float> = _speechVolume.asStateFlow()

    val isAvailable: Boolean
        get() = SpeechRecognizer.isRecognitionAvailable(context)

    fun startListening() {
        mainHandler.post {
            try {
                if (!isAvailable) {
                    return@post
                }

                if (recognizer == null) {
                    recognizer = SpeechRecognizer.createSpeechRecognizer(context)
                    setupListener()
                }

                val intent = Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).apply {
                    putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL, RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
                    putExtra(RecognizerIntent.EXTRA_LANGUAGE, Locale.CHINESE.toString())
                    putExtra(RecognizerIntent.EXTRA_LANGUAGE_PREFERENCE, "zh-CN")
                    putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS, true)
                    putExtra(RecognizerIntent.EXTRA_MAX_RESULTS, 1)
                }

                recognizer?.startListening(intent)
                _isListening.value = true
                _partialTranscript.value = ""
            } catch (e: Exception) {
                _isListening.value = false
            }
        }
    }

    fun stopListening() {
        mainHandler.post {
            try {
                recognizer?.stopListening()
            } catch (_: Exception) {}
            _isListening.value = false
        }
    }

    fun cancel() {
        mainHandler.post {
            try {
                recognizer?.cancel()
            } catch (_: Exception) {}
            _isListening.value = false
            _partialTranscript.value = ""
            _speechVolume.value = 0f
        }
    }

    fun destroy() {
        mainHandler.post {
            try {
                recognizer?.destroy()
            } catch (_: Exception) {}
            recognizer = null
            _isListening.value = false
        }
    }

    private fun setupListener() {
        recognizer?.setRecognitionListener(object : RecognitionListener {
            override fun onReadyForSpeech(params: Bundle?) {
                _isListening.value = true
            }

            override fun onBeginningOfSpeech() {
                _isListening.value = true
            }

            override fun onRmsChanged(rmsdB: Float) {
                // rmsdB ranges usually from -2 to 10 dB
                val normalized = ((rmsdB + 2f) / 12f).coerceIn(0f, 1f)
                _speechVolume.value = normalized
            }

            override fun onBufferReceived(buffer: ByteArray?) {}

            override fun onEndOfSpeech() {
                _isListening.value = false
                _speechVolume.value = 0f
            }

            override fun onError(error: Int) {
                _isListening.value = false
                _speechVolume.value = 0f
            }

            override fun onResults(results: Bundle?) {
                _isListening.value = false
                _speechVolume.value = 0f
                val matches = results?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                val text = matches?.firstOrNull()?.trim()
                if (!text.isNullOrEmpty()) {
                    _partialTranscript.value = text
                    onFinalResult(text)
                }
            }

            override fun onPartialResults(partialResults: Bundle?) {
                val matches = partialResults?.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION)
                val text = matches?.firstOrNull()?.trim()
                if (!text.isNullOrEmpty()) {
                    _partialTranscript.value = text
                }
            }

            override fun onEvent(eventType: Int, params: Bundle?) {}
        })
    }
}
