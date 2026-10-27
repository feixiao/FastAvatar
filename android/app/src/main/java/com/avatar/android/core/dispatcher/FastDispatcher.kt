package com.avatar.android.core.dispatcher

import com.avatar.android.core.mockvehicle.VehicleIntentParser

class FastDispatcher {

    fun evaluate(text: String, isCloudConsented: Boolean, isOnline: Boolean): DispatchResult {
        val startNano = System.nanoTime()
        val clean = text.trim()

        // P1: Check local memory oral command
        if (isMemoryCommand(clean)) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.LOCAL_MEMORY,
                intentName = "memory.command",
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        // P2: Check mock vehicle intent (空调 / 车窗 / 驾驶模式等设备控制)
        val vehicleParse = VehicleIntentParser.parse(clean)
        if (vehicleParse.matched) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.MOCK_VEHICLE,
                intentName = vehicleParse.intent,
                slots = vehicleParse.slots,
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        // P3: Check local persona switch command
        if (isPersonaSwitchCommand(clean)) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.LOCAL_PERSONA,
                intentName = "persona.switch",
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        // P4: Open-ended conversational query (Cloud candidate)
        val target = when {
            !isCloudConsented -> RouteTarget.LOCAL_ONLY_PROMPT
            !isOnline -> RouteTarget.OFFLINE_PROMPT
            else -> RouteTarget.CLOUD_CHAT
        }

        val latencyMs = (System.nanoTime() - startNano) / 1_000_000
        return DispatchResult(
            target = target,
            intentName = "dialog.open_ended",
            confidence = 1.0,
            latencyMs = latencyMs
        )
    }

    private fun isPersonaSwitchCommand(text: String): Boolean {
        if (text.contains("驾驶") || text.contains("漂移") || text.contains("赛道") || text.contains("省油")) {
            return false
        }
        return text.startsWith("切换成") ||
                text.startsWith("切换为") ||
                text.startsWith("变成") ||
                text.startsWith("换成") ||
                text.startsWith("切换到")
    }

    private fun isMemoryCommand(text: String): Boolean {
        return text.startsWith("记住") ||
                text.startsWith("帮我记一下") ||
                text.startsWith("记一下") ||
                text.startsWith("忘掉") ||
                text.startsWith("忘了") ||
                text.startsWith("清除记忆")
    }
}
