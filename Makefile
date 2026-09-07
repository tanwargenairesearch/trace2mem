GO ?= go
export GOCACHE ?= /tmp/brain-go-cache
.PHONY: build generate setup dev-up test test-integration test-e2e test-fuse test-model-local test-model-openai terraform-check
build:
	mkdir -p bin
	$(GO) build -o bin/brain-server ./cmd/brain-server
	$(GO) build -o bin/brain-worker ./cmd/brain-worker
	$(GO) build -o bin/brainctl ./cmd/brainctl
generate:
	buf lint
	buf generate
setup:
	docker compose run --build --rm init

bootstrap-token:
	docker compose run --no-deps --rm --entrypoint cat init /data/secrets/.local/bootstrap-token
dev-up:
	docker compose up --build -d
test:
	$(GO) test -race ./...
test-integration: test-e2e
test-e2e:
	BRAIN_ALLOW_SCRIPTED=true docker compose up --build -d
	docker compose --profile test run --build --rm test go test -v ./tests/integration
test-fuse:
	docker compose --profile fuse run --rm fuse-test
test-model-local:
	BRAIN_LIVE_PROVIDER=ollama $(GO) test -v ./tests/live
test-model-openai:
	BRAIN_LIVE_PROVIDER=openai $(GO) test -v ./tests/live
terraform-check:
	terraform fmt -check -recursive infra
	terraform -chdir=infra/bootstrap init -backend=false
	terraform -chdir=infra/bootstrap validate
	terraform -chdir=infra/gcp init -backend=false
	terraform -chdir=infra/gcp validate

test-model:
	$(GO) test -v ./tests/live

test-release:
	scripts/acceptance.sh

vuln:
	$(GO) run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
