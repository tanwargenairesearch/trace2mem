#!/usr/bin/env bash
set -euo pipefail

# Archive tracked HEAD so private configuration and uncommitted files cannot mask setup gaps.
repo=$(git rev-parse --show-toplevel)
checkout=$(mktemp -d "${TMPDIR:-/tmp}/trace2mem-checkout.XXXXXX")
export COMPOSE_PROJECT_NAME="trace2mem-check-$$"
export TRACE2MEM_POSTGRES_VOLUME="$COMPOSE_PROJECT_NAME-postgres"
export TRACE2MEM_BLOBS_VOLUME="$COMPOSE_PROJECT_NAME-blobs"
export TRACE2MEM_SECRETS_VOLUME="$COMPOSE_PROJECT_NAME-secrets"
export TRACE2MEM_EXTERNAL_VOLUMES=false
export TRACE2MEM_PORT="${TRACE2MEM_CHECK_PORT:-18788}"
export TRACE2MEM_DATABASE_URL='postgres://trace2mem:trace2mem@postgres:5432/trace2mem?sslmode=disable'
export TRACE2MEM_DB_USER=trace2mem TRACE2MEM_DB_PASSWORD=trace2mem TRACE2MEM_DB_NAME=trace2mem
export TRACE2MEM_ALLOW_SCRIPTED=true
export TRACE2MEM_MODEL_ENDPOINTS='http://ollama:11434,http://host.docker.internal:11434'
cleanup() {
  (cd "$checkout" && docker compose down --volumes) || true
  rm -rf "$checkout"
}
trap cleanup EXIT
git -C "$repo" archive HEAD | tar -x -C "$checkout"
cd "$checkout"
docker compose up --build -d
docker compose --profile test run --build --no-deps --rm test go test -v ./tests/integration ./internal/dream
docker compose --profile fuse run --no-deps --rm fuse-test
git -C "$repo" rev-parse HEAD
