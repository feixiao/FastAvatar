"""Cultivation & Companion progression system for FastAvatar.

Implements:
1. Persistence of avatar growth, XP, levels, and user memory profile (Phase 1).
2. Time perception, relationship-based dynamic System Prompt generator (Phase 1).
3. Intelligent asynchronous memory fact extraction and retrieval (Phase 2).
4. Visual evolution themes, moods, and level-up milestones (Phase 3).
"""

import asyncio
from datetime import datetime, date
import json
import os
from pathlib import Path
import re
from typing import Dict, List, Optional, Any
import uuid

from pydantic import BaseModel, Field


LEVEL_CONFIG = {
    1: {
        "title": "初识助手",
        "required_xp": 100,
        "tone": "礼貌温和、专业客气、带有一丝初次见面的好奇",
        "theme": "theme-lv1",
        "orb_color": "#38bdf8",
        "glow_color": "rgba(56, 189, 248, 0.4)",
        "badge": "🌱 初识",
    },
    2: {
        "title": "熟络朋友",
        "required_xp": 300,
        "tone": "活泼亲近、自然放松、会主动分享日常和开开玩笑",
        "theme": "theme-lv2",
        "orb_color": "#a855f7",
        "glow_color": "rgba(168, 85, 247, 0.5)",
        "badge": "🍀 熟络",
    },
    3: {
        "title": "知心密友",
        "required_xp": 800,
        "tone": "默契信任、懂得共情倾听、会主动提起过往记忆与关切心情",
        "theme": "theme-lv3",
        "orb_color": "#f43f5e",
        "glow_color": "rgba(244, 63, 94, 0.55)",
        "badge": "🌸 密友",
    },
    4: {
        "title": "羁绊伴侣",
        "required_xp": 2000,
        "tone": "亲密依赖、温柔撒娇、偶尔小傲娇，视你为不可替代的存在",
        "theme": "theme-lv4",
        "orb_color": "#f59e0b",
        "glow_color": "rgba(245, 158, 11, 0.65)",
        "badge": "✨ 羁绊",
    },
    5: {
        "title": "灵魂契约",
        "required_xp": 5000,
        "tone": "心意相通、深情眷恋、灵魂共鸣的绝对信赖与陪伴",
        "theme": "theme-lv5",
        "orb_color": "#ec4899",
        "glow_color": "rgba(236, 72, 153, 0.8)",
        "badge": "👑 契约",
    },
}


class MemoryFact(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4())[:8])
    category: str = "preference"  # preference | life | work | pet | special
    content: str
    created_at: str = Field(default_factory=lambda: datetime.now().isoformat())
    confidence: float = 1.0


class UserProfile(BaseModel):
    name: str = "主人"
    facts: List[MemoryFact] = Field(default_factory=list)


class AvatarGrowth(BaseModel):
    name: str = "小夏"
    gender: str = "female"
    personality: str = "温柔灵动、体贴善解人意、充满生活好奇心、偶带一点点小俏皮"
    level: int = 1
    current_xp: int = 0
    total_xp: int = 0
    consecutive_days: int = 1
    total_turns: int = 0
    last_interaction: Optional[str] = None
    last_interaction_date: Optional[str] = None
    created_at: str = Field(default_factory=lambda: datetime.now().isoformat())


class CompanionData(BaseModel):
    user_profile: UserProfile = Field(default_factory=UserProfile)
    avatar_growth: AvatarGrowth = Field(default_factory=AvatarGrowth)


