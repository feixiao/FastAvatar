package com.avatar.android.core.cloud

import com.google.gson.annotations.SerializedName

enum class CloudConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED
}

data class TokenRequest(
    @SerializedName("user_id") val userId: String,
    @SerializedName("device_id") val deviceId: String,
    @SerializedName("client_type") val clientType: String,
    @SerializedName("cloud_consent") val cloudConsent: Boolean
)

data class TokenResponse(
    @SerializedName("livekit_url") val liveKitUrl: String,
    @SerializedName("room_name") val roomName: String,
    @SerializedName("token") val token: String,
    @SerializedName("expires_in_seconds") val expiresInSeconds: Long
)

data class InterruptRequest(
    @SerializedName("session_id") val sessionId: String,
    @SerializedName("turn_id") val turnId: String,
    @SerializedName("reason") val reason: String
)

data class CloseSessionRequest(
    @SerializedName("session_id") val sessionId: String
)
