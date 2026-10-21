#!/usr/bin/env bash
set -euo pipefail

# FastAvatar Benchmark Harness
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Ensure environment is configured
export PYTHONPATH="$SCRIPT_DIR:${PYTHONPATH:-}"

# Run automated unit tests first
uv run pytest tests/ -q > /dev/null 2>&1 || {
  echo "Unit tests failed!" >&2
  exit 1
}

# Run deterministic benchmark
uv run python benchmarks/run_benchmark.py
