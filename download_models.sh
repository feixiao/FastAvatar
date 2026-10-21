#!/usr/bin/env bash
# ==============================================================================
# FastAvatar: 一键模型下载与环境准备助手 (download_models.sh)
# ==============================================================================
set -euo pipefail

# ANSI 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
BOLD='\033[1m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"
MODELS_DIR="$SCRIPT_DIR/models"
mkdir -p "$MODELS_DIR"

log_info() {
  echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
  echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $1"
}

# 具备代理重试的下载与解压函数
download_tar_bz2() {
  local name="$1"
  local url="$2"
  local dest_dir="$3"
  local check_file="$4"

  if [ -f "$dest_dir/$check_file" ]; then
    log_success "$name 模型已就绪，跳过下载 ($dest_dir/$check_file)"
    return 0
  fi

  local archive_name
  archive_name="$(basename "$url")"
  local temp_archive="$MODELS_DIR/$archive_name"

  log_info "正在下载 $name 模型..."
  log_info "源地址: $url"

  # 首先尝试直接从 GitHub 官方源下载
  if ! curl -fL --progress-bar "$url" -o "$temp_archive"; then
    log_warn "从官方源下载失败或超时，尝试使用镜像加速节点重试..."
    local mirror_url="https://ghproxy.net/$url"
    if ! curl -fL --progress-bar "$mirror_url" -o "$temp_archive"; then
      log_error "下载 $name 失败，请检查网络连接！"
      rm -f "$temp_archive"
      return 1
    fi
  fi

  log_info "正在解压模型包至 $MODELS_DIR..."
  tar -xjf "$temp_archive" -C "$MODELS_DIR"
  rm -f "$temp_archive"

  if [ -f "$dest_dir/$check_file" ]; then
    log_success "$name 下载解压完成！目录: $dest_dir"
  else
    log_warn "$name 解压完成，但未发现预期文件: $dest_dir/$check_file"
  fi
}

install_python_deps() {
  log_info "检查 Python 依赖与 sherpa-onnx..."
  if command -v uv >/dev/null 2>&1; then
    uv pip install sherpa-onnx -q
    log_success "sherpa-onnx 依赖检查通过 (uv)"
  elif [ -n "${VIRTUAL_ENV:-}" ]; then
    pip install sherpa-onnx -q
    log_success "sherpa-onnx 依赖检查通过 (pip)"
  else
    log_warn "未检测到已激活的虚拟环境，建议使用 'uv sync' 或在虚拟环境中运行。"
  fi
}

setup_asr() {
  echo ""
  echo -e "${BOLD}=== [1/3] 离线语音识别 ASR 模型 (SenseVoice-Small) ===${NC}"
  local asr_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/asr-models/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17.tar.bz2"
  local asr_dir="$MODELS_DIR/sherpa-onnx-sense-voice-zh-en-ja-ko-yue-2024-07-17"
  download_tar_bz2 "SenseVoice-Small ASR (~200MB)" "$asr_url" "$asr_dir" "tokens.txt"
}

setup_tts() {
  echo ""
  echo -e "${BOLD}=== [2/3] 离线语音合成 TTS 模型 (Sherpa-ONNX VITS 中文) ===${NC}"
  local tts_url="https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/vits-zh-aishell3.tar.bz2"
  local tts_dir="$MODELS_DIR/vits-zh-aishell3"
  download_tar_bz2 "VITS 中文 Aishell3 TTS (~50MB)" "$tts_url" "$tts_dir" "vits-aishell3.onnx"
}

