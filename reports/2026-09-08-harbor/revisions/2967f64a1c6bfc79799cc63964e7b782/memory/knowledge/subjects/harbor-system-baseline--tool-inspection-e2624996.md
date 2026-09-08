# Harbor system baseline (tool inspection)

# Harbor system baseline (tool inspection)

## Harbor system baseline (tool inspection)

**Status: historical, tool-measured** — re-read as evidence of a past state per the user's change record, not as current decisions [cite:harbor-0018] [cite:harbor-0015].

The `inspect_system` tool reported the synthetic Harbor baseline identically in both sessions [cite:harbor-0004] [cite:harbor-0018]:

- **Architecture:** Go webhook receiver writes a PostgreSQL outbox row in the same transaction as the business change.
- **Team:** three engineers; no Kafka expertise.
- **Data residency:** EU-only event storage.
- **Load:** average 400 events/s; peak 2,400 events/s for 20 minutes (superseded by the 3,600 events/s requirement [cite:harbor-0022]); payload 1,500 bytes.
- **Consumer throughput:** measured sustainable 600 events/s per replica.
- **Processing:** idempotent by event_id.
- **Storage:** 100 GiB free in existing PostgreSQL.
- **Budget:** EUR 900/month ceiling; no vendor price quotes available.
- **Timeline:** launch target 15 November 2026 (superseded by 1 December 2026 [cite:harbor-0015]).
- **Retention proposal:** raw payloads 30 days (superseded by Legal-approved 7 days [cite:harbor-0015]); metadata 90 days (still current).

Per the assistant's change-session analysis, the still-current measured values are: 400 events/s average, 1,500-byte payloads, 600 events/s per replica, 100 GiB free, 90-day metadata retention, three engineers, and EU-only storage [cite:harbor-0028]. Values that must not be stated as current: 15 November launch, 2,400/s peak, six consumers, 30-day retention, and any characterization of the v1 decisions as tentative [cite:harbor-0028].

The same-transaction outbox write means disk exhaustion would halt business writes, not just event delivery — a coupling the assistant highlighted in its risk analysis [cite:harbor-0012].

Related: Harbor capacity calculations (tool, 50% headroom) — consumes these measurements; Harbor v1 architecture decision — the user decisions that superseded several baseline values.

- [[knowledge/subjects/harbor-capacity-calculations--tool--50--headroom-b79b7d93.md]]
- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]