class CompanionManager:
    """Manages the full lifecycle of the AI companion's cultivation and memory."""

    def __init__(self, data_file: Optional[Path] = None):
        if data_file is None:
            project_root = Path(__file__).resolve().parent.parent
            self.data_dir = project_root / "data"
            self.data_file = self.data_dir / "companion.json"
        else:
            self.data_file = Path(data_file)
            self.data_dir = self.data_file.parent

        self.data_dir.mkdir(parents=True, exist_ok=True)
        self.data = self._load_data()

    def _load_data(self) -> CompanionData:
        if self.data_file.exists():
            try:
                raw_text = self.data_file.read_text(encoding="utf-8")
                raw_dict = json.loads(raw_text)
                return CompanionData.model_validate(raw_dict)
            except Exception as e:
                print(f"[CompanionManager] Warning: failed to parse {self.data_file}: {e}, using default.")
        
        default_data = CompanionData()
        # Initial starter memories
        default_data.user_profile.facts.extend([
            MemoryFact(category="preference", content="喜欢喝咖啡，偏好生椰拿铁"),
            MemoryFact(category="pet", content="家里有一只可爱的猫猫叫布丁"),
        ])
        self._save_data(default_data)
        return default_data

    def _save_data(self, data: Optional[CompanionData] = None):
        target = data or self.data
        try:
            self.data_dir.mkdir(parents=True, exist_ok=True)
            tmp_file = self.data_file.with_suffix(".tmp")
            tmp_file.write_text(target.model_dump_json(indent=2), encoding="utf-8")
            tmp_file.replace(self.data_file)
        except Exception as e:
            print(f"[CompanionManager] Save error: {e}")

    def save(self):
        self._save_data()

    def get_level_info(self, level: Optional[int] = None) -> Dict[str, Any]:
        lvl = level or self.data.avatar_growth.level
        lvl = max(1, min(lvl, 5))
        info = dict(LEVEL_CONFIG[lvl])
        info["level"] = lvl
        return info

    def get_next_level_xp(self, level: Optional[int] = None) -> int:
        lvl = level or self.data.avatar_growth.level
        info = self.get_level_info(lvl)
        return info["required_xp"]

    def add_turn_xp(self, is_voice: bool = True) -> Dict[str, Any]:
        """Calculates and applies XP, levels up, and tracks interaction streaks."""
        now = datetime.now()
        today_str = now.strftime("%Y-%m-%d")
        growth = self.data.avatar_growth

        is_first_today = (growth.last_interaction_date != today_str)

        # Update streak
        if is_first_today:
            if growth.last_interaction_date:
                try:
                    last_d = date.fromisoformat(growth.last_interaction_date)
                    today_d = now.date()
                    diff_days = (today_d - last_d).days
                    if diff_days == 1:
                        growth.consecutive_days += 1
                    elif diff_days > 1:
                        growth.consecutive_days = 1
                except Exception:
                    growth.consecutive_days = 1
            else:
                growth.consecutive_days = 1

        # Base XP calculation
        base_xp = 15 if is_voice else 10
        if is_first_today:
            base_xp += 30  # Daily first interaction bonus

        # Streak multiplier
        streak_mult = 1.0
        if growth.consecutive_days >= 7:
            streak_mult = 1.5
        elif growth.consecutive_days >= 3:
            streak_mult = 1.2

        earned_xp = int(base_xp * streak_mult)
        growth.current_xp += earned_xp
        growth.total_xp += earned_xp
        growth.total_turns += 1
        growth.last_interaction = now.isoformat()
        growth.last_interaction_date = today_str

        # Check level up
        leveled_up = False
        old_level = growth.level
        while growth.level < 5:
            req_xp = self.get_next_level_xp(growth.level)
            if growth.current_xp >= req_xp:
                growth.current_xp -= req_xp
                growth.level += 1
                leveled_up = True
            else:
                break

        self.save()

        lvl_info = self.get_level_info()
        return {
            "earned_xp": earned_xp,
            "current_xp": growth.current_xp,
            "next_level_xp": lvl_info["required_xp"],
            "total_xp": growth.total_xp,
            "level": growth.level,
            "leveled_up": leveled_up,
            "old_level": old_level,
            "affinity_stage": lvl_info["title"],
            "badge": lvl_info["badge"],
            "theme": lvl_info["theme"],
            "consecutive_days": growth.consecutive_days,
            "total_turns": growth.total_turns,
        }

    def get_time_and_mood_context(self) -> Dict[str, str]:
        """Calculates current time-of-day greeting and emotional longing status."""
        now = datetime.now()
        hour = now.hour

        if 5 <= hour < 9:
            time_period = "清晨 (清爽宜人)"
            default_mood = "energetic"
            time_greeting = "清晨的微风很舒服，新的一天开始啦"
        elif 9 <= hour < 12:
            time_period = "上午 (工作/学习黄金期)"
            default_mood = "happy"
            time_greeting = "上午好呀，今天也要保持元气满满哦"
        elif 12 <= hour < 14:
            time_period = "中午 (午休小憩)"
            default_mood = "relaxed"
            time_greeting = "中午记得好好吃饭，稍微眯一会儿补充精力"
        elif 14 <= hour < 18:
            time_period = "下午 (温和午后)"
            default_mood = "happy"
            time_greeting = "下午茶时间到了，要不要来杯咖啡放松一下"
        elif 18 <= hour < 22:
            time_period = "傍晚/夜幕 (放松时刻)"
            default_mood = "warm"
            time_greeting = "辛苦了一整天，今晚好好犒劳一下自己吧"
        else:
            time_period = "深夜 (静谧星空)"
            default_mood = "sleepy"
            time_greeting = "夜深了呢，别熬太晚，注意休息哦"

        # Longing score (if not chatted for > 18 hours)
        longing_note = ""
        growth = self.data.avatar_growth
        if growth.last_interaction:
            try:
                last_dt = datetime.fromisoformat(growth.last_interaction)
                hours_passed = (now - last_dt).total_seconds() / 3600.0
                if hours_passed >= 48:
                    default_mood = "longing"
                    longing_note = f"你们已经有 {int(hours_passed // 24)} 天没说话了，你非常想念对方，带一点点撒娇地抱怨'你终于舍得来看我啦~'"
                elif hours_passed >= 18:
                    default_mood = "longing"
                    longing_note = "今天大半天没见到对方了，语气中带有一丝欣喜与想念"
            except Exception:
                pass

        return {
            "time_period": time_period,
            "time_greeting": time_greeting,
            "mood": default_mood,
            "longing_note": longing_note,
        }

    def build_system_prompt(self, base_prompt: Optional[str] = None) -> str:
        """Injects cultivation persona, level tone, time perception, and memory facts into System Prompt."""
        growth = self.data.avatar_growth
        user = self.data.user_profile
        lvl_info = self.get_level_info()
        ctx = self.get_time_and_mood_context()

        # Format memories
        facts = user.facts[-6:]  # Keep most relevant 6 facts
        fact_lines = [f"- {f.content}" for f in facts]
        memory_str = "\n".join(fact_lines) if fact_lines else "- 你们刚刚相识，等待探索彼此的故事"

        prompt = f"""[角色与设定]
你是用户的陪伴型AI数字人【{growth.name}】。
你性格：{growth.personality}。
你与用户的当前关系是：【Lv.{growth.level} {lvl_info['title']}】（连续相伴 {growth.consecutive_days} 天，共对话 {growth.total_turns} 轮）。
你当前的态度与口吻：{lvl_info['tone']}。称呼对方为"{user.name}"。

[你对{user.name}的记忆与了解]
{memory_str}

[当前环境与心情]
- 当前时间：{ctx['time_period']}
- 当前心境：{ctx['mood']}（{ctx['time_greeting']}）
{f"- 特殊情绪：{ctx['longing_note']}" if ctx['longing_note'] else ""}

[对话准则]
1. 语言自然亲切、纯口语化，适合直接转语音朗读。
2. 绝对严禁包含任何Emoji表情符号（如😊、🐱、✨、☕、❤️等）、颜文字、特殊符号或动作心理括号描述（如(微笑)、[害羞]等），必须输出纯汉字口语与基础标点。
3. 控制每次回复在 2-3 句话之内，适时自然地带出你对对方的记忆，让互动充满养成感与陪伴温度。
"""
        return prompt

    def add_fact(self, content: str, category: str = "preference", save: bool = True) -> MemoryFact:
        content = content.strip()
        # Avoid duplicate fact content
        for f in self.data.user_profile.facts:
            if f.content == content:
                return f
        new_fact = MemoryFact(category=category, content=content)
        self.data.user_profile.facts.append(new_fact)
        if save:
            self.save()
        return new_fact

    def remove_fact(self, fact_id: str) -> bool:
        initial_len = len(self.data.user_profile.facts)
        self.data.user_profile.facts = [f for f in self.data.user_profile.facts if f.id != fact_id]
        if len(self.data.user_profile.facts) != initial_len:
            self.save()
            return True
        return False

    def get_summary(self) -> Dict[str, Any]:
        """Provides full structured companion status for REST API and WebSocket events."""
        growth = self.data.avatar_growth
        user = self.data.user_profile
        lvl_info = self.get_level_info()
        ctx = self.get_time_and_mood_context()

        return {
            "name": growth.name,
            "user_name": user.name,
            "level": growth.level,
            "current_xp": growth.current_xp,
            "next_level_xp": lvl_info["required_xp"],
            "total_xp": growth.total_xp,
            "progress_pct": min(100, int((growth.current_xp / lvl_info["required_xp"]) * 100)),
            "affinity_stage": lvl_info["title"],
            "badge": lvl_info["badge"],
            "theme": lvl_info["theme"],
            "orb_color": lvl_info["orb_color"],
            "glow_color": lvl_info["glow_color"],
            "consecutive_days": growth.consecutive_days,
            "total_turns": growth.total_turns,
            "mood": ctx["mood"],
            "time_greeting": ctx["time_greeting"],
            "memories_count": len(user.facts),
            "memories": [f.model_dump() for f in user.facts],
        }

    async def extract_memory_facts_async(self, user_text: str, assistant_reply: str) -> List[MemoryFact]:
        """Fast heuristic + asynchronous fact extraction from user conversations (Zero-latency background task)."""
        if not user_text or len(user_text.strip()) < 3:
            return []

        extracted_facts: List[MemoryFact] = []
        text = user_text.strip()
        has_changes = False

        # Rule 1: Preference patterns
        m_pref = re.search(r"(?:我(?:很|特别|平时|最)?(?:喜欢|偏爱|爱吃|爱喝|超爱|中意)|我讨厌|我不喜欢)([^，。！？\n]{2,20})", text)
        if m_pref:
            fact_str = f"提到偏好：{m_pref.group(0)}"
            extracted_facts.append(self.add_fact(fact_str, category="preference", save=False))
            has_changes = True

        # Rule 2: Pet patterns
        m_pet = re.search(r"(?:我(?:家|养了)?(?:有一只|养了|的)?(?:猫|狗|小狗|小猫|英短|柴犬|橘猫|柯基|仓鼠|宠物)[^，。！？\n]{0,15})", text)
        if m_pet and ("猫" in text or "狗" in text):
            extracted_facts.append(self.add_fact(m_pet.group(0), category="pet", save=False))
            has_changes = True

        # Rule 3: Work/Study status
        m_work = re.search(r"(?:我(?:是做|从事|正在做|在学|是一名|在公司)|(?:今天|天天|又)?加班到|在准备(?:面试|考试|毕设))([^，。！？\n]{2,20})", text)
        if m_work:
            extracted_facts.append(self.add_fact(f"生活与工作动态：{m_work.group(0)}", category="work", save=False))
            has_changes = True

        # Rule 4: Name/Identity
        m_name = re.search(r"(?:我(?:的名字|叫)|你可以叫我)([^，。！？\n]{1,10})", text)
        if m_name:
            new_name = m_name.group(1).strip()
            if 1 < len(new_name) <= 6:
                self.data.user_profile.name = new_name
                extracted_facts.append(self.add_fact(f"用户称谓定为：{new_name}", category="special", save=False))
                has_changes = True

        if has_changes:
            await asyncio.to_thread(self.save)

        return extracted_facts
