## Project Harbor — Current State

**Architecture (confirmed, user-decided):** Go webhook receiver writes a PostgreSQL outbox row in the same transaction as the business change; EU-only event storage; at-least-once delivery with `event_id` deduplication; EUR 900/month infrastructure ceiling. No Kafka adoption is approved. Team: three engineers, no Kafka expertise. [cite:harbor-0015] [cite:harbor-0022] [cite:harbor-0004]

**Launch date:** 1 December 2026 (superseded 15 November 2026). [cite:harbor-0015]

**Retention policy:** raw payloads 7 days (Legal-approved; superseded 30-day proposal); metadata retention 90 days (unchanged). [cite:harbor-0015]

**Peak capacity:** requirement is 3,600 events/s peak (superseded 2,400/s); average remains 400 events/s. Measured sustainable throughput is 600 events/s per consumer replica. At 50% headroom the tool returns 9 replicas (3,600 × 1.5 = 5,400 ÷ 600); average load needs 1 replica, so nine is peak-provisioned, not steady state. [cite:harbor-0022] [cite:harbor-0025] [cite:harbor-0020]

**Owners:** Mira — receiver/outbox; Leon — consumers/load testing; Saanvi — operations. Confirmed in the accepted operational policy and reaffirmed in the launch-memo directive. [cite:harbor-0013] [cite:harbor-0029]

**Operational thresholds:** alert when oldest unprocessed outbox row reaches 120 seconds; pause noncritical producers at 300 seconds; replay only after root-cause mitigation (not merely diagnosis). These are the approved backpressure mechanism. [cite:harbor-0013] [cite:harbor-0014]

**Launch gates (user-directed):** 3,600 events/s soak test; duplicate-delivery test; recovery drill; EU storage check; actual cost estimate under EUR 900. Memo must include unresolved risks, rollback triggers, and a decision ledger. [cite:harbor-0029]

## Superseded decisions

| Historical (do not state as current) | Replaced by |
|---|---|
| Launch 15 November 2026 | 1 December 2026 [cite:harbor-0015] |
| Peak 2,400 events/s | 3,600 events/s [cite:harbor-0022] |
| Six-consumer initial target [cite:harbor-0011] | Nine consumers, conditional on DB-contention validation [cite:harbor-0022] |
| Raw retention 30 days (proposal) | 7 days, Legal-approved [cite:harbor-0015] |
| v1 decisions "tentative" | Confirmed v1 decisions [cite:harbor-0015] [cite:harbor-0028] |

Earlier tool run at 2,400/s × 1.5 headroom (6 replicas, 311 GB/day) is superseded by the 3,600/s run. [cite:harbor-0009] [cite:harbor-0025]

## Unknowns and unapproved items (not confirmed facts)

- **Storage shortfall:** 7-day raw retention implies ~338 GiB (assistant arithmetic from tool output), but only 100 GiB is free on existing PostgreSQL; real gap is larger since tool figures exclude indexes/WAL/metadata. Storage expansion (~500 GiB or EU object-storage offload) is recommended, not approved. [cite:harbor-0021] [cite:harbor-0020]
- **Cost feasibility:** nine replicas plus storage expansion vs. EUR 900/month is unverifiable — no vendor quotes exist; no cost figure may be stated as measured. An actual cost estimate is a launch gate. [cite:harbor-0028] [cite:harbor-0029] [cite:harbor-0011]
- **DB contention:** nine consumers polling one PostgreSQL outbox is unvalidated; fallbacks (batch-claim polling, dispatcher/consumer split) are proposed, not approved. [cite:harbor-0028]
- **Unapproved guardrails:** replay-window alerts (~24h) and escalation (~3 days); dead-letter archive beyond 7 days; disk-based triggers (<20%/<10% free). [cite:harbor-0021] [cite:harbor-0014]
- **Launch memo:** directed but not evidenced as drafted in the record. [cite:harbor-0029]
- **Risk coupling:** same-transaction outbox write means disk exhaustion halts business writes, not just delivery. [cite:harbor-0012]

All cited event IDs were resolved via memory_evidence: harbor-0004, 0009, 0011, 0012, 0013, 0014, 0015, 0020, 0021, 0022, 0025, 0027, 0028, 0029. No facts were guessed; items above are labeled confirmed vs. assistant-originated.