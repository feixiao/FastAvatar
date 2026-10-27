package com.avatar.android.core.cloud

import com.google.gson.Gson
import kotlinx.coroutines.Dispatchers
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
