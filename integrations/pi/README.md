# Pi adapter

Pi extension with live text/message capture, tool calls/results, session lifecycle, bounded initial index context, and `memory_search`, `memory_read`, `memory_evidence` tools. Uses the shared Python SQLite worker over a private stdio pipe; no credentials are passed as command-line arguments.

From the repository root:

```sh
python3 -m venv /tmp/trace2mem-pi
/tmp/trace2mem-pi/bin/pip install ./integrations/core
export TRACE2MEM_PYTHON=/tmp/trace2mem-pi/bin/python
export TRACE2MEM_URL=http://localhost:8787
# Set TRACE2MEM_TOKEN privately to a per-user token with read + ingest scopes.
export TRACE2MEM_SPOOL="$HOME/.local/share/trace2mem/pi.sqlite"
pi -e ./integrations/pi/index.ts
```

Use an absolute path to `index.ts` when launching elsewhere. Keep `bridge.mjs` beside it. Pi supplies the `typebox` runtime import. The extension is type-checked against `@earendil-works/pi-coding-agent` **0.85.1** (the current package name; older `@mariozechner` releases are not validated). The [upstream extension types](https://github.com/badlogic/pi-mono/blob/main/packages/coding-agent/src/core/extensions/types.ts) were checked on 2026-09-09. The lockfile records the exact development dependency tree.

`message_end` captures new user/assistant text and tool results. Assistant tool-call blocks preserve their IDs, names and arguments. Source timestamps use Pi's millisecond timestamp. Thinking blocks are excluded; custom extension messages, including injected memory, are ignored. Images/other unsupported content raise a capture error rather than being flattened or silently omitted. Sessions use `pi-` plus the SHA-256 of Pi's session ID (the original is retained in event metadata). Shutdown queues closure except on extension reload, which only flushes; a replacement runtime starts a fresh bridge. This requires Pi's documented shutdown/replacement lifecycle.

Initial context is bounded to 3,500 content characters, tagged as untrusted evidence, and includes revision and truncation metadata. The index is loaded at session start and supplied as one transient custom message through Pi’s `context` hook. It is not persisted to history and replaces any previous adapter context in the outgoing request. A missing index/offline service warns and leaves capture enabled. Retrieval tools accept explicit revisions so the model can pin reads to the index/search result. Memory requires compilation before new captures can be retrieved.

[Queue limits, isolation, outage recovery and retry semantics](../core/README.md) apply. Pi logs extension callback failures; a failed callback is not automatically replayed. A hard crash before a hook is durably acknowledged may lose that hook. Monitor capture errors, including oversized events and full spools.

Validation (Node 22+):

```sh
npm ci --ignore-scripts --prefix integrations/pi
npm run check --prefix integrations/pi
npm test --prefix integrations/pi
```

Tests exercise the real Node-to-Python bridge against a local HTTP fixture, including flush/restart and spool lock release. Type-checking uses actual Pi types. The actual Pi 0.85.1 CLI also passed a local scripted-model smoke test (two provider requests, file tool pair, bounded context and shutdown capture); see the [evaluation report](../../reports/2026-09-09-adapter-smoke/README.md). A single OpenRouter capture → live compilation → fresh recall trial passed, including two evidence reads; see the [live evaluation](../../reports/2026-09-09-adapter-live/README.md). This is not a broad reliability benchmark.