setup_llm() {
  echo ""
  echo -e "${BOLD}=== [3/3] 本地大语言模型 LLM ===${NC}"
  
  if command -v ollama >/dev/null 2>&1; then
    log_info "检测到已安装 Ollama CLI！"
    echo -e "准备拉取推荐的通义千问模型 ${GREEN}qwen2.5:7b${NC}（推荐 8G/16G 显存/统一内存）..."
    read -p "是否立即拉取该模型? [Y/n] (输入 3b 或 1.5b 可拉取更小版本): " -r llm_choice || true
    llm_choice="${llm_choice:-Y}"
    case "$llm_choice" in
      [yY][eE][sS]|[yY])
        ollama pull qwen2.5:7b
        log_success "Qwen2.5 7B 模型拉取完成！"
        ;;
      3b|3B)
        ollama pull qwen2.5:3b
        log_success "Qwen2.5 3B 模型拉取完成！"
        ;;
      1.5b|1.5B)
        ollama pull qwen2.5:1.5b
        log_success "Qwen2.5 1.5B 模型拉取完成！"
        ;;
      *)
        log_info "跳过 Ollama 模型拉取。"
        ;;
    esac
  elif command -v lms >/dev/null 2>&1; then
    log_info "检测到已安装 LM Studio CLI (lms)！"
    read -p "是否使用 lms 拉取 qwen2.5-7b-instruct? [Y/n]: " -r lms_choice || true
    lms_choice="${lms_choice:-Y}"
    if [[ "$lms_choice" =~ ^[Yy]$ ]]; then
      lms get qwen2.5-7b-instruct
      log_success "LM Studio 模型拉取完成！"
    else
      log_info "跳过 LM Studio 模型拉取。"
    fi
  else
    log_warn "未在当前系统 PATH 中检测到 Ollama 或 LM Studio。"
    echo -e "建议根据需求选择安装其一："
    echo -e "  - ${BOLD}Ollama (推荐终端极简)${NC}: https://ollama.com 或运行 ${GREEN}brew install ollama${NC}"
    echo -e "  - ${BOLD}LM Studio (推荐图形界面+高吞吐)${NC}: https://lmstudio.ai"
  fi
}

show_banner() {
  echo -e "${BLUE}"
  echo "============================================================"
  echo "          FastAvatar 一键模型下载与环境准备助手             "
  echo "============================================================"
  echo -e "${NC}"
}

print_summary() {
  echo ""
  echo -e "${GREEN}============================================================${NC}"
  echo -e "${GREEN}             🎉 模型与运行环境准备完毕！                   ${NC}"
  echo -e "${GREEN}============================================================${NC}"
  echo -e "本地模型存储目录: ${BOLD}$MODELS_DIR${NC}"
  if [ -d "$MODELS_DIR" ]; then
    ls -lh "$MODELS_DIR"
  fi
  echo ""
  echo -e "后续启动方式："
  echo -e "  1. 启动服务: ${GREEN}uv run python -m src.server${NC}"
  echo -e "  2. 浏览器访问: ${GREEN}http://localhost:8000${NC}"
  echo ""
}

# 模式分支
ACTION="${1:-}"

show_banner

case "$ACTION" in
  all|"")
    if [ -t 0 ] && [ -z "$ACTION" ]; then
      echo "请选择要执行的操作："
      echo "  1) 一键下载全部推荐模型 (ASR + TTS + 检查 LLM) [推荐]"
      echo "  2) 仅下载 SenseVoice ASR 离线语音识别模型 (~200MB)"
      echo "  3) 仅下载 VITS 中文 TTS 离线语音合成模型 (~50MB)"
      echo "  4) 仅拉取本地 LLM (Ollama / LM Studio)"
      echo "  0) 退出"
      read -p "请输入选项 [1-4, 默认 1]: " -r menu_opt || true
      menu_opt="${menu_opt:-1}"
      case "$menu_opt" in
        1)
          install_python_deps
          setup_asr
          setup_tts
          setup_llm
          print_summary
          ;;
        2)
          install_python_deps
          setup_asr
          ;;
        3)
          install_python_deps
          setup_tts
          ;;
        4)
          setup_llm
          ;;
        0)
          echo "已取消。"
          exit 0
          ;;
        *)
          log_error "无效选项: $menu_opt"
          exit 1
          ;;
      esac
    else
      # 非交互模式或指定了 all
      install_python_deps
      setup_asr
      setup_tts
      setup_llm
      print_summary
    fi
    ;;
  asr)
    install_python_deps
    setup_asr
    ;;
  tts)
    install_python_deps
    setup_tts
    ;;
  llm)
    setup_llm
    ;;
  *)
    echo "用法: $0 [all|asr|tts|llm]"
    exit 1
    ;;
esac
