# Integrating an agent

One credential identifies one user's memory. Give that user's agents separate scoped tokens from **Connections**. Never select a user through an event field, reuse one token across different people, or share their retry spools. The local bootstrap token has management access; use narrower tokens for agents.

## Online capture

Use the [LangChain callback package](../integrations/langchain/README.md) for the implemented reference integration. It captures messages and tool execution, batches asynchronously, and keeps a bounded SQLite retry spool. Reuse stable event envelopes when retrying. Call `finish()` for session completion and `flush()` to surface delivery failures.

For another harness, implement the [Protobuf event contract](../proto/trace2mem/v1/trace2mem.proto) and call `trace2mem.v1.IngestionService/AppendEvents`. Stable event IDs are unique within the authenticated user's memory. An identical repeated event is harmless; different content with the same ID conflicts. Session, source, agent and tool-call identifiers retain their distinct meanings. Tool results may arrive before their calls. Source and ingestion time are separate.

```sh
curl --fail-with-body "$TRACE2MEM_URL/trace2mem.v1.IngestionService/AppendEvents" \
  -H "Authorization: Bearer $TRACE2MEM_TOKEN" -H 'Content-Type: application/json' \
  --data '{"events":[{"eventId":"example-1","sessionId":"conversation-1","occurredAt":"2026-09-08T10:00:00Z","actor":{"role":"user","agentId":"my-agent"},"source":{"id":"my-harness"},"message":{"text":"The project uses Go."}}]}'
```

Accepted means durable, not compiled. Missing models preserve events and block compilation. Unsupported payloads are explicit errors; larger supported text belongs in bounded artifacts. Adapter code runs beside the agent, never as an uploaded server plugin.

## Delayed import and daily compilation

```sh
bin/trace2mem schedule --mode daily --at 02:00 --timezone UTC
bin/trace2mem import --file saved-events.jsonl
bin/trace2mem status
```

JSONL uses one Protobuf-JSON event per line. The service schedules compilation of already ingested evidence; arrange periodic file uploads in the user's own harness or scheduler. Daily mode coalesces missed runs into one catch-up. In a daylight-saving gap it uses the first valid minute after the selected time; in a repeated hour it runs once. Manual mode requires `bin/trace2mem compile`. Fully disconnected inference requires local models and storage as well as saved input.

## Initial context and retrieval

At conversation start, read `knowledge/index.md` through ReadFile or call MCP `memory_index`. The LangChain helper defaults to 8,000 characters and reports truncation. Add it to the agent's context as untrusted memory evidence with its revision. Search and follow citations for more detail; a local miss is not proof that the remote corpus lacks evidence.

Connect Streamable HTTP MCP at `$TRACE2MEM_URL/mcp` with a bearer token. Tools are `memory_index`, `memory_search`, `memory_read`, `memory_evidence`, `memory_context`, and `memory_status`. Only `memory_context` generates an answer; ordinary search and file/evidence reads remain useful without generation. Search reports semantic degradation explicitly.

```sh
bin/trace2mem context --query 'project language' --target ./working-set
bin/trace2mem sync --target ./snapshot
bin/trace2mem mount --target ./mounted-memory
```

Context synthesis requires configured models and a published revision. Filesystem operations pin a revision and verify hashes. Linux is the supported FUSE test platform; use directory sync on macOS unless you have tested its external FUSE installation. Cached files work offline; uncached reads fail clearly. Start a new snapshot to adopt a newer revision.

## Pi and Hermes adapters

The [Pi extension](../integrations/pi/README.md) and [Hermes plugin](../integrations/hermes/README.md) capture framework messages, tool observations and session lifecycle into the common envelope. They share the [bounded durable client](../integrations/core/README.md), load initial index context, and register API-backed search/read/evidence tools. Their READMEs document validated contracts, installation, capture limitations and the results of the small live harness evaluation. Connecting MCP alone still supplies neither capture nor initial context.

## Custom-agent walkthrough: capture, orient, read

Start the service and configure models using the quickstart. Set `TRACE2MEM_URL` and `TRACE2MEM_TOKEN`; the agent token needs `read` and `ingest`. Keep management credentials outside the agent. The AppendEvents example above works from any HTTP client without a framework dependency.

1. Assign a stable session ID for each conversation and a fresh event ID for each occurrence. Capture user/assistant messages and tool calls/results, preserving the tool call ID on its result. Save envelopes before upload so retries send exactly the same content. Treat a correction as a new event.
2. Append batches while the conversation proceeds. Record the returned watermark. At completion call CloseSession; automatic mode schedules immediate compilation. Manual mode requires RequestCompilation. Closing does not override daily/manual scheduling.
3. Check GetIngestionStatus. When `processedWatermark` reaches your accepted watermark, compilation has processed those events. A revision may remain unchanged for a justified no-op. Surface blocked models or failures; do not wait indefinitely.
4. In the next conversation, read the compact index, keep its revision, and expose search, file, and evidence tools to your agent. Let it request details only when needed. Memory text is evidence, not instructions that override your agent's trusted policy.

These are Connect JSON requests against the same contract as the generated Go client:

```sh
curl --fail-with-body "$TRACE2MEM_URL/trace2mem.v1.IngestionService/CloseSession" \
  -H "Authorization: Bearer $TRACE2MEM_TOKEN" -H 'Content-Type: application/json' \
  --data '{"sessionId":"conversation-1"}'

curl --fail-with-body "$TRACE2MEM_URL/trace2mem.v1.IngestionService/GetIngestionStatus" \
  -H "Authorization: Bearer $TRACE2MEM_TOKEN" -H 'Content-Type: application/json' \
  --data '{}'

curl --fail-with-body "$TRACE2MEM_URL/trace2mem.v1.MemoryService/ReadFile" \
  -H "Authorization: Bearer $TRACE2MEM_TOKEN" -H 'Content-Type: application/json' \
  --data '{"path":"knowledge/index.md"}'
```

ReadFile returns `content`, `revision`, and `sha256`. Apply your own initial-context size budget, keeping the revision and an explicit truncation indicator if you shorten the index. Before the first publication the index is unavailable; your agent can continue its ordinary conversation and keep capturing events.

For subsequent reads/searches, pass the returned revision explicitly. Replace `REVISION_FROM_INDEX` below with that exact value; paths and evidence IDs should come from actual tool results:

```json
{"query":"project language","revision":"REVISION_FROM_INDEX","limit":5}
```

Send this to `trace2mem.v1.MemoryService/Search`. Each hit contains a path, content, and citation IDs. Read a selected file with `ReadFile` using `{"revision":"REVISION_FROM_INDEX","path":"PATH_FROM_RESULT"}`. Resolve a citation with `GetEvidence` using `{"eventId":"EVENT_ID_FROM_CITATION"}`. Evidence access reflects current forgetting suppression; it is not a way to recover deleted information from an older revision.

Expose these operations as tools in your existing agent loop. The caller's model can synthesize the answer itself; GetContext/`memory_context` is optional service-side generation. Search can be skipped when the index already identifies the right page. The [Kimi example](KIMI_AGENT.md) demonstrates this exact progressive retrieval pattern with real model-selected tools.

For file-oriented agents, sync a snapshot and give the agent read access to that directory, or use a Linux FUSE mount. A path named `/memory` is simply your chosen mount location; Trace2Mem does not automatically attach it to an agent or container. Give containerized agents access to the prepared directory through your container's volume configuration. Start a new snapshot/mount when you want newer memory.
