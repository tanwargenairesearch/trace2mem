# Implementation and validation status

## Implemented foundation

- Apache-2.0, Go binaries, versioned Protobuf/Connect contract, JSONL adapter/importer.
- PostgreSQL durable ingestion, idempotency/conflicts, scoped identity and membership, durable leased jobs.
- Three memory layers, stable observation IDs and citations, bounded Dream tools, structural/semantic proposal verification, fenced revision publication, corrections and forgetting.
- Explicit YAML generation/embedding configuration, OpenAI/Ollama generation, Gemini/Vertex/OpenAI/Ollama embeddings, credential encryption, budget reservations, resumable staged reindex.
- Keyword/semantic retrieval with explicit semantic status, bounded context agent, six MCP tools, pinned directory export/cache and read-only FUSE.
- Basic management console, OIDC/OAuth configuration, evaluations and operator candidate promotion.
- Local Compose, Google Cloud Terraform, CI/release workflows and operations documentation.

These are implementation claims, not completion of every release acceptance criterion in the original plan.

## Tests recorded locally

2026-09-07, Apple Silicon host with Linux Docker:

- Go 1.26.6 unit/race suite: passed after dependency security updates.
- Docker HTTP/gRPC/MCP lifecycle: passed. Durable duplicate/conflict handling, citations, temporal correction, embedding-model replacement, forgetting, directory synchronization and offline warm reads.
- Database regressions: passed. Stale proposal/publication after forgetting, concurrent budget reservations, valid-token membership isolation, incremental 180-event history over the prompt limit preserving all subjects.
- Linux Docker FUSE smoke: passed. `ls`, `stat`, `cat`, recursive citation search, and write rejection. Native macOS FUSE not tested.
- Terraform bootstrap/GCP init and validation: passed. GCP mock plan test: passed. Formatting, Protobuf lint, and backward-compatibility check against e49b7c6: passed.
- Go vet: passed after final dependency update.
- Vulnerability scan found advisories in Go1.26.3 and dependencies; patched to Go1.26.6 and identified fixed module versions. Final scan: zero reachable vulnerabilities; one advisory remains in imported/required code that the scanner reports is not called.

## Three-lens review triage

Must-fix findings addressed:

- Fence proposal and candidate persistence against forgetting; lock candidate promotion before reading its content.
- Use serialized rather than compressed payload bytes for incremental bounds; retain only new records in session segments.
- Native provider tool-call/result pairing and empty-required JSON Schema correctness.
- Reserve concurrent provider budgets and mark embedding usage as estimated.
- Validate synthesized citations against inspected evidence.
- Bound cache metadata/content across revisions, preserve manifest identity, account replacements correctly, and synchronize selected context files.
- Preserve independent facts through stable observation IDs; keep inspected artifact ranges in materialized evidence.
- Stage embedding configuration, checkpoint vectors, publish by transactional SQL join, and explicitly report historical index incompatibility.
- Inject optional OIDC client secret by Secret Manager reference; use one Compose bootstrap identity.
- Close/cancel server and worker resources; remove dead declarations.
- Require Docker and Linux FUSE acceptance before release image publication.

Production and design reviewers reported zero findings on the final bounded reindex/search/artifact fixes. Principles' release-gate finding was addressed by scripts/acceptance.sh in both CI and release workflows.

Deferred / remaining release gates:

- Real Gemini/Vertex/Ollama/OpenAI capability and memory-quality evaluation needs user-selected model IDs, credentials, and project where applicable. No measured quality or cost improvement is claimed.
- Cloud smoke deployment, IAM/private SQL validation, actual backup restoration, and external OIDC/OAuth interoperability remain untested.
- CI workflows, cross-platform release artifacts, registry SBOM/provenance publication have not run on GitHub.
- FUSE performance comparison, interrupted downloads/cache exhaustion/offline full-working-set stress, broader semantic contradictions/artifact-only claims, and exhaustive failure-injection tests remain to broaden.
- Snapshot APIs load corpus content into Go memory; pagination/database keyword retrieval and complete metrics/dashboards are needed before large-scale production claims.

The cloud deployment is not authorized implicitly by local Terraform validation. No Terraform apply or image publication has been performed.

## Local checkpoint

Implementation commit: `d832614`. Local API/console remains at http://localhost:18787 for this workstation (8787 was occupied). Obtain the bootstrap token with `make bootstrap-token`. No provider secret values are committed.
