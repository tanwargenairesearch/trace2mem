# Harbor v1 architecture decision

# Harbor v1 architecture decision

## Harbor v1 architecture decision

**Status: current, user-decided** [cite:harbor-0015] [cite:harbor-0022].

The user confirmed the v1 decisions via an authoritative change record: Go implementation, PostgreSQL outbox, EU-only event storage, EUR 900/month infrastructure ceiling, and at-least-once delivery with event_id deduplication [cite:harbor-0015]. These were previously tentative; the change record elevated them to confirmed [cite:harbor-0028].

**Current parameters set by the user:**
- **Launch date:** 1 December 2026, replacing 15 November 2026 [cite:harbor-0015].
- **Retention:** raw payload 7 days, Legal-approved, replacing the 30-day proposal; metadata retention unchanged at 90 days [cite:harbor-0015].
- **Peak requirement:** 3,600 events/s, replacing the original 2,400 events/s; average remains 400 events/s [cite:harbor-0022].
- **Consumers:** nine consumers replace the initial six-consumer target, subject to validating database contention [cite:harbor-0022].
- **Kafka:** no Kafka adoption is approved; the PostgreSQL outbox stays [cite:harbor-0022].

Earlier history (superseded): the user first decided PostgreSQL outbox consumers for v1 with six consumers as the initial peak target subject to load validation, directing that no measured cost be claimed because no vendor quotes exist [cite:harbor-0011]. The assistant's sizing math confirms nine consumers at 50% headroom (3,600 × 1.5 = 5,400 events/s ÷ 600 events/s per replica) [cite:harbor-0028] [cite:harbor-0025], and the launch review memo later reinforced the dedup, alert, and cost constraints [cite:harbor-0029].

The assistant's supporting analysis (assistant-originated, not user fact): 7-day retention shrinks raw storage to ~338 GiB but still exceeds 100 GiB free, and the 7-day window is now the implicit replay limit [cite:harbor-0021].

Related: Harbor system baseline (tool inspection) — supplies the measured values the decision builds on; Harbor capacity calculations (tool, 50% headroom) — validates the nine-consumer figure; Harbor launch review memo (user-finalized) — operationalizes the decision into launch gates; Harbor unapproved recommendations and uncosted items — tracks proposals this decision did not approve.

- [[knowledge/subjects/harbor-system-baseline--tool-inspection-e2624996.md]]
- [[knowledge/subjects/harbor-capacity-calculations--tool--50--headroom-b79b7d93.md]]
- [[knowledge/subjects/harbor-launch-review-memo--user-finalized-2eac219a.md]]
- [[knowledge/subjects/harbor-unapproved-recommendations-and-uncosted-items-cb539f5f.md]]
- [[knowledge/subjects/harbor-accepted-operational-policy-9143850a.md]]