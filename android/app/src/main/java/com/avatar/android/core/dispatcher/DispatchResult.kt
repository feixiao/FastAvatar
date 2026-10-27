package com.avatar.android.core.dispatcher

data class DispatchResult(
    val target: RouteTarget,
    val intentName: String,
    val slots: Map<String, Any?> = emptyMap(),
    val confidence: Double = 1.0,
    val latencyMs: Long = 0L
)
