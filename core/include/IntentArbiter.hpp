#pragma once

#include <string>
#include <string_view>
#include <vector>
#include <unordered_map>
#include "TinyIntentClassifier.hpp"

namespace audio_agent {

/**
 * @brief 意图所属领域
 */
enum class ArbiterDomain : uint8_t {
    OPEN_ENDED = 0,     ///< 开放域问答/复杂闲聊，转交 LLM 认知环
    ACTION_COMMAND = 1, ///< 通用动作/运动学控制指令 (姿态/位移/表达)
    SYSTEM_TOOL = 2,    ///< 系统/硬件设备控制 (闹钟/音乐/天气/拨号)
    PERSONA_SWITCH = 3, ///< 伴侣人设切换
    MEMORY_COMMAND = 4  ///< 本地记忆指令
};

/**
 * @brief 意图仲裁状态
 */
enum class ArbiterStatus : uint8_t {
    UNMATCHED = 0,      ///< 未命中确定性指令，透传 LLM
    MATCHED = 1,        ///< 明确匹配指令且槽位完整
    SLOT_MISSING = 2    ///< 命中指令但关键槽位缺失 (需追问澄清)
};

/**
 * @brief 纯动作语义类型 (通用契约，零角色与零展现形态耦合)
 */
enum class ActionType : uint16_t {
    UNKNOWN = 0,

    // 姿态类动作
    SIT,            // 蹲下、坐下、趴下、卧倒
    STAND,          // 起立、站起来、站好、起床
    JUMP,           // 跳跃、纵跳、翻跟头、空翻
    REST,           // 休息、打盹、卧倒休眠

    // 空间运动类动作 (带矢量参数)
    MOVE_FORWARD,   // 前进、向前走
    MOVE_BACKWARD,  // 后退、向后退
    MOVE_LEFT,      // 向左移、向左跑、向左走
    MOVE_RIGHT,     // 向右移、向右跑、向右走
    STOP,           // 停下、别动、原地定住

    // 交互与表达类动作
    GREET,          // 打招呼、招手、致意、握手
    CELEBRATE,      // 欢呼、转圈、开心动作
    CONFUSED,       // 困惑、疑问、装死、晕倒
    PONDER,         // 思考、审视、歪头观察
    VOCALIZE,       // 鸣叫、发声 (叫一声)
    CONSUME         // 进食、干饭、暴风吸入
};

/**
 * @brief 空间运动学意图参数
 */
struct MotionParams {
    float distance{0.0f};       ///< 位移幅度建议值 (相对步长或单位距离)
    float speed{1.0f};          ///< 运动速度建议倍率
    const char* direction{""};  ///< 方向 ("forward", "backward", "left", "right", "")
};

/**
 * @brief 纯动作指令结构体 (零角色耦合)
 */
struct ActionCommand {
    ActionType type{ActionType::UNKNOWN};
    const char* action_name{""};    ///< 动作标识 ("sit", "stand", "jump", "move_left", "move_right", "stop"...)
    MotionParams motion;            ///< 伴随的空间运动学参数
    float confidence{1.0f};         ///< 意图置信度
    std::string raw_query;          ///< 触发该动作的原始语句
};

/**
 * @brief 结构化意图仲裁决策结果
 */
struct ArbitrationResult {
    ArbiterStatus status{ArbiterStatus::UNMATCHED};
    ArbiterDomain domain{ArbiterDomain::OPEN_ENDED};
    std::string intent;                 ///< 意图名称 ("action.command", "play_music", "set_alarm", "memory.command")
    std::string action;                 ///< 动作或子命令
    ActionCommand action_cmd;           ///< 若 domain == ACTION_COMMAND，携带标准动作结构体
    ClassificationResult classification; ///< 若 domain == OPEN_ENDED，携带微型端侧分类器评估结果
    std::unordered_map<std::string, std::string> slots; ///< 提取的语义槽位
    std::string missing_slot;           ///< 若状态为 SLOT_MISSING，记录缺失的必填槽位名
    float confidence{0.0f};             ///< 置信度 (0.0 ~ 1.0)
    std::string payload_json;           ///< 标准化生成的 JSON (供上层 CGO / JNI 跨语言消费)
};
/**
 * @brief 生产级端侧意图仲裁器 (Production Fast-Path Intent Arbiter)
 * 
 * 核心架构准则 (依据工业级车载/端侧 Agent 规范)：
 * 1. 零虚假兜底 (Zero Hallucination)：槽位缺失时明确标记追问或退避，严禁私自脑补虚假城市/歌曲/地名/号码；
 * 2. 规则与实体解耦：支持动态注册实体字典（通讯录、城市、POI 等），无需重新编译；
 * 3. 泛化中文规整 (Universal Normalizer)：支持通用中英文数字/时间双向解析，杜绝枚举有限数组；
 * 4. 防冲突高专属性前置：严格拦截开放域问答与闲聊，避免误触发系统控制；
 * 5. 结构化契约输出：提供结构化 `ArbitrationResult` 供上层对话状态跟踪 (DST) 消费。
 */
class IntentArbiter {
public:
    /**
     * @brief 结构化决策入口 (生产推荐)
     */
    static ArbitrationResult Arbitrate(
        const std::string& query,
        const std::string& last_route = "",
        const std::string& last_domain = ""
    );
    /**
     * @brief 判定 Query 是否匹配已知系统工具，若匹配且槽位完整则返回标准格式的 Tool Call JSON；否则返回空串
     */
    static std::string ResolveToolCall(const std::string& query);


    /**
     * @brief 纯动作指令决策入口 (若非动作指令返回 ActionType::UNKNOWN)
     */
    static ActionCommand ArbitrateAction(const std::string& query);

    /**
     * @brief 统一输出结构化分流 JSON 串 (供 C-API / CGO / JNI 直接消费)
     */
    static std::string ResolveDispatchJson(
        const std::string& query,
        const std::string& last_route = "",
        const std::string& last_domain = ""
    );
    /**
     * @brief 提取音乐查询词 (若为空串则由上层判定为通用播放)
     */
    static std::string ExtractMusicQuery(const std::string& query);

    /**
     * @brief 提取闹钟设定时间 (格式 HH:mm，未指定时间返回空串)
     */
    static std::string ExtractAlarmTime(const std::string& query);

    /**
     * @brief 提取天气查询城市 (未指定城市返回空串，代表默认当前位置)
     */
    static std::string ExtractWeatherCity(const std::string& query);

    /**
     * @brief 提取电话拨号号码或联系人 (未指定返回空串)
     */
    static std::string ExtractPhoneTarget(const std::string& query);

    /**
     * @brief 动态注册城市实体词典 (支持外部下发或系统定位注入)
     */
    static void RegisterCities(const std::vector<std::string>& cities);

    /**
     * @brief 动态注册联系人实体词典 (支持系统通讯录动态挂载)
     */
    static void RegisterContacts(const std::vector<std::string>& contacts);

    /**
     * @brief 清理文本首尾标点、空白与中文语气助词
     */
    static std::string CleanPunctuationAndParticles(std::string_view text);

    /**
     * @brief 通用中文/阿拉伯数字转整数 (支持 0 ~ 99)
     */
    static int ParseChineseNumber(const std::string& text);

private:
    static int ChineseDigitToVal(std::string_view d);
    static int ParseChineseNumberToken(std::string_view token);
};

} // namespace audio_agent
