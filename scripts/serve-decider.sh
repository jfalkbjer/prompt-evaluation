#!/usr/bin/env bash
# Serve Mapika's Decider-4B for the PII eval.
# Usage: ./scripts/serve-decider.sh
# Then:  go run ./cmd/eval -backends decider
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$ROOT/.cache/decider"
PORT="${DECIDER_PORT:-8013}"

mkdir -p "$CACHE"
cd "$CACHE"

if [[ ! -x .venv/bin/python ]]; then
  uv venv --python 3.13
fi
uv pip install --python .venv/bin/python "decider-ai[serve,metal]"

export PYTHONUNBUFFERED=1
export HF_HUB_DISABLE_XET=1
export DECIDER_MODEL="${DECIDER_RUN:-Mapika/decider-4b}"
export DECIDER_DEVICE="${DECIDER_DEVICE:-mps}"

exec .venv/bin/uvicorn decider.serve:app --host 127.0.0.1 --port "$PORT"
