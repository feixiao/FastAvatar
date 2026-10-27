package com.avatar.android.core.mockvehicle

import java.util.regex.Pattern

data class ParsedVehicleIntent(
    val intent: String,
    val slots: Map<String, Any?>,
    val matched: Boolean
)

object VehicleIntentParser {

    private val tempPattern = Pattern.compile("(\\d{1,2}(?:\\.\\d)?)度")

    fun parse(rawText: String): ParsedVehicleIntent {
        val text = rawText.trim()

        // 1. Climate intent
        if (text.contains("空调") || text.contains("温度") || (text.contains("调到") && text.contains("度"))) {
            val slots = mutableMapOf<String, Any?>()
            val matcher = tempPattern.matcher(text)
            if (matcher.find()) {
                val tempStr = matcher.group(1)
                tempStr?.toDoubleOrNull()?.let {
                    slots["target_temperature"] = it
                }
            }

            // Zone extraction
            val zone = when {
                text.contains("主驾") -> "driver"
                text.contains("副驾") -> "passenger"
                text.contains("后排左") || text.contains("左后") -> "rear_left"
                text.contains("后排右") || text.contains("右后") -> "rear_right"
                text.contains("全部") || text.contains("全车") -> "all"
                else -> "driver"
            }
            slots["zone"] = zone

            return ParsedVehicleIntent(
                intent = "climate.set_temperature",
                slots = slots,
                matched = true
            )
        }

        // 2. Window intent
        if (text.contains("车窗") || text.contains("窗户") || text.contains("窗子")) {
            val slots = mutableMapOf<String, Any?>()
            val action = when {
                text.contains("打开") || text.contains("降下") || text.contains("摇下") || text.contains("开") -> "open"
                text.contains("关闭") || text.contains("关上") || text.contains("升起") || text.contains("摇上") || text.contains("关") -> "close"
                text.contains("透气") || text.contains("微开") || text.contains("缝") -> "vent"
                else -> ""
            }
            if (action.isNotEmpty()) {
                slots["action"] = action
            }

            val position = when {
                text.contains("主驾") -> "driver"
                text.contains("副驾") -> "passenger"
                text.contains("左后") || text.contains("后排左") -> "rear_left"
                text.contains("右后") || text.contains("后排右") -> "rear_right"
                text.contains("所有") || text.contains("全") || text.contains("四个") -> "all"
                else -> "all"
            }
            slots["position"] = position

            return ParsedVehicleIntent(
                intent = "window.control",
                slots = slots,
                matched = true
            )
        }

        // 3. Drive mode intent
        if (text.contains("模式") || text.contains("驾驶")) {
            val slots = mutableMapOf<String, Any?>()
            val mode = when {
                text.contains("节能") || text.contains("省油") || text.contains("eco", ignoreCase = true) -> "eco"
                text.contains("运动") || text.contains("sport", ignoreCase = true) -> "sport"
                text.contains("舒适") || text.contains("标准") || text.contains("comfort", ignoreCase = true) -> "comfort"
                text.contains("漂移") || text.contains("drift", ignoreCase = true) -> "drift"
                text.contains("赛道") || text.contains("竞速") || text.contains("race", ignoreCase = true) -> "race"
                text.contains("越野") || text.contains("offroad", ignoreCase = true) -> "offroad"
                else -> ""
            }
            if (mode.isNotEmpty()) {
                slots["mode"] = mode
            }

            return ParsedVehicleIntent(
                intent = "drive.switch_mode",
                slots = slots,
                matched = true
            )
        }

        return ParsedVehicleIntent(
            intent = "",
            slots = emptyMap(),
            matched = false
        )
    }
}
