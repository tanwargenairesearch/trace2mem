# Harbor assistant analysis of change impacts

# Harbor assistant analysis of change impacts

## Harbor assistant analysis of change impacts

**Status: current, assistant-originated analysis** — not user-approved fact [cite:harbor-0021] [cite:harbor-0028].

**Retention impact:** the assistant computed that Legal-approved 7-day raw retention cuts raw storage from ~1,448 GiB (30 days) to ~338 GiB, a ~76.7% reduction, but this still exceeds the 100 GiB free on existing PostgreSQL; the real shortfall is larger because tool figures exclude indexes, WAL, and metadata. Storage expansion therefore remains a launch blocker, though a smaller procurement that fits more easily under the EUR 900/month ceiling [cite:harbor-0021].

**Replay-window impact:** with at-least-once delivery and event_id dedup, the 7-day raw retention is now the implicit replay window. Downstream consumers more than 7 days behind cannot be replayed from raw payloads; dead-lettered events must be resolved within the window or archived separately. DR runbooks and consumer-lag alerting must be re-baselined well under 7 days; 90-day metadata supports audit/lineage but not payload replay [cite:harbor-0021].

**Consumer sizing and rollout:** nine consumers matches the tool math (3,600 × 1.5 ÷ 600) exactly; average load needs only one replica, so nine is peak-provisioned capacity. The rollout is gated on validating PostgreSQL outbox contention with 9 concurrent pollers (e.g., `SELECT … FOR UPDATE SKIP LOCKED`), with batch-claim polling or a dispatcher/consumer split as fallbacks that stay on PostgreSQL because Kafka is not approved. Steady state runs at 1–2 replicas, scaling to 9 for peaks; scale-out is safe under at-least-once plus event_id dedup [cite:harbor-0028].

**Timeline:** the 16-day launch slip (15 November → 1 December 2026) is absorbed by the storage expansion and updated retention/replay runbooks without new scope [cite:harbor-0021].

**Historical-value discipline:** the assistant tabulated values that must not appear as current — 15 November 2026 launch, 2,400 events/s peak, six-consumer target, 30-day raw retention, and tentative v1 decisions — against their replacements [cite:harbor-0028].

Related: Harbor v1 architecture decision — the user decisions this analysis interprets; Harbor capacity calculations (tool, 50% headroom) — the tool outputs underpinning the arithmetic; Harbor unapproved recommendations and uncosted items — the proposals this analysis flagged but the user has not approved.

- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]
- [[knowledge/subjects/harbor-capacity-calculations--tool--50--headroom-b79b7d93.md]]
- [[knowledge/subjects/harbor-unapproved-recommendations-and-uncosted-items-cb539f5f.md]]