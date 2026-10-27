package com.avatar.android.core.diagnostics

/**
 * Diagnostic event per PRD Section 5.3.
 * Data minimization: Non-content metadata only!
 */
data class DiagnosticEvent(
    val event: String,
    val timestamp: String,
    val properties: Map<String, Any?>
)
