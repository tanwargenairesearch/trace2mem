# Harbor implementation sequence (assistant proposal)

## Harbor implementation sequence (assistant proposal)

**Status: current, assistant-proposed** — an implementation plan built on the user-accepted policy; the steps themselves were proposed by the assistant, not separately approved by the user [cite:harbor-0014] [cite:harbor-0013].

The assistant proposed an 8-step sequence, each with an acceptance check [cite:harbor-0014]:

1. **Mira** — emit an `oldest_outbox_age_seconds` metric (age of oldest unprocessed outbox row); check: synthetic lag injection shows the metric updating at ≤10s granularity.
2. **Saanvi** — wire alerting to page at 120s and auto-pause noncritical producers at 300s; check: game-day shows alert ≤120s, pause ≤300s, automatic resume below 120s.
3. **Mira** — pause mechanism returning HTTP 429 with `Retry-After` to noncritical producers while business-path writes continue; check: paused producers receive 429 during induced lag while critical writes succeed.
4. **Leon** — enforce `event_id` deduplication (unique constraint/upsert) in consumers; check: duplicate-delivery test yields exactly one side effect per `event_id`.
5. **Leon** — deploy 6 consumers (600 events/s each measured → 3,600 events/s vs. 2,400 events/s peak) and validate; check: sustain 2,400 events/s for >20 minutes with zero 120s alerts [cite:harbor-0004].
6. **Leon** — failure and drain test: kill one consumer mid-peak (residual 3,000 events/s), then restore; check: net drain ≈3,200 events/s at average inflow, and a full 20-minute peak backlog (~2.88M events) drains in ≤40 minutes during a following peak.
7. **Saanvi** — replay runbook with a hard gate blocking replay until mitigation sign-off; check: a rehearsed pre-mitigation replay attempt is refused, and post-mitigation replay dedups correctly.
8. **Saanvi** — operational sign-off before the 15 November 2026 launch [cite:harbor-0004]; check: all prior checks pass consecutively with runbook, alert routing, and pause/resume documented.

The assistant's stated sequencing rationale: the 120s/300s policy depends on the metric existing first; the pause mechanism must be verified before load validation so a failed test cannot endanger business writes; replay gating is rehearsed only after drain behavior is measured [cite:harbor-0014].

- [[knowledge/subjects/harbor-accepted-operational-policy-9143850a.md]]
- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]
- [[knowledge/subjects/harbor-unapproved-recommendations-and-uncosted-items-cb539f5f.md]]