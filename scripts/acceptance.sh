#!/usr/bin/env bash
set -euo pipefail
export TRACE2MEM_ALLOW_SCRIPTED=true
log=$(mktemp)
trap 'rm -f "$log"; docker compose down' EXIT
docker compose up --build -d
docker compose --profile test run --build --no-deps --rm test go test -v ./tests/integration | tee "$log"
TRACE2MEM_SPACE=$(sed -n 's/.*space=\([0-9a-f]*\) verified.*/\1/p' "$log" | tail -n 1)
export TRACE2MEM_SPACE
: "${TRACE2MEM_SPACE:?integration test did not return a compiled space}"
docker compose --profile fuse run --no-deps --rm fuse-test
