# Harbor v1 architecture decision

## Harbor v1 architecture decision

**Status: current, user-decided** [cite:harbor-0011].

The user decided that Harbor v1 will adopt PostgreSQL outbox consumers, retain the Go implementation, keep EU-only storage, and respect an infrastructure ceiling of EUR 900/month [cite:harbor-0011]. The user explicitly directed that no measured cost may be claimed, since no vendor price quotes exist [cite:harbor-0011] [cite:harbor-0004]. Six consumers is the initial peak target, expressly subject to load validation rather than a settled figure [cite:harbor-0011].

Context for the decision: the existing Go receiver already writes the PostgreSQL outbox in the same transaction as the business change, and processing is idempotent by event_id [cite:harbor-0004]. Tool calculations showed 6 consumer replicas are needed to cover the 2,400 events/s peak with 50% headroom (3,600 events/s target at 600 events/s measured per replica) [cite:harbor-0009] [cite:harbor-0004].

The user also flagged the retention risk given only 100 GiB of free PostgreSQL storage and requested bounded backpressure and recovery planning as part of this decision [cite:harbor-0011]. The assistant's supporting analysis (assistant-originated, not user-approved fact) held that 30-day raw retention is infeasible in PostgreSQL at ~48.3 GiB/day average raw volume and that Kafka adoption should be deferred [cite:harbor-0012] [cite:harbor-0010]. Kafka remains a deferred, unapproved option [cite:harbor-0014].

- [[knowledge/subjects/harbor-system-baseline--tool-inspection-e2624996.md]]
- [[knowledge/subjects/harbor-capacity-calculations--tool--50--headroom-b79b7d93.md]]
- [[knowledge/subjects/harbor-accepted-operational-policy-9143850a.md]]
- [[knowledge/subjects/harbor-unapproved-recommendations-and-uncosted-items-cb539f5f.md]]