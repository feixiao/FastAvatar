package com.avatar.android.core.dispatcher

import com.ai.audioagent.nativebridge.AudioAgentNative
import kotlin.math.exp

class FastDispatcher {

    @Volatile
    private var lastRoute: String = ""

    @Volatile
    private var lastDomain: String = ""

    fun updateContext(route: String, domain: String) {
        lastRoute = route
        lastDomain = domain
    }

    fun evaluate(
        text: String,
        isCloudConsented: Boolean,
        isOnline: Boolean,
        overrideLastRoute: String = lastRoute,
        overrideLastDomain: String = lastDomain
    ): DispatchResult {
        val startNano = System.nanoTime()
        val clean = text.trim()

        // 1. 优先通过 JNI 调用 C++ 核心库 (IntentArbiter + TinyIntentClassifier) 单一决策源
        val nativeJson = AudioAgentNative.dispatchIntentContext(clean, overrideLastRoute, overrideLastDomain)
        if (!nativeJson.isNullOrBlank()) {
            val parsed = parseNativeDispatchJson(nativeJson, isCloudConsented, isOnline, startNano)
            if (parsed != null) return parsed
        }

        // 2. 纯 JVM 单测环境回退镜像 (当宿主机 JVM 未挂载 libaudio_agent.so 时与 C++ 保持严格同构)
        if (isMemoryCommand(clean)) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.LOCAL_MEMORY,
                intentName = "memory.command",
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        val actionResult = matchActionCommand(clean)
        if (actionResult != null) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.LOCAL_PET_ACTION,
                intentName = "pet.action",
                slots = mapOf(
                    "action" to actionResult.action,
                    "cue" to actionResult.cue,
                    "move" to actionResult.move,
                    "response" to actionResult.response
                ),
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        if (isPersonaSwitchCommand(clean)) {
            val latencyMs = (System.nanoTime() - startNano) / 1_000_000
            return DispatchResult(
                target = RouteTarget.LOCAL_PERSONA,
                intentName = "persona.switch",
                confidence = 1.0,
                latencyMs = latencyMs
            )
        }

        val cls = classifyComplexityMirror(clean, overrideLastRoute, overrideLastDomain)
        val target = when {
            !isCloudConsented -> RouteTarget.LOCAL_ONLY_PROMPT
            !isOnline -> RouteTarget.OFFLINE_PROMPT
            else -> RouteTarget.CLOUD_CHAT
        }

        val latencyMs = (System.nanoTime() - startNano) / 1_000_000
        return DispatchResult(
            target = target,
            intentName = "dialog.open_ended",
            slots = mapOf(
                "need_cloud" to cls.needDeepReasoning,
                "cloud_probability" to cls.cloudProbability,
                "primary_domain" to cls.domainName,
                "reason" to cls.reason
            ),
            confidence = 1.0,
            latencyMs = latencyMs
        )
    }

    private fun parseNativeDispatchJson(
        json: String,
        isCloudConsented: Boolean,
        isOnline: Boolean,
        startNano: Long
    ): DispatchResult? {
        val domain = extractJsonString(json, "domain") ?: return null
        val latencyMs = (System.nanoTime() - startNano) / 1_000_000

        return when (domain) {
            "MEMORY_COMMAND" -> DispatchResult(
                target = RouteTarget.LOCAL_MEMORY,
                intentName = "memory.command",
                confidence = 1.0,
                latencyMs = latencyMs
            )
            "PERSONA_SWITCH" -> DispatchResult(
                target = RouteTarget.LOCAL_PERSONA,
                intentName = "persona.switch",
                slots = mapOf("persona" to (extractJsonString(json, "persona") ?: "companion")),
                confidence = 1.0,
                latencyMs = latencyMs
            )
            "ACTION_COMMAND" -> {
                val coreAction = extractJsonString(json, "action") ?: "jump"
                val direction = extractJsonString(json, "direction") ?: ""
                val mapped = mapCoreActionToPresentation(coreAction, direction)
                DispatchResult(
                    target = RouteTarget.LOCAL_PET_ACTION,
                    intentName = "pet.action",
                    slots = mapOf(
                        "action" to mapped.action,
                        "cue" to mapped.cue,
                        "move" to mapped.move,
                        "response" to mapped.response
                    ),
                    confidence = 1.0,
                    latencyMs = latencyMs
                )
            }
            "OPEN_ENDED" -> {
                val needCloud = json.contains("\"need_cloud\":true")
                val primaryDomain = extractJsonString(json, "primary_domain") ?: "chat"
                val reason = extractJsonString(json, "reason") ?: ""
                val target = when {
                    !isCloudConsented -> RouteTarget.LOCAL_ONLY_PROMPT
                    !isOnline -> RouteTarget.OFFLINE_PROMPT
                    else -> RouteTarget.CLOUD_CHAT
                }
                DispatchResult(
                    target = target,
                    intentName = "dialog.open_ended",
                    slots = mapOf(
                        "need_cloud" to needCloud,
                        "primary_domain" to primaryDomain,
                        "reason" to reason
                    ),
                    confidence = 1.0,
                    latencyMs = latencyMs
                )
            }
            else -> null
        }
    }

