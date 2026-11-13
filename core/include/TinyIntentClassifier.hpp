#pragma once

#include <string>
#include <string_view>
#include <vector>
#include <cstdint>

namespace audio_agent {

/**
 * @brief 认知复杂度推理领域
 */
enum class ReasoningDomain : uint8_t {
    CHAT_CASUAL = 0,            ///< 日常问候/轻量闲聊/情绪陪伴 (端侧小模型)
    MATH_ALGORITHM = 1,         ///< 数学公式/算法/推导计算 (云端大模型)
    CODE_DEV = 2,               ///< 编程开发/代码编写/调试排错 (云端大模型)
    SYSTEM_ARCHITECTURE = 3,    ///< 架构方案/设计模式/工程技术 (云端大模型)
    PROFESSIONAL_KNOWLEDGE = 4, ///< 专业科学/学术论文/深度解释 (云端大模型)
    CONTEXT_FOLLOWUP = 5        ///< 多轮复杂上下文追问继承 (云端大模型)
};

/**
 * @brief 微型分类器决策结果
 */
struct ClassificationResult {
    bool need_deep_reasoning{false};    ///< 是否需要云端大模型深度推理
    float cloud_probability{0.0f};      ///< P(Need_Cloud) 概率估计 [0.0f ~ 1.0f]
    ReasoningDomain domain{ReasoningDomain::CHAT_CASUAL};
    const char* domain_name{""};        ///< "chat", "math", "code", "architecture", "context_followup"
    std::string reason;                 ///< 决策可解释性原因 (供审计与遥测)
};

/**
 * @brief 生产级微型端侧意图与认知复杂度分类器 (Tiny Intent & Complexity Classifier)
 *
 * 核心指标与设计准则：
 * 1. 纳秒级纯 C++ 前置决策：CPU 耗时 < 100us，零外部重量级环境依赖；
 * 2. 多轮会话状态跟踪 (DST: Dialogue State Tracking)：结合上一轮会话路由继承状态，短指代强制继承；
 * 3. 泛化多维特征与 Sigmoid 校准：结合专业语义激活与生活闲聊柔化，杜绝字数暴力切分。
 */
class TinyIntentClassifier {
public:
    /**
     * @brief 认知复杂度多维打分与分类
     * @param query 当前识别文本
     * @param last_route 上一轮对话路由 (如 "CLOUD_CHAT", "EDGE_CHAT")
     * @param last_domain 上一轮对话领域 (如 "code", "math")
     */
    static ClassificationResult Classify(
        const std::string& query,
        const std::string& last_route = "",
        const std::string& last_domain = ""
    );
};

} // namespace audio_agent
