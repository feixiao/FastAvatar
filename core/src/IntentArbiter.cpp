#include "IntentArbiter.hpp"
#include <string>
#include <string_view>
#include <algorithm>
#include <regex>
#include <vector>
#include <sstream>
#include <mutex>
#include <shared_mutex>

namespace audio_agent {

// 动态注册字典（支持系统或外部动态下发）
static std::shared_mutex s_dict_mutex;
static std::vector<std::string> s_registered_cities = {
    "北京", "上海", "广州", "深圳", "杭州", "成都", "武汉",
    "南京", "重庆", "西安", "苏州", "天津", "长沙", "郑州"
};

static std::vector<std::string> s_registered_contacts = {
    "张三", "李四", "王五", "赵六", "爸爸", "妈妈", "老婆", "老公"
};

void IntentArbiter::RegisterCities(const std::vector<std::string>& cities) {
    std::unique_lock<std::shared_mutex> lock(s_dict_mutex);
    s_registered_cities = cities;
}

void IntentArbiter::RegisterContacts(const std::vector<std::string>& contacts) {
    std::unique_lock<std::shared_mutex> lock(s_dict_mutex);
    s_registered_contacts = contacts;
}

struct ActionRule {
    ActionType type;
    const char* name;
    MotionParams motion;
    std::vector<std::string_view> keywords;
};

static const ActionRule s_action_rules[] = {
    // 1. 姿态：蹲下、坐下、趴下、卧倒
    {
        ActionType::SIT, "sit", {0.0f, 1.0f, ""},
        {"蹲下", "蹲着", "蹲一个", "蹲", "坐下", "坐着", "坐好", "坐", "趴下", "趴着", "趴好", "趴", "卧倒", "躺下", "躺平", "躺着"}
    },
    // 2. 姿态：起立、站起来、站好、起床
    {
        ActionType::STAND, "stand", {0.0f, 1.0f, ""},
        {"站起来", "起来", "站好", "醒醒", "起床"}
    },
    // 3. 姿态：跳跃、纵跳、翻跟头、空翻、跳舞
    {
        ActionType::JUMP, "jump", {0.0f, 1.0f, ""},
        {"跳一下", "跳一个", "跳跳", "翻个跟头", "翻跟头", "蹦一下", "蹦一个", "空翻", "跳起来", "转圈", "跳舞", "跳", "蹦"}
    },
    // 4. 姿态：休息、休眠、睡觉、打盹
    {
        ActionType::REST, "rest", {0.0f, 1.0f, ""},
        {"去睡觉", "睡觉吧", "休息吧", "打个盹", "闭嘴睡觉", "安静睡觉"}
    },
    // 5. 空间移动：向左移 / 奔跑
    {
        ActionType::MOVE_LEFT, "move_left", {-35.0f, 1.0f, "left"},
        {"向左跑", "往左跑", "去左边", "向左走", "往左走", "左边巡视", "左边巡逻", "往左"}
    },
    // 6. 空间移动：向右移 / 奔跑
    {
        ActionType::MOVE_RIGHT, "move_right", {35.0f, 1.0f, "right"},
        {"向右跑", "往右跑", "去右边", "向右走", "往右走", "右边巡视", "右边巡逻", "往右"}
    },
    // 7. 空间移动：前进 / 巡游漫游
    {
        ActionType::MOVE_FORWARD, "move_forward", {30.0f, 1.0f, "forward"},
        {"巡视桌面", "去巡逻", "巡视", "巡逻", "到处逛逛", "溜达溜达", "走两步", "转悠转悠", "往前走", "前进"}
    },
    // 8. 空间运动：停止 / 原地定住
    {
        ActionType::STOP, "stop", {0.0f, 0.0f, ""},
        {"别动", "停下", "停", "定住", "立定"}
    },
    // 9. 交互表达：打招呼 / 握手
    {
        ActionType::GREET, "greet", {0.0f, 1.0f, ""},
        {"打个招呼", "挥手", "打招呼", "招招手", "握手", "握个手", "伸手", "致意"}
    },
    // 10. 交互表达：庆祝 / 欢呼
    {
        ActionType::CELEBRATE, "celebrate", {0.0f, 1.0f, ""},
        {"真棒", "好棒", "庆祝一下", "开心一下"}
    },
    // 11. 交互表达：困惑 / 装死 / 晕倒
    {
        ActionType::CONFUSED, "confused", {0.0f, 1.0f, ""},
        {"装死", "倒下", "晕一个", "犯错", "晕倒"}
    },
    // 12. 交互表达：思考 / 审视 / 歪头
    {
        ActionType::PONDER, "ponder", {0.0f, 1.0f, ""},
        {"歪头", "想一想", "思考一下", "琢磨琢磨"}
    },
    // 13. 交互表达：鸣叫 / 发声
    {
        ActionType::VOCALIZE, "vocalize", {0.0f, 1.0f, ""},
        {"叫一声", "学叫", "发声"}
    },
    // 14. 交互表达：进食 / 干饭
    {
        ActionType::CONSUME, "consume", {0.0f, 1.0f, ""},
        {"吃东西", "喂食", "开饭", "吃饭", "暴风吸入", "大口吃", "吃饱"}
    }
};

static constexpr std::string_view s_memory_prefixes[] = {
    "记住", "帮我记一下", "记一下", "忘掉", "忘了", "清除记忆"
};

static constexpr std::string_view s_persona_prefixes[] = {
    "切换成", "切换为", "变成", "换成", "切换到"
};

/**
 * @brief 清理文本首尾标点、空白与中文语气助词
 */
std::string IntentArbiter::CleanPunctuationAndParticles(std::string_view sv) {
    if (sv.empty()) return "";

    // 清理首尾空白
    size_t start = sv.find_first_not_of(" \t\r\n");
    if (start == std::string_view::npos) return "";
    size_t end = sv.find_last_not_of(" \t\r\n");
    sv = sv.substr(start, end - start + 1);

    // 剥离末尾常见的导航疑问词、语气词与标点
    static constexpr std::string_view trailing_particles[] = {
        "怎么走", "怎么去", "的最佳路线", "的最优路线", "推荐路线", "的路线", "路线",
        "一下吧", "一下呀", "一下啊", "一下",
        "吧", "吗", "呀", "啊", "呢", "哈", "么",
        "。", "！", "？", "，", "、", ".", "!", "?", ","
    };

    bool modified = true;
    while (modified && !sv.empty()) {
        modified = false;
        for (const auto& suffix : trailing_particles) {
            if (sv.size() >= suffix.size() &&
                sv.compare(sv.size() - suffix.size(), suffix.size(), suffix) == 0) {
                sv.remove_suffix(suffix.size());
                modified = true;
                break;
            }
        }
    }

    start = sv.find_first_not_of(" \t\r\n");
    if (start == std::string_view::npos) return "";
    end = sv.find_last_not_of(" \t\r\n");
    return std::string(sv.substr(start, end - start + 1));
}

int IntentArbiter::ChineseDigitToVal(std::string_view d) {
    if (d == "一") return 1;
    if (d == "二" || d == "两") return 2;
    if (d == "三") return 3;
    if (d == "四") return 4;
    if (d == "五") return 5;
    if (d == "六") return 6;
    if (d == "七") return 7;
    if (d == "八") return 8;
    if (d == "九") return 9;
    if (d == "零") return 0;
    return -1;
}

int IntentArbiter::ParseChineseNumberToken(std::string_view token) {
    if (token.empty()) return -1;
    size_t pos_ten = token.find("十");
    if (pos_ten != std::string_view::npos) {
        int tens = 1;
        if (pos_ten > 0) {
            std::string_view pre = token.substr(0, pos_ten);
            tens = ChineseDigitToVal(pre);
            if (tens < 0) tens = 1;
        }
        int units = 0;
        if (pos_ten + 3 < token.size()) {
            std::string_view post = token.substr(pos_ten + 3);
            units = ChineseDigitToVal(post);
            if (units < 0) units = 0;
        }
        return tens * 10 + units;
    }
    return ChineseDigitToVal(token);
}

int IntentArbiter::ParseChineseNumber(const std::string& text) {
    static const std::regex num_reg(R"((\d+))");
    std::smatch m;
    if (std::regex_search(text, m, num_reg)) {
        try {
            return std::stoi(m[1].str());
        } catch (...) {}
    }
    return ParseChineseNumberToken(text);
}


std::string IntentArbiter::ExtractMusicQuery(const std::string& query) {
    std::string s = CleanPunctuationAndParticles(query);
    if (s.empty()) return "";

    static constexpr std::string_view music_prefixes[] = {
        "放一首", "放首", "播放一首", "播放首", "播放", "听一首", "听听",
        "听音乐", "放音乐", "放歌", "听歌", "唱一首", "点一首",
        "我想听", "我要听", "帮我放", "来一首", "来首", "放"
    };

    std::string_view sv(s);
    std::string_view song_view;
    for (const auto& pre : music_prefixes) {
        size_t p = sv.find(pre);
        if (p != std::string_view::npos) {
            song_view = sv.substr(p + pre.size());
            break;
        }
    }

    return CleanPunctuationAndParticles(song_view);
}

std::string IntentArbiter::ExtractAlarmTime(const std::string& query) {
    // 1. 匹配标准冒号时间 如 "07:30", "8:00"
    static const std::regex time_colon(R"((\d{1,2}):(\d{2}))");
    std::smatch m;
    if (std::regex_search(query, m, time_colon)) {
        int h = std::stoi(m[1].str());
        int min = std::stoi(m[2].str());
        char buf[16];
        snprintf(buf, sizeof(buf), "%02d:%02d", h, min);
        return std::string(buf);
    }

    // 2. 匹配阿拉伯数字口语时间 如 "7点半", "8点30分", "8点"
    static const std::regex time_point(R"((\d{1,2})点(\d{1,2}|半)?(?:分)?)");
    if (std::regex_search(query, m, time_point)) {
        int h = std::stoi(m[1].str());
        int min = 0;
        if (m[2].matched) {
            if (m[2].str() == "半") {
                min = 30;
            } else {
                min = std::stoi(m[2].str());
            }
        }
        if (query.find("下午") != std::string::npos || query.find("晚上") != std::string::npos) {
            if (h < 12) h += 12;
        }
        char buf[16];
        snprintf(buf, sizeof(buf), "%02d:%02d", h, min);
        return std::string(buf);
    }

    // 3. 匹配中文数字口语时间 如 "七点半", "八点"
    static const std::regex cn_time_point(R"(((?:(?:一|二|两|三|四|五|六|七|八|九)?十(?:一|二|两|三|四|五|六|七|八|九)?|(?:一|二|两|三|四|五|六|七|八|九)))点((?:(?:一|二|两|三|四|五|六|七|八|九)?十(?:一|二|两|三|四|五|六|七|八|九)?|(?:一|二|两|三|四|五|六|七|八|九))|半)?(?:分)?)");
    if (std::regex_search(query, m, cn_time_point)) {
        int h = ParseChineseNumberToken(m[1].str());
        int min = 0;
        if (m[2].matched) {
            if (m[2].str() == "半") {
                min = 30;
            } else {
                min = ParseChineseNumberToken(m[2].str());
                if (min < 0) min = 0;
            }
        }
        if (h >= 0 && h <= 24) {
            if (query.find("下午") != std::string::npos || query.find("晚上") != std::string::npos) {
                if (h < 12) h += 12;
            }
            char buf[16];
            snprintf(buf, sizeof(buf), "%02d:%02d", h, min);
            return std::string(buf);
        }
    }

    // 槽位缺失严格返回空串，严禁伪造默认时间
    return "";
}

std::string IntentArbiter::ExtractWeatherCity(const std::string& query) {
    std::shared_lock<std::shared_mutex> lock(s_dict_mutex);
    for (const auto& city : s_registered_cities) {
        if (query.find(city) != std::string::npos) {
            return city;
        }
    }
    return "";
}

std::string IntentArbiter::ExtractPhoneTarget(const std::string& query) {
    // 1. 匹配纯数字电话号码 (3 ~ 11 位)
    static const std::regex num_reg(R"((\d{3,11}))");
    std::smatch m;
    if (std::regex_search(query, m, num_reg)) {
        return m[1].str();
    }

    // 2. 匹配已挂载联系人字典
    {
        std::shared_lock<std::shared_mutex> lock(s_dict_mutex);
        for (const auto& contact : s_registered_contacts) {
            if (query.find(contact) != std::string::npos) {
                return contact;
            }
        }
    }

    // 3. 语法提取模式 ("打电话给XX", "给XX打电话", "呼叫XX", "拨打XX")
    static constexpr std::string_view phone_prefixes[] = {
        "打电话给", "电话给", "呼叫", "拨打", "拨给"
    };
    std::string_view q(query);
    for (const auto& pre : phone_prefixes) {
        size_t p = q.find(pre);
        if (p != std::string_view::npos) {
            std::string target = CleanPunctuationAndParticles(q.substr(p + pre.size()));
            if (!target.empty()) return target;
        }
    }

    return "";
}

ActionCommand IntentArbiter::ArbitrateAction(const std::string& query) {
    ActionCommand cmd;
    if (query.empty()) return cmd;

    std::string_view q(query);
    // 排除疑问与开放闲聊
    if (q.find("为什么") != std::string_view::npos ||
        q.find("怎么会") != std::string_view::npos ||
        q.find("是什么") != std::string_view::npos ||
        q.find("如何") != std::string_view::npos ||
        q.find("笑话") != std::string_view::npos ||
        q.find("你是谁") != std::string_view::npos) {
        return cmd;
    }

    for (const auto& rule : s_action_rules) {
        for (const auto& kw : rule.keywords) {
            // 单字指令仅在全句完全一致时匹配 (如 "停", "坐")，多字词组支持子串匹配 (如 "停下", "别动")
            bool match = (kw.size() > 3) ? (q.find(kw) != std::string_view::npos) : (q == kw);
            if (match) {
                cmd.type = rule.type;
                cmd.action_name = rule.name;
                cmd.motion = rule.motion;
                cmd.confidence = 1.0f;
                cmd.raw_query = query;
                return cmd;
            }
        }
    }
    return cmd;
}

ArbitrationResult IntentArbiter::Arbitrate(
    const std::string& query,
    const std::string& last_route,
    const std::string& last_domain
) {
    ArbitrationResult res;
    if (query.empty()) return res;

    std::string_view q(query);

    // 1. 严格防冲突前置过滤：开放常识、时间问询、笑话、自我介绍等一律跳过指令匹配直接进入分类器
    bool bypass_command_matching = false;
    if (q.find("为什么") != std::string_view::npos ||
        q.find("怎么会") != std::string_view::npos ||
        q.find("是什么") != std::string_view::npos ||
        q.find("笑话") != std::string_view::npos ||
        q.find("几点了") != std::string_view::npos ||
        q.find("时间") != std::string_view::npos ||
        q.find("你是谁") != std::string_view::npos ||
        q.find("自我介绍") != std::string_view::npos) {
        bypass_command_matching = true;
    }

    if (!bypass_command_matching) {

    // 2. 本地记忆指令前缀
    for (const auto& pre : s_memory_prefixes) {
        if (q.find(pre) == 0) {
            res.status = ArbiterStatus::MATCHED;
            res.domain = ArbiterDomain::MEMORY_COMMAND;
            res.intent = "memory.command";
            res.confidence = 1.0f;
            res.slots["query"] = query;
            res.payload_json = R"({"domain":"MEMORY_COMMAND","intent":"memory.command","query":")" + query + R"("})";
            return res;
        }
    }

    // 3. 伴侣人设切换指令前缀
    for (const auto& pre : s_persona_prefixes) {
        if (q.find(pre) == 0) {
            std::string target = "companion";
            if (q.find("猫") != std::string_view::npos || q.find("宠物") != std::string_view::npos) {
                target = "pet";
            } else if (q.find("毒舌") != std::string_view::npos || q.find("损友") != std::string_view::npos) {
                target = "roaster";
            } else if (q.find("幕僚") != std::string_view::npos || q.find("助理") != std::string_view::npos || q.find("顾问") != std::string_view::npos || q.find("效率") != std::string_view::npos) {
                target = "jarvis";
            }
            res.status = ArbiterStatus::MATCHED;
            res.domain = ArbiterDomain::PERSONA_SWITCH;
            res.intent = "persona.switch";
            res.action = "switch";
            res.slots["persona"] = target;
            res.confidence = 1.0f;
            res.payload_json = R"({"domain":"PERSONA_SWITCH","intent":"persona.switch","persona":")" + target + R"("})";
            return res;
        }
    }

    // 4. 纯动作指令匹配 (零角色概念)
    ActionCommand act = ArbitrateAction(query);
    if (act.type != ActionType::UNKNOWN) {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::ACTION_COMMAND;
        res.intent = "action.command";
        res.action = act.action_name;
        res.action_cmd = act;
        res.confidence = act.confidence;
        res.slots["action"] = act.action_name;
        res.slots["direction"] = act.motion.direction;
        res.payload_json = R"({"domain":"ACTION_COMMAND","intent":"action.command","action":")" + std::string(act.action_name) +
                           R"(","direction":")" + std::string(act.motion.direction) +
                           R"(","distance":)" + std::to_string(act.motion.distance) +
                           R"(,"confidence":)" + std::to_string(act.confidence) + R"(})";
        return res;
    }
    // 3. 闹钟意图
    if (q.find("闹钟") != std::string_view::npos ||
        q.find("叫我起床") != std::string_view::npos ||
        q.find("提醒我") != std::string_view::npos) {
        res.intent = "set_alarm";
        std::string alarm_time = ExtractAlarmTime(query);
        if (alarm_time.empty()) {
            res.status = ArbiterStatus::SLOT_MISSING;
            res.missing_slot = "time";
        } else {
            res.status = ArbiterStatus::MATCHED;
            res.domain = ArbiterDomain::SYSTEM_TOOL;
            res.slots["time"] = alarm_time;
            res.payload_json = R"({"action":"set_alarm","time":")" + alarm_time + R"("})";
            res.confidence = 1.0f;
        }
        return res;
    }

