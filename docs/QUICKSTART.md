# Docker quickstart

This sequence was validated from clean commit `9c13818` with fresh volumes on 2026-09-08. `make test-fresh-checkout` repeats Docker/FUSE acceptance from tracked HEAD without loading private configuration; it removes only its uniquely named test volumes.

Requirements: Git, Docker with Compose and a running daemon. The default stack uses PostgreSQL with vectors, shared blob storage, and separate API/worker processes. Linux and Apple Silicon Docker are supported; inference acceleration inside Docker is not promised.

From a checkout:

```sh
make dev-up
make bootstrap-token
```

Open `http://localhost:8787`, authenticate with the printed token, and follow **Models → Connections or Import → Activity → Memory**. Set `TRACE2MEM_PORT` before startup if that port is occupied. Model setup is mandatory for compilation and Ask; ingestion works before setup. No provider is preselected.

For local inference, configure a reachable Ollama endpoint and explicit installed generation/embedding models. The default endpoint allowlist includes `http://host.docker.internal:11434` for host Ollama and `http://ollama:11434` for the optional container profile. For cloud inference, choose providers independently and supply credentials through environment variables or the model form. Vertex ADC must be available to the process making requests; host ADC is not automatically copied into containers.

Build the optional host CLI with `make build` using the Go version in `go.mod`. Export `TRACE2MEM_URL` and `TRACE2MEM_TOKEN` privately. Copy a YAML example from `configs/`, replace every model placeholder, and run:

```sh
bin/trace2mem configure --file models.yaml
bin/trace2mem import --file fixtures/history.jsonl
bin/trace2mem compile
bin/trace2mem status
bin/trace2mem search --query Launch
```

The web form performs the same configuration probe. A custom endpoint needs approval in the operator's `TRACE2MEM_MODEL_ENDPOINTS` before containers start. Do not commit YAML containing secret values.

Automatic compilation uses a 60-second quiet period and five-minute cap; closed sessions compile immediately. Settings supports daily and manual modes. Durable acceptance and published freshness are separate states. Read the index before asking an agent to retrieve more memory.

The bootstrap user has one memory, shared by their agents. Connections issues narrower agent tokens. Shared multi-user deployments use verified OIDC identities and correctly configured OAuth for MCP; the local bootstrap token must not be distributed to different people.

## Troubleshooting

| Symptom | Action |
|---|---|
| Ask disabled | Configure both model roles and wait for a published revision |
| Accepted events but stale memory | Inspect Activity/status and Settings; manual mode requires Compile |
| Compilation blocked | Configure missing models; replacement credentials resume blocked/failed approved work |
| Provider error | Verify model ID, credentials, capability probe, endpoint allowlist, and budget; inspect sanitized status |
| Search reports semantic fallback | Keyword search remains available; restore the embedding provider or finish reindexing |
| Import conflict | Retry the exact envelope; a correction needs a new event ID |
| Token/login error | Use `make bootstrap-token` for this installation or a valid unexpired agent token; match the configured public origin |
| Offline file read fails | That file was not cached; reconnect and preload it or sync a complete snapshot |

`docker compose stop` stops processes while retaining data. Back up the PostgreSQL, blob and secrets volumes together. Never remove an old installation's volumes to upgrade. The pre-release per-user redesign starts fresh namespaces and rejects old space-bearing clients; see [operations](OPERATIONS.md).
