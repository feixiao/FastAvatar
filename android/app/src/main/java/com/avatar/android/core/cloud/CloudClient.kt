package com.avatar.android.core.cloud

import com.google.gson.Gson
import com.google.gson.JsonObject
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.withContext
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.IOException
import java.util.concurrent.TimeUnit

class CloudClient(
    var serverUrl: String,
    val userId: String,
    val deviceId: String,
    val clientType: String = "pixel_10",
    private val httpClient: OkHttpClient = OkHttpClient.Builder()
        .connectTimeout(5, TimeUnit.SECONDS)
        .readTimeout(5, TimeUnit.SECONDS)
        .build()
) {
    private val gson = Gson()
    private val jsonMediaType = "application/json; charset=utf-8".toMediaType()
    private val lock = Any()

    @Volatile
    var state: CloudConnectionState = CloudConnectionState.DISCONNECTED
        private set

    @Volatile
    var currentRoom: String = ""
        private set

    @Volatile
    var currentToken: String = ""
        private set

    @Volatile
    var activeTurnId: String = ""
        private set

    // PRD Guardrail: Audio track is muted by default (Gating)
    @Volatile
    var isAudioMuted: Boolean = true
        private set

    suspend fun acquireSessionToken(cloudConsent: Boolean): Result<TokenResponse> = withContext(Dispatchers.IO) {
        if (!cloudConsent) {
            return@withContext Result.failure(IllegalStateException("Cloud processing requires explicit user consent"))
        }

        synchronized(lock) {
            state = CloudConnectionState.CONNECTING
        }

        val bodyObj = TokenRequest(
            userId = userId,
            deviceId = deviceId,
            clientType = clientType,
            cloudConsent = cloudConsent
        )
        val bodyJson = gson.toJson(bodyObj)
        val request = Request.Builder()
            .url("$serverUrl/api/v1/auth/token")
            .post(bodyJson.toRequestBody(jsonMediaType))
            .build()

        try {
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    synchronized(lock) { state = CloudConnectionState.DISCONNECTED }
                    return@withContext Result.failure(IOException("Server returned status ${response.code}"))
                }
                val respBody = response.body?.string() ?: ""
                val tokenResp = gson.fromJson(respBody, TokenResponse::class.java)

                synchronized(lock) {
                    currentRoom = tokenResp.roomName
                    currentToken = tokenResp.token
                    state = CloudConnectionState.CONNECTED
                    isAudioMuted = true // Gated by default
                }

                Result.success(tokenResp)
            }
        } catch (e: Exception) {
            synchronized(lock) { state = CloudConnectionState.DISCONNECTED }
            Result.failure(e)
        }
    }

    /**
     * 发送聊天文本到云端服务器或用户配置的自定义 LLM 端点
     */
    suspend fun sendChatMessage(
        turnId: String,
        message: String,
        persona: String,
        customLlmUrl: String = "",
        customLlmKey: String = "",
        customLlmModel: String = ""
    ): Result<String> = withContext(Dispatchers.IO) {
        // 1. 如果用户配置了自定义 OpenAI / Ollama 兼容端点
        if (customLlmUrl.isNotBlank()) {
            val endpoint = if (customLlmUrl.endsWith("/chat/completions")) {
                customLlmUrl
            } else {
                "${customLlmUrl.trimEnd('/')}/chat/completions"
            }

            val systemPrompt = when (persona) {
                "roaster" -> "你是一位幽默犀利的毒舌损友，说话风趣、善意吐槽，拒绝鸡汤和套话，简短口语化。"
                "jarvis" -> "你是一位高水准的私人幕僚顾问，回答极其凝练、专业、有条理，适合快速语音收听。"
                "pet" -> "你是一只元气可爱的猫娘/宠物小精灵，说话活泼软萌、爱撒娇，常用‘喵呜’等语气词。"
                else -> "你是一位温柔体贴的知心伴侣，温和共情、善解人意，像老朋友闲聊一样陪伴用户。"
            }

            val requestBodyJson = gson.toJson(
                mapOf(
                    "model" to (customLlmModel.ifBlank { "gpt-3.5-turbo" }),
                    "messages" to listOf(
                        mapOf("role" to "system", "content" to systemPrompt),
                        mapOf("role" to "user", "content" to message)
                    ),
                    "temperature" to 0.7,
                    "max_tokens" to 300
                )
            )

            val reqBuilder = Request.Builder()
                .url(endpoint)
                .post(requestBodyJson.toRequestBody(jsonMediaType))

            if (customLlmKey.isNotBlank()) {
                reqBuilder.addHeader("Authorization", "Bearer $customLlmKey")
            }

            return@withContext try {
                httpClient.newCall(reqBuilder.build()).execute().use { response ->
                    if (!response.isSuccessful) {
                        return@withContext Result.failure(IOException("LLM request failed with status ${response.code}"))
                    }
                    val bodyStr = response.body?.string() ?: ""
                    val root = gson.fromJson(bodyStr, JsonObject::class.java)
                    val choices = root.getAsJsonArray("choices")
                    val content = choices?.get(0)?.asJsonObject
                        ?.getAsJsonObject("message")
                        ?.get("content")?.asString ?: ""
                    if (content.isNotBlank()) {
                        Result.success(content)
                    } else {
                        Result.failure(IOException("Empty LLM reply"))
                    }
                }
            } catch (e: Exception) {
                Result.failure(e)
            }
        }

        // 2. 默认向云端网关发起 /api/v1/chat
        val bodyObj = ChatTurnRequest(
            sessionId = currentRoom.ifBlank { "sess_${deviceId}" },
            turnId = turnId,
            userId = userId,
            deviceId = deviceId,
            message = message,
            persona = persona
        )
        val bodyJson = gson.toJson(bodyObj)
        val request = Request.Builder()
            .url("$serverUrl/api/v1/chat")
            .post(bodyJson.toRequestBody(jsonMediaType))
            .build()

        try {
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    return@withContext Result.failure(IOException("Server /api/v1/chat failed with status ${response.code}"))
                }
                val respBody = response.body?.string() ?: ""
                val chatResp = gson.fromJson(respBody, ChatTurnResponse::class.java)
                Result.success(chatResp.response)
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /**
     * 流式获取聊天回复 Token (基于 SSE Server-Sent Events 流式通道)
     */
    fun streamChatMessage(
        turnId: String,
        message: String,
        persona: String,
        customLlmUrl: String = "",
        customLlmKey: String = "",
        customLlmModel: String = ""
    ): Flow<String> = flow {
        if (customLlmUrl.isNotBlank()) {
            val endpoint = if (customLlmUrl.endsWith("/chat/completions")) {
                customLlmUrl
            } else {
                "${customLlmUrl.trimEnd('/')}/chat/completions"
            }

            val systemPrompt = when (persona) {
                "roaster" -> "你是一位幽默犀利的毒舌损友，说话风趣、善意吐槽，拒绝鸡汤和套话，简短口语化。"
                "jarvis" -> "你是一位高水准的私人幕僚顾问，回答极其凝练、专业、有条理，适合快速语音收听。"
                "pet" -> "你是一只元气可爱的猫娘/宠物小精灵，说话活泼软萌、爱撒娇，常用‘喵呜’等语气词。"
                else -> "你是一位温柔体贴的知心伴侣，温和共情、善解人意，像老朋友闲聊一样陪伴用户。"
            }

            val requestBodyJson = gson.toJson(
                mapOf(
                    "model" to (customLlmModel.ifBlank { "gpt-3.5-turbo" }),
                    "stream" to true,
                    "messages" to listOf(
                        mapOf("role" to "system", "content" to systemPrompt),
                        mapOf("role" to "user", "content" to message)
                    ),
                    "temperature" to 0.7,
                    "max_tokens" to 300
                )
            )

            val reqBuilder = Request.Builder()
                .url(endpoint)
                .post(requestBodyJson.toRequestBody(jsonMediaType))

            if (customLlmKey.isNotBlank()) {
                reqBuilder.addHeader("Authorization", "Bearer $customLlmKey")
            }

            val response = httpClient.newCall(reqBuilder.build()).execute()
            if (!response.isSuccessful) {
                throw IOException("LLM streaming failed with status ${response.code}")
            }

            response.body?.source()?.let { source ->
                while (!source.exhausted()) {
                    val line = source.readUtf8Line() ?: break
                    if (line.startsWith("data: ")) {
                        val data = line.removePrefix("data: ").trim()
                        if (data == "[DONE]") break
                        try {
                            val root = gson.fromJson(data, JsonObject::class.java)
                            val choices = root.getAsJsonArray("choices")
                            val delta = choices?.get(0)?.asJsonObject?.getAsJsonObject("delta")
                            val content = delta?.get("content")?.asString
                            if (!content.isNullOrEmpty()) {
                                emit(content)
                            }
                        } catch (_: Exception) {
                        }
                    }
                }
            }
        } else {
            // 云端标准接口非流式降级为单次 emit
            val result = sendChatMessage(turnId, message, persona)
            if (result.isSuccess) {
                emit(result.getOrThrow())
            } else {
                throw result.exceptionOrNull() ?: IOException("Failed to send chat message")
            }
        }
    }.flowOn(Dispatchers.IO)

    fun startTurn(turnId: String): Boolean {
        synchronized(lock) {
            if (state != CloudConnectionState.CONNECTED) return false
            activeTurnId = turnId
            isAudioMuted = false // Open media gate for open-ended turn
            return true
        }
    }

    fun finishTurn(turnId: String) {
        synchronized(lock) {
            if (activeTurnId == turnId) {
                activeTurnId = ""
                isAudioMuted = true // Re-mute media gate
            }
        }
    }

    suspend fun interruptActiveTurn(turnId: String, reason: String = "barge_in"): Result<Unit> = withContext(Dispatchers.IO) {
        val room: String
        synchronized(lock) {
            room = currentRoom
            activeTurnId = ""
            isAudioMuted = true // Mute audio instantly (< 50ms)
        }

        if (room.isEmpty()) {
            return@withContext Result.success(Unit)
        }

        val bodyObj = InterruptRequest(
            sessionId = room,
            turnId = turnId,
            reason = reason
        )
        val bodyJson = gson.toJson(bodyObj)
        val request = Request.Builder()
            .url("$serverUrl/api/v1/session/interrupt")
            .post(bodyJson.toRequestBody(jsonMediaType))
            .build()

        try {
            httpClient.newCall(request).execute().use { response ->
                if (!response.isSuccessful) {
                    return@withContext Result.failure(IOException("Interrupt request failed with status ${response.code}"))
                }
                Result.success(Unit)
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    suspend fun closeSession(): Result<Unit> = withContext(Dispatchers.IO) {
        val room: String
        synchronized(lock) {
            room = currentRoom
            currentRoom = ""
            currentToken = ""
            activeTurnId = ""
            isAudioMuted = true
            state = CloudConnectionState.DISCONNECTED
        }

        if (room.isEmpty()) {
            return@withContext Result.success(Unit)
        }

        val bodyObj = CloseSessionRequest(sessionId = room)
        val bodyJson = gson.toJson(bodyObj)
        val request = Request.Builder()
            .url("$serverUrl/api/v1/session/close")
            .post(bodyJson.toRequestBody(jsonMediaType))
            .build()

        try {
            httpClient.newCall(request).execute().use {
                Result.success(Unit)
            }
        } catch (e: Exception) {
            Result.failure(e)
        }
    }
}
