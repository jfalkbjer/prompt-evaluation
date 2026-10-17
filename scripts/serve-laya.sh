#!/usr/bin/env bash
# Serve Laya's multilingual checkpoint for the PII eval.
# Usage: ./scripts/serve-laya.sh
# Then:  go run ./cmd/eval -backends laya
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$ROOT/.cache/laya"
PORT="${LAYA_PORT:-8010}"

mkdir -p "$CACHE"
cd "$CACHE"

if [[ ! -x .venv/bin/python ]]; then
  uv venv --python 3.13
fi
uv pip install --python .venv/bin/python "laya[serve]"

export PYTHONUNBUFFERED=1
export HF_HUB_DISABLE_XET=1
export LAYA_HOST=127.0.0.1
export LAYA_PORT="$PORT"
export LAYA_DEVICE="${LAYA_DEVICE:-mps}"
export LAYA_PRELOAD=1
export LAYA_MODELS="${LAYA_MODELS:-multilingual}"
export LAYA_DEFAULT_MODEL="${LAYA_DEFAULT_MODEL:-multilingual}"

exec .venv/bin/laya-serve
