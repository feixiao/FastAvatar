"""Unit tests for streaming sentence splitter."""

import pytest
from src.sentence_splitter import SentenceSplitter


def test_sentence_splitter_basic():
    splitter = SentenceSplitter(min_length=5, max_length=20)
    tokens = ["你", "好", "呀", "！", "今", "天", "天", "气", "真", "好", "。"]
    results = []
    for t in tokens:
        results.extend(splitter.process_token(t))
    results.extend(splitter.flush())

    assert len(results) >= 2
    assert "你好呀！" in results[0]
    assert "今天天气真好。" in results[1]


def test_sentence_splitter_decimal_protection():
    splitter = SentenceSplitter(min_length=3, max_length=30)
    # 3.14 should not be split across period
    tokens = ["圆", "周", "率", "是", "3", ".", "1", "4", "，", "对", "吗", "？"]
    results = []
    for t in tokens:
        results.extend(splitter.process_token(t))
    results.extend(splitter.flush())

    assert any("3.14" in r for r in results)

def test_sentence_splitter_markdown_cleaning():
    splitter = SentenceSplitter(min_length=3, max_length=30)
    tokens = ["**", "小", "夏", "**", "：", "*", "微", "笑", "*", "你", "好", "呀", "！"]
    results = []
    for t in tokens:
        results.extend(splitter.process_token(t))
    results.extend(splitter.flush())

    assert len(results) >= 1
    combined = "".join(results)
    assert "**" not in combined
    assert "*" not in combined
    assert "小夏" in combined
    assert "你好呀！" in combined

def test_sentence_splitter_stage_action_filtering():
    splitter = SentenceSplitter(min_length=3, max_length=30)
    tokens = ["你", "好", "呀", "！", "（", "微", "笑", "着", "挥", "手", "）", "今", "天", "过", "得", "好", "吗", "？"]
    results = []
    for t in tokens:
        results.extend(splitter.process_token(t))
    results.extend(splitter.flush())

    combined = "".join(results)
    assert "微笑" not in combined
    assert "（" not in combined
    assert "）" not in combined
    assert "你好呀！" in combined
    assert "今天过得好吗？" in combined
