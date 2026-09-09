# Hermes adapter

A native Hermes plugin providing live capture, initial memory context and progressive retrieval. Install the Python client into **the same Python environment that runs Hermes**, then install this directory as a plugin:

```sh
# From the repository root, with the Hermes Python environment active:
python -m pip install ./integrations/core
mkdir -p "$HOME/.hermes/plugins/trace2mem"
cp integrations/hermes/__init__.py integrations/hermes/plugin.yaml "$HOME/.hermes/plugins/trace2mem/"
export TRACE2MEM_URL=http://localhost:8787
# Set TRACE2MEM_TOKEN privately to a per-user token with read + ingest scopes.
export TRACE2MEM_SPOOL="$HOME/.local/share/trace2mem/hermes.sqlite"
hermes plugins enable trace2mem
hermes
```

For a custom `HERMES_HOME`/profile, use its plugin directory and a distinct spool. Keep spool data outside the plugin installation. The credential belongs to exactly one person: use separate gateway processes/profiles for different users. This plugin does not select credentials from sender IDs. See [shared queue behavior and recovery](../core/README.md).

Hook mapping follows the [official Hermes hook contract](https://github.com/NousResearch/hermes-agent/blob/main/website/docs/user-guide/features/hooks.md) and [native plugin registration](https://hermes-agent.nousresearch.com/docs/developer-guide/plugins), checked on 2026-09-09:

| Hook | Capture |
| --- | --- |
| `on_session_start` | Started lifecycle |
| `pre_llm_call` | Current user text; on first turn inject bounded untrusted index with revision/truncation metadata |
| `post_llm_call` | Successful final assistant response |
| `post_tool_call` | Executed/blocked tool call and result, retaining `tool_call_id` and post-policy arguments |
| `on_session_finalize` | Closed lifecycle |

`on_session_end` is deliberately not used for closure because it can run after every turn. Sessions use `hermes-` plus the SHA-256 of the framework session ID (the original is retained in event metadata). Concurrent callbacks share the thread-safe client, without mutable global current-session state. The plugin exposes `memory_search`, `memory_read` (both accept explicit revision) and `memory_evidence`. Memory is evidence, never trusted policy.

This captures completed tool observations and successful final replies; it does not capture private reasoning, intermediate assistant messages, or a tool killed before its post hook fires. Text-only capture rejects unsupported message media. Missing `session_id`/`tool_call_id` is an explicit callback error; older Hermes versions without these fields or `on_session_finalize` are unsupported. Hermes isolates/logs callback errors and continues the agent, so monitor plugin logs for capture failures. A missing index warns and still captures the user message. Delivery failures retain accepted local envelopes for replay; process exit attempts a bounded flush.

Validation uses contract-shaped hook fixtures and the real SQLite client, plus retrieval and registration tests in `integrations/core/tests`. The actual Hermes 0.21.1 plugin dispatcher and tool observer passed a local HTTP smoke test; success uses `status="ok"`. See the [evaluation report](../../reports/2026-09-09-adapter-smoke/README.md) for the checked commit. A single OpenRouter live trial recalled both facts with search citations, but the model omitted `eventId` on two evidence calls. See the [live evaluation](../../reports/2026-09-09-adapter-live/README.md). Explicit evidence verification remains a failed check; this is not a release-wide compatibility claim.
