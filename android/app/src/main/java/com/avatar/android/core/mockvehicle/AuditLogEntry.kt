package com.avatar.android.core.mockvehicle

import com.google.gson.annotations.SerializedName

enum class DecisionType {
    @SerializedName("MOCK_ACCEPTED")
    MOCK_ACCEPTED,

    @SerializedName("MOCK_REJECTED")
    MOCK_REJECTED
}

/**
 * Strict JSON Lines audit record per PRD Section 4.4.
 * Must NOT contain raw voice audio or raw transcript text.
 */
data class AuditLogEntry(
    val timestamp: String,
    val intent: String,
    val slots: Map<String, Any?>,
    val decision: DecisionType,
    val reason: String,
    val simulation: Boolean = true
)

object ReasonCodes {
    const val VALID_SLOTS_AND_SAFE_RANGE = "VALID_SLOTS_AND_SAFE_RANGE"
    const val ERR_SLOT_MISSING = "ERR_SLOT_MISSING"
    const val ERR_TEMP_OUT_OF_RANGE = "ERR_TEMP_OUT_OF_RANGE"
    const val ERR_UNSUPPORTED_WINDOW_ACTION = "ERR_UNSUPPORTED_WINDOW_ACTION"
    const val ERR_DANGEROUS_ACTION_REJECTED = "ERR_DANGEROUS_ACTION_REJECTED"
    const val ERR_UNKNOWN_VEHICLE_INTENT = "ERR_UNKNOWN_VEHICLE_INTENT"
}

data class ExecutionResult(
    val entry: AuditLogEntry,
    val confirmation: String,
    val accepted: Boolean
)
