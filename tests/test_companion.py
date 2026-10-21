"""Unit tests for the companion cultivation engine (Phase 1 & Phase 2)."""

import pytest
from pathlib import Path
from src.companion import CompanionManager, LEVEL_CONFIG


def test_companion_initialization(tmp_path: Path):
    data_file = tmp_path / "companion.json"
    mgr = CompanionManager(data_file=data_file)
    assert data_file.exists()
    assert mgr.data.avatar_growth.level == 1
    assert mgr.data.avatar_growth.name == "小夏"
    assert len(mgr.data.user_profile.facts) >= 2

    summary = mgr.get_summary()
    assert summary["level"] == 1
    assert summary["affinity_stage"] == "初识助手"
    assert summary["theme"] == "theme-lv1"


def test_xp_and_level_up(tmp_path: Path):
    data_file = tmp_path / "companion.json"
    mgr = CompanionManager(data_file=data_file)
    
    # 1. Voice turn adds XP
    res = mgr.add_turn_xp(is_voice=True)
    # First turn of the day gets bonus: 15 base + 30 bonus = 45 XP
    assert res["earned_xp"] >= 45
    assert mgr.data.avatar_growth.total_turns == 1

    # 2. Add enough XP to level up to Lv.2
    mgr.data.avatar_growth.current_xp = 90
    res2 = mgr.add_turn_xp(is_voice=False) # 90 + 10 = 100 -> level 2
    assert res2["level"] == 2
    assert res2["leveled_up"] is True
    assert res2["affinity_stage"] == "熟络朋友"
    assert res2["theme"] == "theme-lv2"


def test_system_prompt_generation(tmp_path: Path):
    data_file = tmp_path / "companion.json"
    mgr = CompanionManager(data_file=data_file)
    prompt = mgr.build_system_prompt()
    assert "小夏" in prompt
    assert "初识助手" in prompt
    assert "咖啡" in prompt  # starter memory included
    assert "布丁" in prompt


def test_memory_management(tmp_path: Path):
    data_file = tmp_path / "companion.json"
    mgr = CompanionManager(data_file=data_file)
    
    fact = mgr.add_fact("喜欢在下雨天听爵士乐", category="preference")
    assert fact.content == "喜欢在下雨天听爵士乐"
    
    # Deduplication
    fact2 = mgr.add_fact("喜欢在下雨天听爵士乐", category="preference")
    assert fact2.id == fact.id
    
    # Removal
    removed = mgr.remove_fact(fact.id)
    assert removed is True
    assert not any(f.id == fact.id for f in mgr.data.user_profile.facts)


@pytest.mark.asyncio
async def test_memory_extraction(tmp_path: Path):
    data_file = tmp_path / "companion.json"
    mgr = CompanionManager(data_file=data_file)
    
    extracted = await mgr.extract_memory_facts_async("我平时超爱喝茉莉绿茶！", "哇，茉莉花茶香气很清雅呢！")
    assert len(extracted) > 0
    assert any("茉莉绿茶" in f.content for f in mgr.data.user_profile.facts)

    # Name extraction
    extracted2 = await mgr.extract_memory_facts_async("你可以叫我阿哲", "好的阿哲！")
    assert mgr.data.user_profile.name == "阿哲"
