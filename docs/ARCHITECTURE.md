# Architecture and contract

The provider-independent domain is in `internal/domain`; storage, Dream, transport, and model implementations depend inward on it. PostgreSQL is the publication coordinator. Blob storage holds uploaded sources. The server and worker share migrations and storage but run independently.

## Durable event flow

Authenticated identity resolves one personal memory. Verified OIDC issuer/subject pairs identify users; per-user token scopes authorize reads, ingestion, and management. No caller-selected memory identifier is accepted. Append accepts up to 256 events, each at most 64 KiB in canonical Protobuf JSON. Identical event-ID/content retries are harmless; changed content conflicts. Scheduling and insertion share a transaction. Original source times remain separate from ingestion order. Tool events retain call relationships even when results arrive late. Unsupported payloads fail explicitly.

The queue issues renewable leases and monotonically increasing fencing tokens. Dream reads a bounded incremental watermark, inspects related prior notes, proposes observations, and separately verifies support. Every publication locks the user’s memory and checks parent revision, deletion generation, and live lease token. Source text is untrusted evidence and cannot authorize service actions. Worker tools have no shell execution capability.

## Memory

- `sessions/evidence/`: original event envelopes alongside inspected excerpts; updated session summaries are model-authored.
- `notes/`: readable Markdown observations with hidden structured provenance, actor attribution, stable IDs, citations, status, and supersession links.
- `knowledge/`: subject pages, compact index, links, and compilation log.

Corrections preserve prior observations with temporal status and supersede individual facts. Subject pages changed by a compilation replace that subject's notes while untouched pages carry forward. Structural and model-based semantic verification gate publication. Semantic judgment is recorded and can be wrong; scripted tests do not establish real-model factual accuracy.

## Models and indexing

Generation and embeddings have separate Go interfaces and configuration. OpenAI uses Responses function-call/result items; Ollama uses its native tool-call protocol. Gemini API-key embeddings use `embedContent`; Vertex IAM embeddings use `predict`. See the [Gemini API](https://ai.google.dev/api/embeddings) and [Vertex text embedding API](https://docs.cloud.google.com/vertex-ai/generative-ai/docs/embeddings/get-text-embeddings).

Embedding identity includes provider, model, dimension selection, endpoint/project/location, and chunking algorithm. New configurations are staged while the current configuration remains usable. The worker checkpoints vectors in batches of 16 pages keyed by the space generation, resuming after interruption. It atomically publishes a fully reindexed revision and activates the pending configuration. Changing configuration or forgetting invalidates staged work. Historical revisions with incompatible vectors return keyword results with explicit `semantic_status`; their content remains readable until retention/forgetting removes it.

Budgets reserve conservative token estimates before provider requests, serialized per space. Generation usage is reconciled when available; embedding usage is marked estimated. Failed calls retain their reservation. Limits constrain request bytes, tool steps, and time. Daily budget exhaustion requires operator action or a later retry.

## Access and portability

Connect JSON and gRPC, six read-only MCP tools, and directory/FUSE access expose the same published revision. The Memory Agent performs bounded searches and validates final citation IDs against inspected material. Search can run without a generation call and reports semantic fallback explicitly. A miss in the compact index or a local working set does not imply the remote corpus has no evidence.

The filesystem fetches its manifest once, uses local metadata, verifies content hashes, coalesces same-hash reads within one cache, and bounds shared cache bytes. Files are immutable per revision; create a new snapshot/mount to advance. FUSE targets Linux; macOS requires separately installed FUSE and is experimental. Already exported or downloaded material cannot be recalled remotely.

## Current implementation limits

Snapshot retrieval loads page content into memory; very large corpora need paginated metadata and database-side keyword retrieval before production scale claims. Embedding pooling and expected-substring evaluation are baseline implementations requiring real-model measurement. Artifact inspection currently supports UTF-8 text ranges. The console is functional Go HTML with small JavaScript, not a finished hosted product. External OIDC/OAuth interoperability and cloud restore/deployment require environment-specific smoke tests.
