#!/usr/bin/env bash
# Serve GLiNER2.5-Decide for the PII eval.
# Usage: ./scripts/serve-gliner.sh
# Then:  go run ./cmd/eval -backends gliner
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$ROOT/.cache/gliner"

mkdir -p "$CACHE"
cd "$CACHE"

if [[ ! -x .venv/bin/python ]]; then
  uv venv --python 3.13
fi
uv pip install --python .venv/bin/python "gliner2[local]" fastapi uvicorn

export PYTHONUNBUFFERED=1
export HF_HUB_DISABLE_XET=1
export GLINER_PORT="${GLINER_PORT:-8012}"
export GLINER_RUN="${GLINER_RUN:-fastino/GLiNER2.5-Decide}"
export GLINER_DEVICE="${GLINER_DEVICE:-mps}"

exec .venv/bin/python "$ROOT/scripts/gliner_server.py"