    private fun extractJsonString(json: String, key: String): String? {
        val pattern = "\"$key\":\""
        val startIdx = json.indexOf(pattern)
        if (startIdx < 0) return null
        val valStart = startIdx + pattern.length
        val endIdx = json.indexOf('"', valStart)
        if (endIdx < 0) return null
        return json.substring(valStart, endIdx)
    }

    private fun mapCoreActionToPresentation(coreAction: String, direction: String): PetActionResult {
        return when (coreAction) {
            "sit" -> PetActionResult("sit", "waiting", "", "好嘞，乖乖蹲坐好啦~ 喵呜！")
            "stand" -> PetActionResult("wake", "jumping", "", "伸个大大的懒腰，我精神满满起来啦！喵~")
            "jump" -> PetActionResult("jump", "jumping", "", "好嘞！翻个跟头跳给你看！喵呜~")
            "rest" -> PetActionResult("sleep", "idle", "", "哈欠~ 困啦困啦，趴下打个盹，晚安喵~")
            "move_left" -> PetActionResult("roam_left", "running-left", "left", "好嘞！向左边巡视小跑，守护你的屏幕！")
            "move_right" -> PetActionResult("roam_right", "running-right", "right", "收到！向右边巡视小跑，守护你的屏幕！")
            "move_forward" -> PetActionResult("roam", "running", "", "巡逻任务启动！周围安全就交给我啦！")
            "stop" -> PetActionResult("failed", "failed", "", "收到，原地定住不动啦！")
            "greet" -> PetActionResult("wave", "waving", "", "嗨！向你挥挥小爪子，今天也要开心哦！")
            "celebrate" -> PetActionResult("pet", "jumping", "", "呼噜呼噜~ 贴贴你好开心呀~")
            "confused" -> PetActionResult("failed", "failed", "", "哎呀呀，晕倒啦……装死中，勿扰喵~")
            "ponder" -> PetActionResult("review", "review", "", "嗯……让我歪头仔细观察审视一下！")
            "vocalize" -> PetActionResult("meow", "waving", "", "喵呜~~ 喵~~")
            "consume" -> PetActionResult("feed", "running", "", "哇，好吃的！嗷呜一口吃掉！真香~")
            else -> PetActionResult(coreAction, "jumping", direction, "好嘞，收到动作指令啦！")
        }
    }

