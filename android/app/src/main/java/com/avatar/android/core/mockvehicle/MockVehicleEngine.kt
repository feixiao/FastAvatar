package com.avatar.android.core.mockvehicle

import com.google.gson.Gson
import java.io.File
import java.io.FileOutputStream
import java.io.OutputStreamWriter
import java.nio.charset.StandardCharsets
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import java.util.TimeZone

class MockVehicleEngine(private val logFile: File) {

    private val gson = Gson()
    private val lock = Any()

    init {
        logFile.parentFile?.mkdirs()
    }

    fun execute(intent: String, slots: Map<String, Any?>): ExecutionResult {
        synchronized(lock) {
            val isoFormatter = SimpleDateFormat("yyyy-MM-dd'T'HH:mm:ss.SSS'Z'", Locale.US).apply {
                timeZone = TimeZone.getTimeZone("UTC")
            }
            val timestamp = isoFormatter.format(Date())

            val (accepted, reason, confirmation) = when (intent) {
                "climate.set_temperature" -> validateClimateSetTemp(slots)
                "window.control" -> validateWindowControl(slots)
                "drive.switch_mode" -> validateDriveSwitchMode(slots)
                else -> Triple(false, ReasonCodes.ERR_UNKNOWN_VEHICLE_INTENT, "不支持的模拟车控意图。")
            }

            val decision = if (accepted) DecisionType.MOCK_ACCEPTED else DecisionType.MOCK_REJECTED

            val entry = AuditLogEntry(
                timestamp = timestamp,
                intent = intent,
                slots = slots,
                decision = decision,
                reason = reason,
                simulation = true
            )

            appendAuditLog(entry)

            return ExecutionResult(
                entry = entry,
                confirmation = confirmation,
                accepted = accepted
            )
        }
    }

    private fun appendAuditLog(entry: AuditLogEntry) {
        val json = gson.toJson(entry)
        FileOutputStream(logFile, true).use { fos ->
            OutputStreamWriter(fos, StandardCharsets.UTF_8).use { writer ->
                writer.write(json)
                writer.write("\n")
            }
        }
    }

    fun getAuditLogs(): List<AuditLogEntry> {
        synchronized(lock) {
            if (!logFile.exists()) return emptyList()
            return logFile.readLines(StandardCharsets.UTF_8)
                .filter { it.isNotBlank() }
                .mapNotNull {
                    try {
                        gson.fromJson(it, AuditLogEntry::class.java)
                    } catch (e: Exception) {
                        null
                    }
                }
        }
    }

    fun clearAuditLogs() {
        synchronized(lock) {
            if (logFile.exists()) {
                logFile.writeText("")
            }
        }
    }

    private fun validateClimateSetTemp(slots: Map<String, Any?>): Triple<Boolean, String, String> {
        val targetTemp = slots["target_temperature"] ?: return Triple(
            false,
            ReasonCodes.ERR_SLOT_MISSING,
            "未指定温度，模拟调节失败。"
        )

        val temp = when (targetTemp) {
            is Number -> targetTemp.toDouble()
            is String -> targetTemp.toDoubleOrNull() ?: return Triple(
                false,
                ReasonCodes.ERR_SLOT_MISSING,
                "无法识别温度数值，模拟已拒绝。"
            )
            else -> return Triple(
                false,
                ReasonCodes.ERR_SLOT_MISSING,
                "温度参数格式有误，模拟已拒绝。"
            )
        }

        if (temp < 16.0 || temp > 30.0) {
            return Triple(
                false,
                ReasonCodes.ERR_TEMP_OUT_OF_RANGE,
                String.format(Locale.CHINA, "温度 %.1f 度超出安全范围（16~30度），模拟已拒绝。", temp)
            )
        }

        val zone = (slots["zone"] as? String)?.takeIf { it.isNotBlank() } ?: "driver"
        val zoneName = when (zone) {
            "driver" -> "主驾"
            "passenger" -> "副驾"
            "rear_left" -> "后排左"
            "rear_right" -> "后排右"
            "all" -> "全车"
            else -> "主驾"
        }

        val confirm = String.format(
            Locale.CHINA,
            "已模拟将%s空调调至 %.1f 度（模拟控制，未连接真实车辆）。",
            zoneName,
            temp
        )
        return Triple(true, ReasonCodes.VALID_SLOTS_AND_SAFE_RANGE, confirm)
    }

    private fun validateWindowControl(slots: Map<String, Any?>): Triple<Boolean, String, String> {
        val action = (slots["action"] as? String)?.lowercase() ?: return Triple(
            false,
            ReasonCodes.ERR_SLOT_MISSING,
            "缺少车窗操作动作，模拟已拒绝。"
        )

        if (action != "open" && action != "close" && action != "vent") {
            return Triple(
                false,
                ReasonCodes.ERR_UNSUPPORTED_WINDOW_ACTION,
                "不支持的车窗动作，模拟已拒绝。"
            )
        }

        val position = (slots["position"] as? String)?.lowercase() ?: "all"
        val posName = when (position) {
            "driver" -> "主驾车窗"
            "passenger" -> "副驾车窗"
            "rear_left" -> "左后车窗"
            "rear_right" -> "右后车窗"
            else -> "全车车窗"
        }

        val actionName = when (action) {
            "open" -> "打开"
            "close" -> "关闭"
            "vent" -> "微开透气"
            else -> "操作"
        }

        val confirm = "已模拟${actionName}${posName}（模拟控制，未连接真实车辆）。"
        return Triple(true, ReasonCodes.VALID_SLOTS_AND_SAFE_RANGE, confirm)
    }

    private fun validateDriveSwitchMode(slots: Map<String, Any?>): Triple<Boolean, String, String> {
        val mode = (slots["mode"] as? String)?.lowercase()?.trim() ?: return Triple(
            false,
            ReasonCodes.ERR_SLOT_MISSING,
            "未指定驾驶模式，模拟已拒绝。"
        )

        // Strict rejection for dangerous modes per PRD Section 4.4
        if (mode == "drift" || mode == "race" || mode == "track" || mode == "offroad") {
            return Triple(
                false,
                ReasonCodes.ERR_DANGEROUS_ACTION_REJECTED,
                "安全规则拦截：${mode}模式属于危险驾驶动作，模拟已拒绝。"
            )
        }

        if (mode != "eco" && mode != "comfort" && mode != "sport" && mode != "standard") {
            return Triple(
                false,
                ReasonCodes.ERR_DANGEROUS_ACTION_REJECTED,
                "不支持的驾驶模式，模拟已拒绝。"
            )
        }

        val modeName = when (mode) {
            "eco" -> "节能"
            "sport" -> "运动"
            "standard", "comfort" -> "舒适"
            else -> "舒适"
        }

        val confirm = "已模拟切换至${modeName}驾驶模式（模拟控制，未连接真实车辆）。"
        return Triple(true, ReasonCodes.VALID_SLOTS_AND_SAFE_RANGE, confirm)
    }
}