    // 4. 音乐播控
    // 4.1 系统播控快捷指令 (暂停 / 停止 / 继续 / 切歌)
    if (q.find("暂停音乐") != std::string_view::npos ||
        q.find("关闭音乐") != std::string_view::npos ||
        q.find("停止音乐") != std::string_view::npos ||
        q.find("关掉音乐") != std::string_view::npos ||
        q.find("别放了") != std::string_view::npos ||
        q.find("停止播放") != std::string_view::npos ||
        q.find("暂停播放") != std::string_view::npos ||
        q == "暂停" || q == "停止") {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "play_music";
        res.action = "pause";
        res.payload_json = R"({"action":"play_music","command":"pause"})";
        return res;
    }
    if (q.find("继续播放") != std::string_view::npos ||
        q.find("恢复播放") != std::string_view::npos ||
        q == "继续") {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "play_music";
        res.action = "resume";
        res.payload_json = R"({"action":"play_music","command":"resume"})";
        return res;
    }
    if (q.find("下一首") != std::string_view::npos ||
        q.find("切歌") != std::string_view::npos ||
        q.find("换一首") != std::string_view::npos ||
        q.find("换首歌") != std::string_view::npos ||
        q.find("下一曲") != std::string_view::npos) {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "play_music";
        res.action = "next";
        res.payload_json = R"({"action":"play_music","command":"next"})";
        return res;
    }
    if (q.find("上一首") != std::string_view::npos || q.find("上一曲") != std::string_view::npos) {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "play_music";
        res.action = "previous";
        res.payload_json = R"({"action":"play_music","command":"previous"})";
        return res;
    }
    // 4.2 点歌与曲目搜索播放
    if (q.find("放歌") != std::string_view::npos ||
        q.find("听歌") != std::string_view::npos ||
        q.find("放首") != std::string_view::npos ||
        q.find("放一首") != std::string_view::npos ||
        q.find("放音乐") != std::string_view::npos ||
        q.find("听音乐") != std::string_view::npos ||
        q.find("播放") != std::string_view::npos) {
        std::string song = ExtractMusicQuery(query);
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "play_music";
        if (song.empty()) {
            res.payload_json = R"({"action":"play_music","command":"play"})";
        } else {
            res.slots["query"] = song;
            res.payload_json = R"({"action":"play_music","query":")" + song + R"("})";
        }
        return res;
    }

