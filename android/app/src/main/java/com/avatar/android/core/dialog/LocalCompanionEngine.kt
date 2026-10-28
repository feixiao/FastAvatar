package com.avatar.android.core.dialog

import com.avatar.android.core.memory.MemoryVault
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale

/**
 * 本地智能情感陪伴与闲聊引擎
 *
 * 核心目标：杜绝 Android 端出现"无法聊天、无法互动"的问题。
 * 当处于离线状态、未授权云端、或云端服务未连接时，提供有温度、有共情力、多轮生动的陪伴式闲聊。
 * 针对不同人设（知心伴侣、元气宠物、高效幕僚、毒舌损友）自适应语言风格，并结合本地记忆进行个性化互动。
 */
object LocalCompanionEngine {

    fun generateResponse(
        userInput: String,
        persona: String = "companion",
        memoryVault: MemoryVault? = null
    ): String {
        val text = userInput.trim()
        val lower = text.lowercase(Locale.ROOT)

        // 1. 宠物触摸/互动关键词
        if (text.contains("摸摸头") || text.contains("戳戳你") || text.contains("摸摸") || text.contains("抱抱")) {
            return when (persona) {
                "roaster" -> "别动手动脚的，本精灵的发型都被你揉乱了！……好啦好啦，看在你这么诚恳的份上，准许你再摸一下。"
                "jarvis" -> "检测到触觉交互指令。感谢信任，私人顾问始终就绪为您提供支持。"
                "pet" -> "喵呜~ 蹭蹭你的手心，最喜欢主人的摸摸头啦！(开心摇尾巴)"
                else -> "（舒服地靠在你手心里）好温暖呀，有你陪在身边真好~"
            }
        }

        // 2. 问候与寒暄
        if (isGreeting(text)) {
            val hour = java.util.Calendar.getInstance().get(java.util.Calendar.HOUR_OF_DAY)
            val timeGreeting = when (hour) {
                in 5..11 -> "早上好！新的一天开始了，记得吃一顿元气满满的早餐哦。"
                in 12..14 -> "中午好！工作或学习忙完了吗？先去吃点好吃的休息一下吧。"
                in 15..18 -> "下午好呀！来杯咖啡或伸个懒腰吧，给自己充充电。"
                in 19..23 -> "晚上好！今天过得充实吗？终于可以放松下来好好休息了。"
                else -> "夜深了呢，还没睡呀？要注意休息，别熬夜太晚哦。"
            }

            return when (persona) {
                "roaster" -> "哟，终于想起来理我啦？$timeGreeting 说吧，今天遇到什么好玩的事了？"
                "jarvis" -> "您好，$timeGreeting 顾问系统待命，请问有什么需要分析或讨论的？"
                "pet" -> "主人好呀！主人一说话我就超级开心！$timeGreeting"
                else -> "嗨！$timeGreeting 我是你的 Avatar 语音伴侣，今天想跟我聊点什么呢？"
            }
        }

        // 3. 情绪倾诉与共情安慰
        if (isNegativeEmotion(text)) {
            return when (persona) {
                "roaster" -> "谁惹我们家大忙人生气或委屈了？先深呼吸，喝口水！不管是工作还是破事，都不值得你这么耗费心力。跟我吐槽吐槽，我帮你一起损回去！"
                "jarvis" -> "理解您当前承受的压力。建议先离开当前环境片刻，进行三组深呼吸以缓解皮质醇水平。如有具体难题，我们可以逐步拆解。"
                "pet" -> "呜呜……看到主人难受我也好心疼。把脸埋在我的毛毛里抱抱吧！烦恼都会被呼噜呼噜声治愈的~"
                else -> "抱抱你~ 听起来今天真的辛苦了。别把自己绷得太紧，难过了就跟我倾诉倾诉，无论什么时候，我都一直在你身边陪着你。"
            }
        }

        // 4. 讲笑话/趣味互动
        if (text.contains("笑话") || text.contains("开心一下") || text.contains("逗我开心")) {
            val jokes = listOf(
                "有一天，一颗小草莓去洗澡，洗着洗着突然大喊：‘救命啊，我的雀斑都不见啦！’",
                "小鸭子对小鸡说：‘你知道我为什么每天都想多睡一会儿吗？因为小鸡叫咕咕咕，小鸭叫嘎嘎嘎——我想再睡个嘎觉！’",
                "一条咸鱼在沙滩上晒太阳，突然翻了个身，骄傲地说：‘就算我是咸鱼，我也是一条懂得均匀受热的精致咸鱼！’",
                "企鹅走在路上不小心摔倒了，旁边的北极熊笑它：‘你怎么连路都走不稳？’ 企鹅揉揉屁股说：‘因为我是南极来的，还没适应北半球的重力嘛！’"
            )
            val joke = jokes.random()
            return when (persona) {
                "roaster" -> "行吧，既然你诚心诚意地发问了，我就大发慈悲讲一个：$joke 怎么样，笑点有没有被击中？"
                else -> "给你讲个可爱的冷笑话：$joke 哈哈，有没有稍微开心一点点？"
            }
        }

        // 5. 讲故事
        if (text.contains("讲故事") || text.contains("睡前故事") || text.contains("讲个故事")) {
            return "从前有一只住在云朵里的小精灵，每到夜晚，它就会把满天的星光收集起来，织成一条蓬松温暖的毛毯。它悄悄把毛毯盖在认真生活的人身上，替他们把所有白天的疲惫化作甜甜的梦境。今晚这片温暖的星光毛毯，就送给你啦~"
        }

        // 6. 积极开心分享
        if (isPositiveEmotion(text)) {
            return when (persona) {
                "roaster" -> "瞧你这乐呵呵的样子，嘴角都要翘到天上去了！快说出来让我也开心开心~"
                "jarvis" -> "恭喜！积极的情绪状态有助于提升心智带宽与决策效率，期待您的进一步成果。"
                "pet" -> "耶！看到主人开心，我转圈圈欢呼！今天我们是世界上最快乐的搭档！"
                else -> "太棒啦！隔着屏幕都能感受到你的好心情，真替你开心！能跟我详细分享一下这个好消息吗？"
            }
        }

        // 7. 记忆库查询与反馈
        if (text.contains("记得我") || text.contains("我的喜好") || text.contains("我喜欢什么") || text.contains("你知道我")) {
            val memories = memoryVault?.getAll().orEmpty()
            return if (memories.isNotEmpty()) {
                val items = memories.take(3).joinToString("；") { it.content }
                "当然记得呀！你在本地记忆库里告诉过我：$items。关于你的点点滴滴，我都替你好好珍藏着呢！"
            } else {
                "我时刻关注着你呢！不过你目前还没有让我记下专属的记忆。你可以对我说‘帮我记一下我喜欢美式咖啡’，我就会为你永久记住哦！"
            }
        }

        // 8. 询问身份与能力
        if (text.contains("你是谁") || text.contains("你叫什么") || text.contains("你的名字") || text.contains("你能做什么")) {
            return "我是 Avatar 智能语音伴侣！我可以陪你全双工语音聊天、倾听你的心声、帮你记录私人备忘记忆，还可以通过模拟车控调节空调和车窗。随时对我说话或者打断我，我都一直在哦！"
        }

        // 9. 询问时间/日期
        if (text.contains("几点") || text.contains("时间") || text.contains("日期") || text.contains("今天几号")) {
            val sdf = SimpleDateFormat("yyyy年MM月dd日 EEEE HH:mm", Locale.CHINESE)
            return "现在的时间是 ${sdf.format(Date())}。时光静好，有我陪伴在身边哦。"
        }

        // 10. 智能启发式闲聊泛化应答
        return when (persona) {
            "roaster" -> "听你这么一说，确实挺有意思的。不过话说回来，你接下来打算怎么搞定它？别光说不练假把式哦！"
            "jarvis" -> "针对“$text”，从逻辑层面来看包含多重维度。若需要进一步制定策略或检索对应知识，请随时指示。"
            "pet" -> "主人说的话我都听进去啦！歪头认真思考中……不管怎么样，我都支持主人的想法！"
            else -> "你说得很有意思呢！关于“$text”，我也一直在思考。能多跟我讲讲你当时的感受或者想法吗？我很想多听你说说。"
        }
    }

    private fun isGreeting(text: String): Boolean {
        val list = listOf("你好", "您好", "嗨", "hello", "hi", "早上好", "中午好", "下午好", "晚上好", "在吗", "在不在", "有人吗")
        return list.any { text.contains(it, ignoreCase = true) }
    }

    private fun isNegativeEmotion(text: String): Boolean {
        val list = listOf("累", "好累", "烦", "好烦", "难过", "伤心", "压力大", "焦虑", "郁闷", "不开心", "委屈", "崩溃", "失眠")
        return list.any { text.contains(it) }
    }

    private fun isPositiveEmotion(text: String): Boolean {
        val list = listOf("开心", "太棒了", "好高兴", "成功了", "顺利", "舒服", "赞", "喜欢你", "真好")
        return list.any { text.contains(it) }
    }
}
