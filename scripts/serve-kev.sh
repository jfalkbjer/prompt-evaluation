#!/usr/bin/env bash
# Serve a local Kev checkpoint for the PII eval.
# Usage: ./scripts/serve-kev.sh
#        KEV_RUN=jaredpalmer/kev-0.8b ./scripts/serve-kev.sh
# Then, in another terminal: go run ./cmd/eval
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
CACHE="$ROOT/.cache/kev"
PORT="${KEV_PORT:-8009}"
RUN="${KEV_RUN:-jaredpalmer/kev-4b}"

if [[ ! -d "$CACHE/.git" ]]; then
  mkdir -p "$ROOT/.cache"
  git clone --depth 1 https://github.com/jaredpalmer/kev.git "$CACHE"
fi

cd "$CACHE"
uv sync --extra serve
# huggingface.co's CDN accepts TCP here and then never answers the TLS handshake.
# hf-mirror.com serves the same public files when that happens.
export PYTHONUNBUFFERED=1
export HF_HUB_DISABLE_XET=1
if [[ "${HF_HUB_OFFLINE:-}" == 1 ]]; then
  echo "HF_HUB_OFFLINE=1: loading $RUN from the local Hugging Face cache"
elif ! curl -fsI --max-time 8 https://huggingface.co >/dev/null 2>&1; then
  echo "huggingface.co TLS did not complete; downloading weights from https://hf-mirror.com"
  export HF_ENDPOINT=https://hf-mirror.com
fi
exec uv run --extra serve python -m kev.serve --run "$RUN" --port "$PORT"