    // 5. 天气查询
    if (q.find("天气") != std::string_view::npos ||
        q.find("下雨") != std::string_view::npos ||
        q.find("降温") != std::string_view::npos ||
        q.find("气温") != std::string_view::npos) {
        res.status = ArbiterStatus::MATCHED;
        res.domain = ArbiterDomain::SYSTEM_TOOL;
        res.intent = "query_weather";
        std::string city = ExtractWeatherCity(query);
        if (city.empty()) {
            res.slots.emplace("city", "current_location");
            res.payload_json = R"({"action":"query_weather","city":"current_location"})";
        } else {
            res.payload_json = R"({"action":"query_weather","city":")" + city + R"("})";
            res.slots.emplace("city", std::move(city));
        }
        return res;
    }

    // 6. 电话拨号
    if (q.find("打电话") != std::string_view::npos ||
        q.find("拨打") != std::string_view::npos ||
        q.find("呼叫") != std::string_view::npos) {
        std::string target = ExtractPhoneTarget(query);
        res.intent = "make_phone_call";
        if (target.empty()) {
            res.status = ArbiterStatus::SLOT_MISSING;
            res.missing_slot = "phone_number";
        } else {
            res.status = ArbiterStatus::MATCHED;
            res.domain = ArbiterDomain::SYSTEM_TOOL;
            res.slots["phone_number"] = target;
            res.payload_json = R"({"action":"make_phone_call","phone_number":")" + target + R"("})";
            res.confidence = 1.0f;
        }
        return res;
    }
    }

    res.status = ArbiterStatus::UNMATCHED;
    res.domain = ArbiterDomain::OPEN_ENDED;
    res.intent = "dialog.open_ended";
    res.classification = TinyIntentClassifier::Classify(query, last_route, last_domain);
    res.payload_json = R"({"domain":"OPEN_ENDED","intent":"dialog.open_ended","matched":false,"need_cloud":)" +
                       std::string(res.classification.need_deep_reasoning ? "true" : "false") +
                       R"(,"cloud_probability":)" + std::to_string(res.classification.cloud_probability) +
                       R"(,"primary_domain":")" + std::string(res.classification.domain_name) +
                       R"(","reason":")" + res.classification.reason + R"("})";
    return res;
}

std::string IntentArbiter::ResolveToolCall(const std::string& query) {
    ArbitrationResult res = Arbitrate(query);
    if (res.status == ArbiterStatus::MATCHED && res.domain == ArbiterDomain::SYSTEM_TOOL) {
        return std::move(res.payload_json);
    }
    return "";
}

std::string IntentArbiter::ResolveDispatchJson(
    const std::string& query,
    const std::string& last_route,
    const std::string& last_domain
) {
    ArbitrationResult res = Arbitrate(query, last_route, last_domain);
    if (!res.payload_json.empty()) {
        return res.payload_json;
    }
    return R"({"domain":"OPEN_ENDED","intent":"dialog.open_ended","matched":false})";
}
} // namespace audio_agent
