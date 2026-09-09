# Shared Trace2Mem capture client

Install from the repository root: `pip install ./integrations/core`. Python 3.10+ on Linux/macOS; no runtime dependencies. `from trace2mem import MemoryClient` provides the same durable SQLite client used by LangChain, Pi and Hermes.

Each process needs its own spool, bound to one server and credential. Use absolute spool paths in a private data directory, outside your project and plugin installation. The queue permits 10 MiB of pending event bodies, 20,000 queued events plus receipts, and 60 KiB per event. Full queues and unsupported media raise errors. Nothing is evicted to make room.

Pi/Hermes capture commits all envelopes from a callback in one transaction before acknowledgment. Invalid events, conflicts and capacity exhaustion roll back the entire callback. A background worker uploads batches and retries network failures, 429 and 5xx responses up to eight consecutive failures. `client.error` reports delivery failure; `flush()` surfaces failure or timeout. Reopen the same spool to resume delivery after resolving the cause. Stable stored envelopes make lost-acknowledgment retries idempotent; callbacks themselves are new occurrences, not replay deduplication. Close retains undelivered events. Already accepted events survive only on the service plus local hash receipts.

Do not share a credential between people. Hermes gateway deployments must isolate users into separate processes/profiles with separate tokens and spools; this adapter does not route credentials by sender. Concurrent Pi instances also require separate spool paths. Rotate a token only after draining/archiving its old spool, or explicitly migrating it.

The Pi stdio bridge is started by its extension. To retry a retained Pi spool after an outage without opening an agent, set the three `TRACE2MEM_*` variables documented in the adapter README and run:

```sh
printf '%s\n' '{"id":1,"op":"flush"}' | python -m trace2mem.bridge
```

The response reports success or an error. This also drains Hermes spools because the queue format is shared. Only run it when the original writer has stopped. Lifecycle events are durably queued with ordinary events and trigger the service's configured compilation schedule when delivered.

Validation:

```sh
python -m unittest discover -s integrations/core/tests -v
```
