# Brain

A self-hosted Go service that turns agent events into cited notes and a Markdown knowledge wiki. Apache-2.0. This repository is an implementation under validation, not a production release or a claim of measured memory-quality gains.

## Local Docker

```sh
make dev-up
make bootstrap-token
```

Open http://localhost:8787 and use the printed bootstrap token. Ports bind to loopback. Set `BRAIN_PORT=18787` if 8787 is occupied. Compose initializes one persistent secrets volume shared by API and worker; do not delete it without backing up its encryption key. The PostgreSQL and blob volumes persist across restarts.

Build the CLI with `make build`. Set `BRAIN_URL=http://localhost:8787` and `BRAIN_TOKEN` to the Docker bootstrap token. `brainctl create --name example` returns a space ID. Use `bin/brainctl` when running from this checkout.

## Mandatory model configuration

No real model is selected automatically. Every space must configure both generation and embedding roles. They may use different providers and locations. Copy one of:

- `configs/models.local.example.yaml`: local Ollama generation and embeddings.
- `configs/models.gemini.example.yaml`: generation plus Gemini API-key embeddings.
- `configs/models.vertex.example.yaml`: generation plus Vertex AI embeddings using application default credentials.

Replace every placeholder with a model available to your account or installed locally. API keys are referenced by environment variable, never embedded in YAML. Then:

```sh
bin/brainctl configure --space SPACE_ID --file models.yaml
```

Configuration probes tool calling, structured tool arguments, and embeddings. Missing models, placeholders, floating `latest` names, unsupported providers, and unapproved custom endpoints are rejected. Generation currently supports OpenAI Responses and native Ollama; embeddings support OpenAI, Ollama, Gemini, and Vertex. Vertex uses IAM credentials of the running process; Terraform grants API/worker identities Vertex access. API-key and IAM embedding protocols are separate adapters.

Changing embedding provider/model/dimensions/endpoint/project/location queues a complete reindex. The current configuration remains active until the new vectors and revision publish atomically. A failed reindex keeps the previous revision usable; retry compilation after resolving the reported error. Long texts use bounded UTF-8 chunks and normalized pooling; this algorithm is versioned in the embedding identity and its retrieval quality still requires evaluation.

## Agent interfaces

`proto/brain/v1/brain.proto` defines the versioned contract. `sdk` includes a Go client, Adapter interface, and Protobuf-JSON JSONL importer. User adapter code runs outside the service.

```sh
bin/brainctl import --space SPACE_ID --file fixtures/history.jsonl
bin/brainctl status --space SPACE_ID
bin/brainctl search --space SPACE_ID --query 'project deadline'
bin/brainctl context --space SPACE_ID --query 'project deadline' --target ./working-set
bin/brainctl sync --space SPACE_ID --target ./snapshot
```

MCP endpoint: `/mcp`, using an authenticated streamable HTTP connection. Tools: `memory_index`, `memory_search`, `memory_read`, `memory_evidence`, `memory_context`, `memory_status`. Connecting MCP does not capture trajectories or automatically inject context. Use an adapter to submit events and `context` to prepare a cited working set.

The API supports Connect JSON and gRPC through the same handlers. Compilation is asynchronous: durable acceptance does not imply wiki freshness. Responses carry revision and watermark. Filesystem snapshots pin one revision; `context --target` downloads supporting files plus the index, while `sync` materializes all files.

## Verification

```sh
make test                 # unit/race tests; external profiles skip without configuration
make test-e2e             # Docker scripted provider, no cloud credentials
BRAIN_SPACE=ID make test-fuse  # Linux FUSE inside Docker, compiled space required
make terraform-check
```

The deterministic provider requires `BRAIN_ALLOW_SCRIPTED=true` and is a fixture, not an inference model. Real model tests are opt-in (`tests/live`) and require explicit generation and embedding models, endpoints, credentials, and budgets. They do not run in ordinary unit tests.

Read [architecture](docs/ARCHITECTURE.md), [operations](docs/OPERATIONS.md), and [implementation/validation status](docs/IMPLEMENTATION.md). The Go module namespace `github.com/brainmemory/brain` is a placeholder until a public repository owner is selected.

For a capability check using the same YAML: `BRAIN_LIVE_CONFIG=/absolute/path/models.yaml make test-model`. This makes real provider calls; use your intended account and budget. Docker startup does not select or download a real model.

OpenRouter users can copy `configs/models.openrouter-vertex.example.yaml`. The generation provider is `openai` because this selects the Responses protocol; the endpoint selects OpenRouter. Add `https://openrouter.ai/api/v1` to the operator's `BRAIN_MODEL_ENDPOINTS`. Vertex uses ADC in the process running the test; host ADC is not automatically available inside Docker. The sample's `us-central1` is an embedding location independent of the service deployment region. Review it for your data-location requirements.
