package com.avatar.android.core.config

import android.content.Context
import android.content.SharedPreferences

class AppSettings(context: Context) {

    private val prefs: SharedPreferences =
        context.getSharedPreferences("avatar_app_settings", Context.MODE_PRIVATE)

    var serverUrl: String
        get() = prefs.getString(KEY_SERVER_URL, DEFAULT_SERVER_URL) ?: DEFAULT_SERVER_URL
        set(value) = prefs.edit().putString(KEY_SERVER_URL, value.trim()).apply()

    var isCloudConsented: Boolean
        get() = prefs.getBoolean(KEY_CLOUD_CONSENT, true) // 默认开启，让用户开箱即能聊天互动
        set(value) = prefs.edit().putBoolean(KEY_CLOUD_CONSENT, value).apply()

    var isAutoTtsEnabled: Boolean
        get() = prefs.getBoolean(KEY_AUTO_TTS, true)
        set(value) = prefs.edit().putBoolean(KEY_AUTO_TTS, value).apply()

    var speechRate: Float
        get() = prefs.getFloat(KEY_SPEECH_RATE, 1.0f)
        set(value) = prefs.edit().putFloat(KEY_SPEECH_RATE, value).apply()

    var activePersona: String
        get() = prefs.getString(KEY_ACTIVE_PERSONA, "companion") ?: "companion"
        set(value) = prefs.edit().putString(KEY_ACTIVE_PERSONA, value).apply()

    var customLlmUrl: String
        get() = prefs.getString(KEY_CUSTOM_LLM_URL, "") ?: ""
        set(value) = prefs.edit().putString(KEY_CUSTOM_LLM_URL, value.trim()).apply()

    var customLlmModel: String
        get() = prefs.getString(KEY_CUSTOM_LLM_MODEL, "gpt-3.5-turbo") ?: "gpt-3.5-turbo"
        set(value) = prefs.edit().putString(KEY_CUSTOM_LLM_MODEL, value.trim()).apply()

    var customLlmKey: String
        get() = prefs.getString(KEY_CUSTOM_LLM_KEY, "") ?: ""
        set(value) = prefs.edit().putString(KEY_CUSTOM_LLM_KEY, value.trim()).apply()

    companion object {
        const val DEFAULT_SERVER_URL = "http://10.0.2.2:8088"
        private const val KEY_SERVER_URL = "server_url"
        private const val KEY_CLOUD_CONSENT = "cloud_consent"
        private const val KEY_AUTO_TTS = "auto_tts"
        private const val KEY_SPEECH_RATE = "speech_rate"
        private const val KEY_ACTIVE_PERSONA = "active_persona"
        private const val KEY_CUSTOM_LLM_URL = "custom_llm_url"
        private const val KEY_CUSTOM_LLM_MODEL = "custom_llm_model"
        private const val KEY_CUSTOM_LLM_KEY = "custom_llm_key"
    }
}
