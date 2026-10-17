#!/usr/bin/env bash
# Serve Strands Decider 2B for the PII eval.
# Usage: ./scripts/serve-strands.sh
# Then:  go run ./cmd/eval -backends strands
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$ROOT/.cache/strands"
PORT="${STRANDS_PORT:-8011}"
RUN="${STRANDS_RUN:-StrandsAgents/strands-decider-2B-hobson-v19}"

mkdir -p "$CACHE"
cd "$CACHE"

if [[ ! -x .venv/bin/python ]]; then
  uv venv --python 3.13
fi
uv pip install --python .venv/bin/python strands-decider

export PYTHONUNBUFFERED=1
export HF_HUB_DISABLE_XET=1

exec .venv/bin/strands-decider serve "$RUN" \
  --host 127.0.0.1 \
  --port "$PORT" \
  --device "${STRANDS_DEVICE:-mps}"
