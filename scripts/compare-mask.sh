#!/usr/bin/env bash
# Score every local model alone and with masking, on the yes/no and gate tasks.
# Each server is started, scored and stopped in turn, so only one model is loaded at a time.
# Usage: ./scripts/compare-mask.sh            (all models)
#        ./scripts/compare-mask.sh decider-4b (only the named models)
# Reports: results/mask/<model>-<task>.json, logs: results/mask/logs/
set -euo pipefail
set -m

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/results/mask"
mkdir -p "$OUT/logs"
cd "$ROOT"

# name | environment | serve script | backend | health URL | text the ready health response contains
MODELS=(
  "kev-4b|HF_HUB_OFFLINE=1 KEV_RUN=jaredpalmer/kev-4b|serve-kev.sh|kev|http://127.0.0.1:8009/v1/models|models"
  "kev-0.8b|HF_HUB_OFFLINE=1 KEV_RUN=jaredpalmer/kev-0.8b|serve-kev.sh|kev|http://127.0.0.1:8009/v1/models|models"
  "laya-multilingual|HF_HUB_OFFLINE=1|serve-laya.sh|laya|http://127.0.0.1:8010/health|"
  "strands-decider-2b|HF_HUB_OFFLINE=1|serve-strands.sh|strands|http://127.0.0.1:8011/health|ok"
  "gliner2.5-decide|HF_HUB_OFFLINE=1|serve-gliner.sh|gliner|http://127.0.0.1:8012/health|ok"
  "gliner2.5-multi-decide|HF_HUB_OFFLINE=1 GLINER_RUN=fastino/GLiNER2.5-multi-Decide|serve-gliner.sh|gliner|http://127.0.0.1:8012/health|ok"
  "decider-4b|HF_HUB_OFFLINE=1|serve-decider.sh|decider|http://127.0.0.1:8013/health|\"ok\":true"
)

wanted() {
  local name=$1; shift
  [[ $# -eq 0 ]] && return 0
  for w in "$@"; do [[ $w == "$name" ]] && return 0; done
  return 1
}

for row in "${MODELS[@]}"; do
  IFS='|' read -r name envs script backend health ready <<<"$row"
  wanted "$name" "$@" || continue

  echo "== $name: starting server"
  env $envs "./scripts/$script" >"$OUT/logs/$name-server.log" 2>&1 &
  server=$!

  up=0
  for _ in $(seq 1 300); do
    if body=$(curl -fs --max-time 2 "$health" 2>/dev/null) && [[ $body == *"$ready"* ]]; then up=1; break; fi
    if ! kill -0 "$server" 2>/dev/null; then
      echo "   $name server exited; see $OUT/logs/$name-server.log"
      continue 2
    fi
    sleep 2
  done
  if [[ $up -eq 0 ]]; then
    echo "   $name server not ready after 10 minutes; see $OUT/logs/$name-server.log"
    kill -- "-$server" 2>/dev/null || kill "$server" 2>/dev/null || true
    wait "$server" 2>/dev/null || true
    continue
  fi

  for task in noul gate; do
    echo "== $name: $task"
    go run ./cmd/eval -task "$task" -backends "$backend" -mask both -out "$OUT/$name-$task.json" \
      >"$OUT/logs/$name-$task.log" 2>&1 || echo "   $name $task failed; see $OUT/logs/$name-$task.log"
    sed -n '/model alone -> model + masking/,$p' "$OUT/logs/$name-$task.log"
  done

  kill -- "-$server" 2>/dev/null || kill "$server" 2>/dev/null || true
  wait "$server" 2>/dev/null || true
done
