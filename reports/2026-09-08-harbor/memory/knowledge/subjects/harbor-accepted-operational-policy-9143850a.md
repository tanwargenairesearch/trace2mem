# Harbor accepted operational policy

## Harbor accepted operational policy

**Status: current, user-accepted** [cite:harbor-0013].

The user recorded the following as accepted operational policy for Harbor [cite:harbor-0013]:

1. Processing semantics are **at-least-once**, with consumers deduplicating by `event_id`.
2. **Alert** when the oldest unprocessed outbox row reaches **120 seconds** of age.
3. **Pause noncritical producers** at **300 seconds** of oldest-outbox age.
4. **Replay only after root-cause mitigation** — not merely after diagnosis.
5. Ownership: **Mira** owns receiver/outbox; **Leon** owns consumers and load testing; **Saanvi** owns operations.

The assistant noted that no tool persisted this policy; its response message served as the record, and nothing had been executed [cite:harbor-0014]. The assistant also carried forward prior accepted constraints into the policy record: PostgreSQL outbox, Go, EU-only storage, 6 consumers for peak subject to load validation, the EUR 900/month ceiling, and no claimed measured costs [cite:harbor-0014] [cite:harbor-0011].

The policy's age-based triggers (120s alert, 300s pause) are the approved backpressure mechanism; additional disk-based guardrails proposed by the assistant remain unapproved [cite:harbor-0014].

- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]
- [[knowledge/subjects/harbor-implementation-sequence--assistant-proposal-a0f8d747.md]]