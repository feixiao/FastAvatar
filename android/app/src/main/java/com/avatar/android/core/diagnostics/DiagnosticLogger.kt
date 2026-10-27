package com.avatar.android.core.diagnostics

import com.google.gson.Gson
import java.io.File
import java.io.FileOutputStream
import java.io.OutputStreamWriter
import java.nio.charset.StandardCharsets
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

class DiagnosticLogger(private val logFile: File) {

    private val gson = Gson()
    private val lock = Any()

    init {
        logFile.parentFile?.mkdirs()
    }

    private fun currentUtcTime(): String {
        return SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'", Locale.US).apply {
            timeZone = TimeZone.getTimeZone("UTC")
        }.format(Date())
    }

    fun logEvent(event: String, properties: Map<String, Any?>) {
        synchronized(lock) {
            // Guardrail: Sanitize properties to prevent accidental leaking of audio or transcript text
            val sanitized = properties.filterKeys { key ->
                !key.contains("audio", ignoreCase = true) &&
                        !key.contains("transcript", ignoreCase = true) &&
                        !key.contains("token", ignoreCase = true) &&
                        !key.contains("secret", ignoreCase = true) &&
                        !key.contains("password", ignoreCase = true)
            }

            val entry = DiagnosticEvent(
                event = event,
                timestamp = currentUtcTime(),
                properties = sanitized
            )

            val json = gson.toJson(entry)
            FileOutputStream(logFile, true).use { fos ->
                OutputStreamWriter(fos, StandardCharsets.UTF_8).use { writer ->
                    writer.write(json)
                    writer.write("\n")
                }
            }
        }
    }

    fun getEvents(): List<DiagnosticEvent> {
        synchronized(lock) {
            if (!logFile.exists()) return emptyList()
            return logFile.readLines(StandardCharsets.UTF_8)
                .filter { it.isNotBlank() }
                .mapNotNull {
                    try {
                        gson.fromJson(it, DiagnosticEvent::class.java)
                    } catch (e: Exception) {
                        null
                    }
                }
        }
    }

    fun clear() {
        synchronized(lock) {
            if (logFile.exists()) {
                logFile.writeText("")
            }
        }
    }
}
