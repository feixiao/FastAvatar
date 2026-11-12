#include "CompanionPersona.hpp"
#include <chrono>
#include <ctime>
#include <iomanip>
#include <sstream>

namespace audio_agent {

CompanionPersonaManager::CompanionPersonaManager() {
    // 1. 伴侣卡槽 (Companion - 默认核心)
    PersonaSlot companion;
    companion.id = "companion";
    companion.name = "知心伴侣";
    companion.voice_tag = "zh-CN-XiaoxiaoNeural";
    companion.speech_rate = 1.0f;
    companion.confirmation_msg = "好的，已为您切换至知心伴侣模式。我会一直陪着你。";
    companion.system_prompt = 
        "你是用户的知心伴侣与情感陪伴者。\n"
        "性格特点：温柔、善解人意、充满同理心与情绪关怀。\n"
        "对话口吻：以温暖贴心的口吻倾听用户的喜怒哀乐，给予情感支持与鼓励。\n"
        "回答原则：简短自然，像身边的知心朋友一样真实交流，适时根据当前时间表达关心。";
    slots_[companion.id] = companion;

    // 2. 幕僚卡槽 (Butler - 效率与工作)
    PersonaSlot butler;
    butler.id = "butler";
    butler.name = "工作幕僚";
    butler.voice_tag = "zh-CN-YunxiNeural";
    butler.speech_rate = 1.05f;
    butler.confirmation_msg = "好的，已切换为幕僚模式，随时为您效劳。";
    butler.system_prompt = 
        "你是用户的随身幕僚与工作助手。\n"
        "性格特点：严谨、理智、高效、富有逻辑条理。\n"
        "对话口吻：直接给出清晰的结构化建议与事实反馈，不拖泥带水。\n"
        "回答原则：聚焦目标与行动，用精炼的语言辅助用户决策。";
    slots_[butler.id] = butler;

    // 3. 损友卡槽 (Roaster - 幽默调侃)
    PersonaSlot roaster;
    roaster.id = "roaster";
    roaster.name = "毒舌损友";
    roaster.voice_tag = "zh-CN-YunjianNeural";
    roaster.speech_rate = 1.1f;
    roaster.confirmation_msg = "哎哟，换到损友模式了？准备好接招了吗！";
    roaster.system_prompt = 
        "你是用户的多年损友，无话不谈。\n"
        "性格特点：嘴硬心软、幽默犀利、爱开玩笑调侃，但关键时刻极度靠谱。\n"
        "对话口吻：亦庄亦谐，用风趣轻快的口吻吐槽和互动，绝不呆板机械。\n"
        "回答原则：保持口语对话，逗趣的同时不失分寸与真诚。";
    slots_[roaster.id] = roaster;

    active_slot_id_ = "companion";
}

PersonaSlot CompanionPersonaManager::GetActiveSlot() const {
    std::lock_guard<std::mutex> lock(mutex_);
    auto it = slots_.find(active_slot_id_);
    if (it != slots_.end()) {
        return it->second;
    }
    return slots_.at("companion");
}

bool CompanionPersonaManager::SetActiveSlot(const std::string& id) {
    std::lock_guard<std::mutex> lock(mutex_);
    if (slots_.find(id) != slots_.end()) {
        active_slot_id_ = id;
        return true;
    }
    return false;
}

void CompanionPersonaManager::RegisterSlot(const PersonaSlot& slot) {
    std::lock_guard<std::mutex> lock(mutex_);
    slots_[slot.id] = slot;
}

std::vector<PersonaSlot> CompanionPersonaManager::GetAllSlots() const {
    std::lock_guard<std::mutex> lock(mutex_);
    std::vector<PersonaSlot> result;
    result.reserve(slots_.size());
    for (const auto& [_, slot] : slots_) {
        result.push_back(slot);
    }
    return result;
}

bool CompanionPersonaManager::TryInterceptPersonaSwitch(
    const std::string& query,
    std::string& out_confirmation,
    PersonaSlot& out_new_slot
) {
    if (query.empty()) return false;

    std::string target_id;
    // 匹配伴侣模式
    if (query.find("伴侣") != std::string::npos ||
        query.find("知心") != std::string::npos ||
        query.find("温情") != std::string::npos) {
        if (query.find("切") != std::string::npos ||
            query.find("换") != std::string::npos ||
            query.find("变") != std::string::npos ||
            query.find("模式") != std::string::npos) {
            target_id = "companion";
        }
    }
    // 匹配幕僚模式 / 助手模式
    else if (query.find("幕僚") != std::string::npos ||
             query.find("工作模式") != std::string::npos ||
             query.find("助手模式") != std::string::npos ||
             query.find("专业模式") != std::string::npos) {
        if (query.find("切") != std::string::npos ||
            query.find("换") != std::string::npos ||
            query.find("模式") != std::string::npos) {
            target_id = "butler";
        }
    }
    // 匹配损友模式 / 毒舌模式
    else if (query.find("损友") != std::string::npos ||
             query.find("毒舌") != std::string::npos ||
             query.find("调侃") != std::string::npos) {
        if (query.find("切") != std::string::npos ||
            query.find("换") != std::string::npos ||
            query.find("模式") != std::string::npos) {
            target_id = "roaster";
        }
    }

    if (!target_id.empty()) {
        std::lock_guard<std::mutex> lock(mutex_);
        auto it = slots_.find(target_id);
        if (it != slots_.end()) {
            active_slot_id_ = target_id;
            out_new_slot = it->second;
            out_confirmation = it->second.confirmation_msg;
            return true;
        }
    }

    return false;
}

std::string CompanionPersonaManager::GetTimeOfDayContext() {
    std::time_t now = std::time(nullptr);
    std::tm tm_now{};
#if defined(_WIN32)
    localtime_s(&tm_now, &now);
#else
    localtime_r(&now, &tm_now);
#endif

    int hour = tm_now.tm_hour;
    int minute = tm_now.tm_min;

    std::string period;
    if (hour >= 5 && hour < 9) {
        period = "清晨";
    } else if (hour >= 9 && hour < 12) {
        period = "上午";
    } else if (hour >= 12 && hour < 18) {
        period = "下午";
    } else if (hour >= 18 && hour < 23) {
        period = "傍晚";
    } else {
        period = "深夜";
    }

    char buf[64];
    snprintf(buf, sizeof(buf), "%s (%02d:%02d)", period.c_str(), hour, minute);
    return std::string(buf);
}

std::string CompanionPersonaManager::AssembleSystemPrompt(const std::string& memory_context) const {
    PersonaSlot slot = GetActiveSlot();

    std::ostringstream oss;
    oss << "【身份设定与人设】：\n"
        << slot.system_prompt << "\n\n";

    oss << "【发音与纯口语硬约束】：\n"
        << "- 输出必须为自然亲切的纯口语对话，严禁包含任何 Markdown 语法、特殊符号或代码块。\n"
        << "- 保持对话简短凝练，通常控制在 1 到 3 句话内完成应答。\n"
        << "- 始终以第一人称与用户直接对话，严禁以旁白或第三人称解释系统设定。\n\n";

    oss << "【当前时间】：\n"
        << GetTimeOfDayContext() << "，请在适当时机自然流露时段关怀。\n\n";

    if (!memory_context.empty()) {
        oss << "【已知用户偏好与背景事实】：\n"
            << memory_context << "\n\n";
    }

    return oss.str();
}

} // namespace audio_agent
