#!/usr/bin/env bash
set -euo pipefail
: "${OPENROUTER_API_KEY:?Set OPENROUTER_API_KEY}"
: "${TRACE2MEM_MODEL_CONFIG:?Set path to explicit model YAML}"
: "${GOOGLE_APPLICATION_CREDENTIALS:?Set ADC path for the Vertex embedding profile}"
report_dir="${TRACE2MEM_REPORT_DIR:-.local/harbor-$(date +%s)}"
run_name="trace2mem-agent-$$"
db_name="$run_name-db"
mkdir -p "$report_dir"
cleanup() {
  docker rm -f "$run_name" >/dev/null 2>&1 || true
  docker rm -fv "$db_name" >/dev/null 2>&1 || true
}
trap cleanup EXIT INT TERM
docker build --target build -t trace2mem-agent:local .
docker run --name "$db_name" -d -e POSTGRES_PASSWORD=local-evaluation -e POSTGRES_DB=trace2mem pgvector/pgvector:pg17
for attempt in {1..30}; do
  if docker exec "$db_name" pg_isready -U postgres >/dev/null; then break; fi
  sleep 1
done
docker create --name "$run_name" --network "container:$db_name" \
  -e OPENROUTER_API_KEY -e GOOGLE_APPLICATION_CREDENTIALS=/tmp/adc.json \
  -e TRACE2MEM_LIVE_DATABASE='postgres://postgres:local-evaluation@127.0.0.1:5432/trace2mem?sslmode=disable' \
  -e TRACE2MEM_AGENT_REPLAY="${TRACE2MEM_AGENT_REPLAY:+/tmp/replay.jsonl}" -e TRACE2MEM_LIVE_CONFIG=/tmp/models.yaml -e TRACE2MEM_LIVE_REPORT_DIR=/tmp/report \
  trace2mem-agent:local go test -v ./tests/live -run TestKimiProjectAgent -count=1 -timeout=26m
docker cp "$GOOGLE_APPLICATION_CREDENTIALS" "$run_name:/tmp/adc.json"
docker cp "$TRACE2MEM_MODEL_CONFIG" "$run_name:/tmp/models.yaml"
if [ -n "${TRACE2MEM_AGENT_REPLAY:-}" ]; then
  docker cp "$TRACE2MEM_AGENT_REPLAY" "$run_name:/tmp/replay.jsonl"
fi
run_status=0
docker start -a "$run_name" || run_status=$?
docker cp "$run_name:/tmp/report/." "$report_dir/" || run_status=1
exit "$run_status"
