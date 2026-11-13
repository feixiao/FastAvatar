package com.avatar.android.core.dispatcher


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
        // P2: Check local pet action oral command
        val actionResult = matchPetActionCommand(clean)
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

    private fun matchPetActionCommand(text: String): PetActionResult? {
        val clean = text.trim()
        if (clean.startsWith("为什么") || clean.startsWith("怎么") || clean.startsWith("如何") || clean.startsWith("什么是")) {
            return null
        }

        val prefixes = listOf(
            "小黑，", "小黑,", "小黑 ", "小黑",
            "罗小黑，", "罗小黑,", "罗小黑 ", "罗小黑",
            "桌宠，", "桌宠,", "桌宠 ", "桌宠",
            "小猫，", "小猫,", "小猫 ", "小猫",
            "宠物，", "宠物,", "宠物 ", "宠物",
            "大肥鱼，", "大肥鱼,", "大肥鱼 ", "大肥鱼"
        )
        var cmd = clean
        for (p in prefixes) {
            if (cmd.startsWith(p)) {
                cmd = cmd.removePrefix(p).trim()
                break
            }
        }
        if (cmd.isEmpty()) return null

        // 1. 移动与漫游巡视
        if (cmd.contains("向右跑") || cmd.contains("往右跑") || cmd.contains("去右边") || cmd.contains("向右走") || cmd.contains("往右走") || cmd.contains("右边巡视")) {
            return PetActionResult("roam_right", "running-right", "right", "收到！向右边巡视小跑，守护你的屏幕！")
        }
        if (cmd.contains("向左跑") || cmd.contains("往左跑") || cmd.contains("去左边") || cmd.contains("向左走") || cmd.contains("往左走") || cmd.contains("左边巡视")) {
            return PetActionResult("roam_left", "running-left", "left", "好嘞！向左边巡视小跑，守护你的屏幕！")
        }
        if (cmd.contains("巡视") || cmd.contains("巡逻") || cmd.contains("到处逛逛") || cmd.contains("溜达溜达") || cmd.contains("走两步") || cmd.contains("转悠转悠")) {
            return PetActionResult("roam", "running", "", "巡逻任务启动！周围安全就交给我啦！")
        }

        // 2. 干饭与投喂
        if (cmd.contains("疯狂干饭") || cmd.contains("暴风吸入") || cmd.contains("狂吃") || cmd.contains("大口吃")) {
            return PetActionResult("turbo", "running", "", "疯狂干饭！两只爪子疯狂刨饭，真好吃！")
        }
        if (cmd.contains("吃米饭") || cmd.contains("吃大米饭") || cmd.contains("干饭") || cmd.contains("开饭") || cmd.contains("大米饭")) {
            return PetActionResult("rice", "running", "", "热气腾腾的大米饭！大口开饭啦~")
        }
        if (cmd.contains("小鱼干") || cmd.contains("吃鱼") || cmd.contains("投喂") || cmd.contains("喂食") || cmd.contains("喂你") || cmd.contains("吃点东西") || cmd.contains("吃零食") || cmd.contains("给你鱼")) {
            return PetActionResult("feed", "running", "", "哇，小鱼干！嗷呜一口吃掉！真香~")
        }

        // 3. 玩草
        if (cmd.contains("逗猫草") || cmd.contains("逗草") || cmd.contains("逗猫棒") || cmd.contains("玩草")) {
            return PetActionResult("grass", "jumping", "", "逗猫草！看我敏捷飞扑！抓到啦！")
        }

        // 4. 跳跃与翻转
        if (cmd.contains("跳一下") || cmd.contains("跳一个") || cmd.contains("跳跳") || cmd.contains("翻个跟头") || cmd.contains("翻跟头") || cmd.contains("蹦一下") || cmd.contains("蹦一个") || cmd.contains("空翻") || cmd.contains("跳起来") || cmd.contains("转圈") || cmd.contains("跳舞") || cmd == "跳" || cmd == "蹦") {
            return PetActionResult("jump", "jumping", "", "好嘞！翻个跟头跳给你看！喵呜~")
        }

        // 5. 蹲下、坐下、趴下、躺下
        if (cmd.contains("蹲下") || cmd.contains("蹲着") || cmd.contains("蹲一个") || cmd == "蹲" ||
            cmd.contains("坐下") || cmd.contains("坐着") || cmd.contains("坐好") || cmd == "坐" ||
            cmd.contains("趴下") || cmd.contains("趴着") || cmd.contains("趴好") || cmd == "趴" ||
            cmd.contains("躺下") || cmd.contains("躺平") || cmd.contains("躺着")) {
            return PetActionResult("sit", "idle", "", "好嘞，乖乖蹲坐好啦~ 喵呜！")
        }

        // 6. 起立、醒醒、起床
        if (cmd.contains("站起来") || cmd.contains("起来") || cmd.contains("醒醒") || cmd.contains("起床")) {
            return PetActionResult("wake", "jumping", "", "伸个大大的懒腰，我精神满满起来啦！喵~")
        }

        // 7. 叫一声 / 喵一声
        if (cmd.contains("叫一声") || cmd.contains("喵一声") || cmd.contains("叫一个") || cmd.contains("学猫叫") || cmd.contains("喵喵叫") || cmd == "叫" || cmd == "喵") {
            return PetActionResult("meow", "waving", "", "喵呜~~ 喵~~")
        }

        // 8. 抚摸与蹭蹭
        if (cmd.contains("摸摸头") || cmd.contains("摸摸你") || cmd.contains("摸摸") || cmd.contains("摸一下") || cmd.contains("揉揉头") || cmd.contains("蹭蹭") || cmd.contains("顺毛") || cmd.contains("挠挠")) {
            return PetActionResult("pet", "jumping", "", "呼噜呼噜~ 摸摸头好舒服呀~")
        }

        // 9. 打招呼与握手
        if (cmd.contains("打个招呼") || cmd.contains("挥手") || cmd.contains("打招呼") || cmd.contains("挥挥爪") || cmd.contains("挥爪") || cmd.contains("招招手") || cmd.contains("握手") || cmd.contains("握个手") || cmd.contains("伸爪")) {
            return PetActionResult("wave", "waving", "", "嗨！向你挥挥小爪子，今天也要开心哦！")
        }

        // 10. 困惑 / 装死 / 别动
        if (cmd.contains("装死") || cmd.contains("别动") || cmd.contains("倒下") || cmd.contains("晕一个") || cmd.contains("犯错")) {
            return PetActionResult("failed", "failed", "", "哎呀呀，晕倒啦……装死中，勿扰喵~")
        }

        // 11. 歪头沉思
        if (cmd.contains("歪头") || cmd.contains("想一想") || cmd.contains("思考一下") || cmd.contains("琢磨琢磨")) {
            return PetActionResult("review", "review", "", "嗯……让本喵歪头仔细观察审视一下！")
        }

        // 12. 睡觉与休眠
        if (cmd.contains("去睡觉") || cmd.contains("睡觉吧") || cmd.contains("休息吧") || cmd.contains("打个盹") || cmd.contains("闭嘴睡觉") || cmd.contains("安静睡觉")) {
            return PetActionResult("sleep", "idle", "", "哈欠~ 困啦困啦，趴下打个盹，晚安喵~")
        }

        return null
    }
}

data class PetActionResult(
    val action: String,
    val cue: String,
    val move: String = "",
    val response: String
)
