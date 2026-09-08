# Architecture and contract

The provider-independent domain is in `internal/domain`; storage, Dream, transport, and model implementations depend inward on it. PostgreSQL is the publication coordinator. Blob storage holds uploaded sources. The server and worker share migrations and storage but run independently.

![Trace2Mem: foreground agents, three-layer durable memory, and background Dream maintenance](assets/architecture.svg)

## Reading the diagram

Read it in three horizontal bands, following the same foreground / durable memory / background distinction used in [Brain Figure 2](https://www.perplexity.ai/hub/blog/brain-agentic-memory-as-a-knowledge-wiki). This is an original diagram of Trace2Mem's implementation, not a reproduction of Brain's system.

1. **Foreground:** a user agent records experience through an adapter and progressively retrieves memory through tools or an optional local working set. Requests and returned content use the same authenticated API. Capture is a separate integration from MCP retrieval.
2. **Durable memory:** each user owns original session evidence, distilled notes, and linked subject pages. Published summaries, notes, and wiki pages share a revision. Ingested raw evidence can be available before compilation; ingestion does not publish generated memory. Notes and wiki claims cite original source events directly. Subject links provide related context.
3. **Background:** Dream inspects new evidence and existing memory, stages a coherent update, verifies it, and publishes atomically. A justified no-op advances the processed watermark without changing content. Failed verification leaves changes unpublished; semantic judgment can still be wrong.

Solid arrows show foreground access/capture and citation direction; dashed arrows show background maintenance. The local working set is optional, read-only, and pinned to one revision; refreshing requires a new snapshot or mount. Search is available between index reading and evidence inspection whenever the index does not identify the needed path. Forgetting suppression still applies to server-side evidence access.

PostgreSQL, interchangeable blob storage, and independently configured generation/embedding models support these flows. The lower row names dependencies, not additional pipeline stages. Git is not the publication coordinator. External connector investigation and Dream subagents are deferred and therefore absent from this diagram.

[Download SVG](assets/architecture.svg) · [Download PNG](assets/architecture.png)

| Diagram component | Implementation reference |
|---|---|
| Capture and scoped ingestion | [Ingestion handlers](../internal/server/ingestion.go), [LangChain adapter](../integrations/langchain/README.md) |
| MCP / API memory access | [Memory handlers](../internal/server/memory.go), [MCP tools](../internal/server/mcp.go) |
| Pinned local working set | [Filesystem cache](../filesystem/cache.go), [FUSE](../filesystem/fuse.go) |
| Per-user memory and scheduling | [Ownership](../internal/store/user_memory.go), [Scheduling](../internal/store/scheduling.go) |
| Dream, staging, verification | [Dream engine](../internal/dream/dream.go), [Composition](../internal/dream/compose.go) |
| Publication and no-op | [Store](../internal/store/store.go), [No-op transaction](../internal/store/noop.go) |

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

Embedding identity includes provider, model, dimension selection, endpoint/project/location, and chunking algorithm. New configurations are staged while the current configuration remains usable. The worker checkpoints vectors in batches of 16 pages keyed by the memory configuration generation, resuming after interruption. It atomically publishes a fully reindexed revision and activates the pending configuration. Changing configuration or forgetting invalidates staged work. Historical revisions with incompatible vectors return keyword results with explicit `semantic_status`; their content remains readable until retention/forgetting removes it.

Budgets reserve conservative token estimates before provider requests, serialized per user. Generation usage is reconciled when available; embedding usage is marked estimated. Failed calls retain their reservation. Limits constrain request bytes, tool steps, and time. Daily budget exhaustion requires operator action or a later retry.

## Access and portability

Connect JSON and gRPC, six read-only MCP tools, and directory/FUSE access expose the same published revision. The Memory Agent performs bounded searches and validates final citation IDs against inspected material. Search can run without a generation call and reports semantic fallback explicitly. A miss in the compact index or a local working set does not imply the remote corpus has no evidence.

The filesystem fetches its manifest once, uses local metadata, verifies content hashes, coalesces same-hash reads within one cache, and bounds shared cache bytes. Files are immutable per revision; create a new snapshot/mount to advance. FUSE targets Linux; macOS requires separately installed FUSE and is experimental. Already exported or downloaded material cannot be recalled remotely.

## Current implementation limits

Manifest reads return metadata only; file reads target one path; keyword candidates are ranked and bounded in PostgreSQL. Full snapshots remain in compilation/evaluation/export internals, and very large corpora still need measured scaling limits. Embedding pooling and deterministic fact/evidence rubrics require broader real-model measurement. Artifact inspection currently supports UTF-8 text ranges. The console is functional Go HTML with small JavaScript, not a finished hosted product. External OIDC/OAuth interoperability and cloud restore/deployment require environment-specific smoke tests.

The Responses adapter extracts final `output_text` from message items; reasoning items are not returned as answers or included in retrieval transcripts. This follows the typed [Responses output contract](https://developers.openai.com/api/reference/typescript/resources/beta/subresources/responses/methods/create). Tool calls remain separate structured records.

Verification and wiki composition require exactly one named tool invocation. The Responses adapter requests it through tool_choice; Ollama relies on the maintenance prompt because this adapter does not send a native forced-choice option. Both HTTP adapters reject a reply that lacks the required invocation, returning any reported usage for reconciliation. This preserves the publication gate, but model compliance and retry frequency can differ between providers. Ordinary agent tools remain optional and retain their existing selection behavior.

If a composed draft fails structural validation, Dream stores the rejected draft and diagnostic, then permits one repair call using the inspected evidence and exact error. Repair inputs remain bounded. A second structural failure stops the run; a structurally repaired draft still undergoes semantic verification and fenced publication. Repair does not turn an unsupported claim into an accepted one by bypassing verification.

The service Memory Agent starts from a compact index and can search bounded excerpts, read selected file ranges by character offset, and resolve original evidence from its pinned revision. Only sources resolved through the evidence tool are eligible for final citation validation. It allows at most 12 model rounds and 16 tool calls per round; each tool result is capped at 128 KiB. Citation validation does not prove semantic support.

Dream prior-note search returns explicit truncation and continuation cursors, including when its byte budget fills. Subject reads resolve cited evidence; verification includes a bounded inbound/outbound subject neighborhood. Every nonempty draft passage must have a recorded support, attribution, and temporal check. Any failed or missing passage check blocks publication even if the overall verdict says supported. These model judgments remain fallible; the Harbor regression tests enforcement of a reported temporal conflict, not a real model's ability to discover every conflict.
