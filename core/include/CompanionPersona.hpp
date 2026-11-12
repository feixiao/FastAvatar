#pragma once

#include <string>
#include <vector>
#include <unordered_map>
#include <mutex>

namespace audio_agent {

/**
 * @brief 语音伴侣/助手多模态人格卡槽定义
 */
struct PersonaSlot {
    std::string id;               ///< 人格唯一标识符 ("butler", "companion", "roaster")
    std::string name;             ///< 人格显示名称 ("幕僚", "知心伴侣", "毒舌损友")
    std::string system_prompt;    ///< 人格核心设定与口吻指令
    std::string voice_tag;        ///< 推荐端侧音色标识符
    float speech_rate{1.0f};      ///< 语速缩放因子 (0.8 ~ 1.5)
    std::string confirmation_msg; ///< 切换成功后的即时口语播报反馈
};

/**
 * @brief 语音伴侣人格与时间感知中枢 (Companion Persona Engine)
 * 
 * 核心能力：
 * 1. 预置三大经典人设（工作幕僚 / 知心伴侣 / 毒舌损友）并支持动态扩展；
 * 2. 0.1ms 口语即时拦截与人格流转（如“切换到伴侣模式”）；
 * 3. 24小时时段自动感知（清晨/上午/下午/傍晚/深夜）与时间关怀注入；
 * 4. 语音伴侣发音约束（纯口语、严禁 Markdown 格式、1~3句短语）。
 */
class CompanionPersonaManager {
public:
    CompanionPersonaManager();

    /// 获取当前激活的人格卡槽
    PersonaSlot GetActiveSlot() const;

    /// 通过 ID 显式切换人格 ("butler" / "companion" / "roaster")
    bool SetActiveSlot(const std::string& id);

    /// 动态注册或覆盖自定义人格卡槽
    void RegisterSlot(const PersonaSlot& slot);

    /// 获取所有已注册的人格列表
    std::vector<PersonaSlot> GetAllSlots() const;

    /// 0.1ms 快速拦截口语切换人格指令
    bool TryInterceptPersonaSwitch(
        const std::string& query,
        std::string& out_confirmation,
        PersonaSlot& out_new_slot
    );

    /// 获取当前时段上下文（如 "清晨 (07:30)"）
    static std::string GetTimeOfDayContext();

    /// 动态组装包含人格、时段、口语硬约束与历史记忆的完整 System Prompt
    std::string AssembleSystemPrompt(const std::string& memory_context = "") const;

private:
    mutable std::mutex mutex_;
    std::string active_slot_id_{"companion"};
    std::unordered_map<std::string, PersonaSlot> slots_;
};

} // namespace audio_agent
