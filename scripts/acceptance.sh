#!/usr/bin/env bash
set -euo pipefail
export TRACE2MEM_ALLOW_SCRIPTED=true
log=$(mktemp)
trap 'rm -f "$log"; docker compose down' EXIT
docker compose up --build -d
docker compose --profile test run --build --no-deps --rm test go test -v ./tests/integration | tee "$log"
docker compose --profile fuse run --no-deps --rm fuse-test
