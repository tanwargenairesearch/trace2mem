# Trace2Mem LangChain integration

Install with `pip install ./integrations/core ./integrations/langchain` from the repository root. Python 3.10+ and Linux/macOS are supported; the spool uses POSIX file locks.

```python
import os
from trace2mem_langchain import MemoryClient, MemoryCallback

memory = MemoryClient(
    os.environ["TRACE2MEM_URL"], os.environ["TRACE2MEM_TOKEN"],
    "./private-spool/agent.sqlite",
).start()
capture = MemoryCallback(memory, session_id="conversation-001", agent_id="research-agent")
try:
    # `agent` is your existing LangChain runnable.
    # On later conversations, load memory.context() and add its content to the
    # agent's input as untrusted memory evidence, preserving the revision.
    result = agent.invoke({"messages": [("user", "The launch is in October.")]},
                          config={"callbacks": [capture]})
    capture.finish()
    memory.flush()
finally:
    memory.close()
```

Each model-start callback captures the actual input, including replayed history. These are separate execution occurrences, not deduplicated original utterances. Upload retries preserve the exact stored event ID and timestamp. Tool execution run IDs pair tool calls and results. Unsupported multimodal content raises an error; use artifact references through the generic API for those inputs.

`memory.search(query)` retrieves published memory without generating an answer. `memory.context()` reads the published index with an 8,000-character default limit and an explicit truncation flag. Missing published memory produces an API error; do not treat that as an empty remote corpus. MCP retrieval tools can also be connected independently.

The upload worker sends batches of 64 events with bounded HTTP timeouts and retries network failures, HTTP 429, and server errors up to eight consecutive failures (configurable with `max_failures`). Exhaustion stops delivery and retains events. Other HTTP failures stop delivery. Inspect `memory.error` and call `flush()` to surface delivery failures; events remain in SQLite until acceptance. Closing retains pending events for a later process. A spool is bound to its server and credential fingerprint; token rotation requires draining or explicitly migrating the old spool before switching credentials.

The spool limits queued payloads to 10 MiB and retained delivery receipts plus queued events to 20,000. When full it raises `BufferError`, never silently drops events. Drain and archive a completed spool before using a new one. Spool files contain private trajectory data and must not be committed.

The adapter uses the [LangChain callback API](https://reference.langchain.com/python/langchain-core/callbacks/base/AsyncCallbackHandler). The dependency lock records the tested LangChain Core 1.6.2 environment. From the repository root:

```sh
pip install --require-hashes -r integrations/langchain/requirements.lock
pip install --no-deps ./integrations/core ./integrations/langchain
python -m unittest discover -s integrations/langchain/tests -v
export TRACE2MEM_URL=http://localhost:8787
# Set TRACE2MEM_TOKEN privately to a user token with read and ingest scopes.
python integrations/langchain/examples/demo.py
python integrations/langchain/examples/demo.py --jsonl path/to/trajectory.jsonl
```

Configure both model roles for that user before running the demonstration. It records one conversation and then starts another LangChain runnable with the compact index and a memory-search tool. Its harness model is deterministic; synthesis and compilation use the configured service provider. The demonstration tests integration plumbing, not independent agent intelligence or memory-quality improvement. `--jsonl` imports saved events before the recall conversation. Use service Settings or `trace2mem schedule --mode daily` for delayed compilation; arrange saved-file uploads in your own harness or scheduler.