    private fun isPersonaSwitchCommand(text: String): Boolean {
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

    private fun matchActionCommand(text: String): PetActionResult? {
        val clean = text.trim()
        if (clean.startsWith("为什么") || clean.startsWith("怎么") || clean.startsWith("如何") || clean.startsWith("什么是")) {
            return null
        }
        val rules = listOf(
            PetActionResult("sit", "waiting", "", "好嘞，乖乖蹲坐好啦~ 喵呜！") to listOf("蹲下", "蹲着", "蹲一个", "蹲", "坐下", "坐着", "坐好", "坐", "趴下", "趴着", "趴好", "趴", "躺下", "躺平", "躺着"),
            PetActionResult("wake", "jumping", "", "伸个大大的懒腰，我精神满满起来啦！喵~") to listOf("站起来", "起来", "站好", "醒醒", "起床"),
            PetActionResult("jump", "jumping", "", "好嘞！翻个跟头跳给你看！喵呜~") to listOf("跳一下", "跳一个", "跳跳", "翻个跟头", "翻跟头", "蹦一下", "蹦一个", "空翻", "跳起来", "转圈", "跳舞", "跳", "蹦"),
            PetActionResult("roam_right", "running-right", "right", "收到！向右边巡视小跑，守护你的屏幕！") to listOf("向右跑", "往右跑", "去右边", "向右走", "往右走", "右边巡视"),
            PetActionResult("roam_left", "running-left", "left", "好嘞！向左边巡视小跑，守护你的屏幕！") to listOf("向左跑", "往左跑", "去左边", "向左走", "往左走", "左边巡视"),
            PetActionResult("roam", "running", "", "巡逻任务启动！周围安全就交给我啦！") to listOf("巡视桌面", "去巡逻", "巡视", "巡逻", "到处逛逛", "溜达溜达", "走两步", "跑两步", "跑一跑", "跑一下", "跑起来", "往前跑", "小跑", "跑", "转悠转悠"),
            PetActionResult("failed", "failed", "", "收到，原地定住不动啦！") to listOf("别动", "停下", "停", "装死", "倒下", "晕一个", "犯错", "晕倒"),
            PetActionResult("wave", "waving", "", "嗨！向你挥挥小爪子，今天也要开心哦！") to listOf("打个招呼", "挥手", "打招呼", "招招手", "握手", "握个手", "伸手"),
            PetActionResult("pet", "jumping", "", "呼噜呼噜~ 摸摸头好舒服呀~") to listOf("摸摸头", "摸摸你", "摸摸", "蹭蹭", "抱抱"),
            PetActionResult("feed", "running", "", "哇，好吃的！嗷呜一口吃掉！真香~") to listOf("吃东西", "吃点东西", "给你吃", "投喂", "喂食", "吃零食", "干饭", "开饭", "吃饭"),
            PetActionResult("review", "review", "", "嗯……让我歪头仔细观察审视一下！") to listOf("歪头", "想一想", "思考一下", "琢磨琢磨"),
            PetActionResult("sleep", "idle", "", "哈欠~ 困啦困啦，趴下打个盹，晚安喵~") to listOf("去睡觉", "睡觉吧", "休息吧", "打个盹", "闭嘴睡觉", "安静睡觉")
        )
        for ((res, kws) in rules) {
            for (kw in kws) {
                val match = if (kw.length > 1) clean.contains(kw) else clean == kw
                if (match) return res
            }
        }
        return null
    }

    private data class MirrorClassification(
        val needDeepReasoning: Boolean,
        val cloudProbability: Double,
        val domainName: String,
        val reason: String
    )

    private fun classifyComplexityMirror(query: String, lastRoute: String, lastDomain: String): MirrorClassification {
        val q = query.lowercase()
        var logit = -1.2

        val mathFeatures = listOf("求导", "导数", "微分", "微积分", "积分", "贝叶斯", "方程", "矩阵", "特征值", "时间复杂度", "x^", "lnx")
        val codeFeatures = listOf("代码", "写一段", "写个", "写一个", "编写", "实现", "函数", "rust", "python", "golang", "c++", "java", "无锁", "环形队列", "并发", "死锁", "debug")
        val archFeatures = listOf("架构设计", "系统架构", "设计模式", "高可用", "分布式", "微服务", "论文", "深入分析", "技术方案")
        val inquiryFeatures = listOf("为什么", "什么意思", "怎么回事", "原理", "机制", "区别", "分析", "总结", "翻译", "规划", "建议", "评价", "如何")
        val followupFeatures = listOf("第", "行", "为什么", "怎么回事", "改一下", "换个写法", "再优化", "继续", "解释一下")
        val casualFeatures = listOf("散步", "公园", "流浪猫", "小猫", "小狗", "花坛", "吃饭", "睡觉", "晚安", "早安", "你好", "哈哈", "无聊", "难过", "开心", "好累", "抱抱", "聊天")

        val mathHits = mathFeatures.count { q.contains(it) }
        val codeHits = codeFeatures.count { q.contains(it) }
        val archHits = archFeatures.count { q.contains(it) }
        val inquiryHits = inquiryFeatures.count { q.contains(it) }
        val casualHits = casualFeatures.count { q.contains(it) }

        logit += mathHits * 3.5 + codeHits * 3.2 + archHits * 2.8 + inquiryHits * 1.8 - casualHits * 2.2
        val isFollowup = (lastRoute == "CLOUD_CHAT" || lastDomain == "code" || lastDomain == "math" || lastDomain == "architecture") &&
            followupFeatures.any { q.contains(it) }
        if (isFollowup) logit += 4.5
        if (q.contains("为什么") && q.toByteArray().size <= 18 && mathHits == 0 && codeHits == 0 && archHits == 0) {
            logit -= 2.2
        }

        val prob = 1.0 / (1.0 + exp(-logit))
        val needCloud = prob >= 0.5
        val domainName = when {
            isFollowup -> "context_followup"
            mathHits > 0 -> "math"
            codeHits > 0 -> "code"
            archHits > 0 -> "architecture"
            needCloud -> "professional"
            else -> "chat"
        }
        return MirrorClassification(needCloud, prob, domainName, "TinyClassifier")
    }
}

data class PetActionResult(
    val action: String,
    val cue: String,
    val move: String = "",
    val response: String
)
