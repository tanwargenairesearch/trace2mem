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

## Pi and Hermes adapter contracts

No first-party Pi or Hermes adapter is implemented. An adapter must map its framework's message/tool/lifecycle events into the same envelope, spool durable retries, load the initial index, and expose retrieval through MCP or the API. Framework hooks and package versions must be validated by the adapter author. Connecting MCP alone supplies neither capture nor initial context.
