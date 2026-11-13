#include "TinyIntentClassifier.hpp"
#include <string>
#include <string_view>
#include <vector>
#include <cmath>
#include <algorithm>

namespace audio_agent {

namespace {

inline float Sigmoid(float x) {
    return 1.0f / (1.0f + std::exp(-x));
}

// 1. 数学、公式与算法特征
static constexpr std::string_view s_math_features[] = {
    "求导", "导数", "微分", "微积分", "不定积分", "定积分",
    "贝叶斯", "方程", "一元二次", "函数极限", "矩阵", "特征值", "特征向量",
    "牛顿法", "二叉树", "动态规划", "贪心算法", "时间复杂度", "空间复杂度",
    "算法推导", "数学证明", "公式推导", "概率分布", "正态分布", "傅里叶变换",
    "x^", "y^", "x^2", "x^3", "lnx", "sin(", "cos(", "log("
};

// 2. 编程、代码编写与调试排错特征
static constexpr std::string_view s_code_features[] = {
    "代码", "写一段", "写个", "写一个", "编写", "实现", "函数",
    "rust", "python", "golang", "c++", "java", "sql", "typescript",
    "无锁", "环形队列", "并发", "死锁", "内存泄漏", "段错误", "空指针",
    "goroutine", "线程安全", "并发竞争", "race condition", "堆栈溢出", "stack overflow",
    "编译报错", "语法错误", "异常堆栈", "traceback", "debug", "反编译", "反汇编"
};

// 3. 架构设计与专业科学深度分析特征
static constexpr std::string_view s_architecture_features[] = {
    "架构设计", "系统架构", "设计模式", "高可用架构", "分布式事务",
    "微服务架构", "分库分表", "缓存击穿", "缓存雪崩", "一致性哈希",
    "论文综述", "学术推导", "深入分析", "对比分析", "原理解释", "技术方案"
};

// 4. 多轮对话短指代与追问特征 (用于 DST 继承)
static constexpr std::string_view s_followup_features[] = {
    "第", "行", "为什么", "怎么回事", "改一下", "换个写法", "再优化",
    "还有吗", "看不懂", "继续", "详细说说", "然后呢", "解释一下", "为什么这样"
};

// 5. 日常闲聊、情绪倾诉与生活陪伴特征 (用于抵消复杂度)
static constexpr std::string_view s_casual_features[] = {
    "散步", "公园", "流浪猫", "小猫", "小狗", "花坛", "小鸟", "吃饭",
    "睡觉", "晚安", "早安", "你好", "哈哈", "无聊", "难过", "开心",
    "好累", "抱抱", "想你", "休息", "听歌", "放首歌", "聊天", "陪我"
};

} // namespace

ClassificationResult TinyIntentClassifier::Classify(
    const std::string& query,
    const std::string& last_route,
    const std::string& last_domain
) {
    ClassificationResult result;
    if (query.empty()) {
        result.need_deep_reasoning = false;
        result.cloud_probability = 0.0f;
        result.domain = ReasoningDomain::CHAT_CASUAL;
        result.domain_name = "chat";
        result.reason = "空查询";
        return result;
    }

    std::string lower_query;
    lower_query.reserve(query.size());
    for (char c : query) {
        lower_query.push_back(static_cast<char>(std::tolower(static_cast<unsigned char>(c))));
    }
    std::string_view q(lower_query);
    float logit = -1.2f; // 默认基准偏置 (倾向于优先端侧小模型以降低延迟)
    int math_matches = 0;
    for (const auto& feat : s_math_features) {
        if (q.find(feat) != std::string_view::npos) {
            math_matches++;
        }
    }

    int code_matches = 0;
    for (const auto& feat : s_code_features) {
        if (q.find(feat) != std::string_view::npos) {
            code_matches++;
        }
    }

    int arch_matches = 0;
    for (const auto& feat : s_architecture_features) {
        if (q.find(feat) != std::string_view::npos) {
            arch_matches++;
        }
    }

    int casual_matches = 0;
    for (const auto& feat : s_casual_features) {
        if (q.find(feat) != std::string_view::npos) {
            casual_matches++;
        }
    }

    // 1. 深度数理特征 (高权重)
    if (math_matches > 0) {
        logit += math_matches * 3.5f;
    }

    // 2. 编程代码特征 (高权重)
    if (code_matches > 0) {
        logit += code_matches * 3.2f;
    }

    // 3. 架构方案与深度学术分析 (中高权重)
    if (arch_matches > 0) {
        logit += arch_matches * 2.8f;
    }

    // 4. 多轮对话状态跟踪 (DST: Dialogue State Tracking)
    // 若上一轮经由云端大模型深度处理 (如代码/数理/架构)，当前短指代追问强制继承
    bool is_context_followup = false;
    if (last_route == "CLOUD_CHAT" || last_domain == "code" || last_domain == "math" || last_domain == "architecture") {
        for (const auto& feat : s_followup_features) {
            if (q.find(feat) != std::string_view::npos) {
                is_context_followup = true;
                break;
            }
        }
        if (is_context_followup) {
            logit += 4.5f; // 强上下文继承，杜绝精神分裂
        }
    }

    // 5. 日常闲聊生活陪伴柔化 (负向抑制，防止生活长故事误判)
    if (casual_matches > 0) {
        logit -= casual_matches * 2.2f;
    }

    // 6. 常见儿童级短问答修正 (如短句 "为什么吃鱼"、"为什么天黑")
    // 文本短且无专业术语时，抑制 "为什么" 的云端倾向
    if (q.find("为什么") != std::string_view::npos && q.size() <= 24 && math_matches == 0 && code_matches == 0 && arch_matches == 0) {
        logit -= 2.0f;
    }

    float prob = Sigmoid(logit);
    result.cloud_probability = prob;
    result.need_deep_reasoning = (prob >= 0.5f);

    // 确定具体推理领域与决策解释
    if (is_context_followup) {
        result.domain = ReasoningDomain::CONTEXT_FOLLOWUP;
        result.domain_name = "context_followup";
        result.reason = "多轮上下文追问继承 (上一轮为云端深度推理)";
    } else if (math_matches > 0) {
        result.domain = ReasoningDomain::MATH_ALGORITHM;
        result.domain_name = "math";
        result.reason = "命中数理公式与算法推导特征";
    } else if (code_matches > 0) {
        result.domain = ReasoningDomain::CODE_DEV;
        result.domain_name = "code";
        result.reason = "命中编程开发与代码实现特征";
    } else if (arch_matches > 0) {
        result.domain = ReasoningDomain::SYSTEM_ARCHITECTURE;
        result.domain_name = "architecture";
        result.reason = "命中系统架构与深度技术方案特征";
    } else if (result.need_deep_reasoning) {
        result.domain = ReasoningDomain::PROFESSIONAL_KNOWLEDGE;
        result.domain_name = "professional";
        result.reason = "综合多维特征达到深度推理阈值";
    } else {
        result.domain = ReasoningDomain::CHAT_CASUAL;
        result.domain_name = "chat";
        result.reason = "日常问候/轻量闲聊/情感陪伴";
    }

    return result;
}

} // namespace audio_agent
