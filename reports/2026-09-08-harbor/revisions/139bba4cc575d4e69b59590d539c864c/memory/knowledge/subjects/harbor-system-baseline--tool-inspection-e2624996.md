# Harbor system baseline (tool inspection)

## Harbor system baseline (tool inspection)

**Status: current, tool-measured** [cite:harbor-0004].

The `inspect_system` tool reported the following synthetic Harbor baseline [cite:harbor-0004]:

- **Architecture:** Go webhook receiver writes a PostgreSQL outbox row in the same transaction as the business change.
- **Team:** three engineers; no Kafka expertise.
- **Data residency:** EU-only event storage.
- **Load:** average 400 events/s; peak 2,400 events/s sustained for 20 minutes; payload size 1,500 bytes.
- **Consumer throughput:** measured sustainable 600 events/s per replica.
- **Processing:** idempotent by `event_id`.
- **Storage:** 100 GiB free in existing PostgreSQL.
- **Budget:** infrastructure ceiling EUR 900/month; no vendor price quotes available.
- **Timeline:** launch target 15 November 2026.
- **Retention proposal:** raw payloads 30 days, metadata 90 days.

The same-transaction outbox write means disk exhaustion would halt business writes, not just event delivery — a coupling the assistant highlighted in its risk analysis [cite:harbor-0012].

- [[knowledge/subjects/harbor-capacity-calculations--tool--50--headroom-b79b7d93.md]]
- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]