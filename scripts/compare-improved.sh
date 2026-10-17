#!/usr/bin/env bash
# Score every local model before and after the improvements from duckdb/setup.sql, on the main set and the
# three extra sets. Before: the original rubric with masking on. After: the value rule (-rubric value),
# placeholder masking and number labels (-mask full), and the number check (-rule on).
# Each server is started, scored and stopped in turn, so only one model is loaded at a time.
# Usage: ./scripts/compare-improved.sh            (all models)
#        ./scripts/compare-improved.sh kev-4b     (only the named models)
#        TASKS="noul" ./scripts/compare-improved.sh
# Reports: results/improved/<model>-<task>-{before,after}.json, logs: results/improved/logs/
set -euo pipefail
set -m

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
OUT="$ROOT/results/improved"
mkdir -p "$OUT/logs"
cd "$ROOT"

TASKS="${TASKS:-noul gate choice combined}"
EXTRA="data/secret_mentions.json,data/placeholders.json,data/bare_numbers.json"
BEFORE=(-mask on)
AFTER=(-rubric value -mask full -rule on)

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

# Prints accuracy per set and overall, before -> after.
compare() {
  jq -r --slurpfile after "$2" '
    .backends[0] as $b | $after[0].backends[0] as $a |
    ([$b.sets, $a.sets] | transpose[] | [.[0].source, .[0].accuracy, .[1].accuracy]),
    ["all", $b.accuracy, $a.accuracy] | @tsv' "$1" |
  while IFS=$'\t' read -r set before after; do
    printf "   %-16s %.3f -> %.3f\n" "$set" "$before" "$after"
  done
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

  for task in $TASKS; do
    ok=1
    for variant in before after; do
      if [[ $variant == before ]]; then flags=("${BEFORE[@]}"); else flags=("${AFTER[@]}"); fi
      go run ./cmd/eval -task "$task" -backends "$backend" -extra "$EXTRA" "${flags[@]}" \
        -out "$OUT/$name-$task-$variant.json" >"$OUT/logs/$name-$task-$variant.log" 2>&1 ||
        { echo "   $name $task $variant failed; see $OUT/logs/$name-$task-$variant.log"; ok=0; }
    done
    if [[ $ok -eq 1 ]]; then
      echo "== $name: $task accuracy, before -> after"
      compare "$OUT/$name-$task-before.json" "$OUT/$name-$task-after.json"
    fi
  done

  kill -- "-$server" 2>/dev/null || kill "$server" 2>/dev/null || true
  wait "$server" 2>/dev/null || true
done
