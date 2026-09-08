All evidence needed was already resolved in this session; no additional lookups required.

## Harbor Go/No-Go Checklist — Leon & Saanvi

### Leon (consumers / load testing)

- ☐ **DB-contention validation (gating):** load-test the outbox claim pattern (e.g., `SELECT … FOR UPDATE SKIP LOCKED`) with 9 concurrent pollers; measure lock waits and impact on the write transaction. Nine consumers is conditional on this — if it fails, fall back to batch-claim polling or dispatcher/consumer split (still PostgreSQL). [cite:harbor-0022] [cite:harbor-0028]
- ☐ **3,600 events/s soak test** on nine consumers (launch gate). [cite:harbor-0029]
- ☐ **Duplicate-delivery test:** enforce `event_id` dedup (unique constraint/upsert); exactly one side effect per `event_id`. [cite:harbor-0014] [cite:harbor-0029]
- ☐ **Failure + drain test:** kill one consumer mid-peak, restore, measure drain rate. [cite:harbor-0014]
- ☐ **Emit `oldest_outbox_age_seconds` metric** (Mira's step 1, but Leon's tests depend on it existing first). [cite:harbor-0014]
- ☐ **Recovery drill** (joint with Saanvi; launch gate). [cite:harbor-0029]

### Saanvi (operations)

- ☐ **Alert wiring:** page at oldest-outbox age 120s; auto-pause noncritical producers at 300s; verify auto-resume below 120s. [cite:harbor-0013] [cite:harbor-0014]
- ☐ **Pause mechanism check:** paused producers get HTTP 429 + `Retry-After`; business-path writes unaffected. [cite:harbor-0014]
- ☐ **Replay runbook with hard gate:** replay blocked until root-cause mitigation sign-off; rehearse refusal of a pre-mitigation replay. [cite:harbor-0013] [cite:harbor-0014]
- ☐ **EU storage check** (launch gate) — but see storage validation below. [cite:harbor-0029]
- ☐ **Actual cost estimate under EUR 900** (launch gate). [cite:harbor-0029]
- ☐ **Rollback triggers, unresolved risks, decision ledger** documented in the launch memo (directed, not yet evidenced as drafted). [cite:harbor-0029]
- ☐ **Re-baseline DR/lag runbooks to the 7-day replay window** — note: the proposed 24h lag alert / 3-day escalation guardrails are **unapproved**; operating without them risks silently consuming the 7-day window. [cite:harbor-0021]

## Why Kafka is not a committed dependency

The user explicitly decided "no Kafka adoption is approved" and the PostgreSQL outbox stays [cite:harbor-0022]. The earlier design decision already chose PostgreSQL outbox consumers for v1 [cite:harbor-0011], and the baseline shows a three-engineer team with no Kafka expertise [cite:harbor-0004]. Contention fallbacks were deliberately specified to stay on PostgreSQL for this reason [cite:harbor-0028]. Kafka was only ever an assistant-deferred decision gate, never user-approved [cite:harbor-0014].

## Why storage still needs validation

- Tool-measured average load produces 51.84 GB/day of raw payload; at 7-day Legal-approved retention that is ~338 GiB (assistant arithmetic), against only **100 GiB free** on existing PostgreSQL — a nominal ~238 GiB shortfall before indexes, WAL, and metadata, which the tool excludes. [cite:harbor-0020] [cite:harbor-0021] [cite:harbor-0004]
- The remediation (~500 GiB EU-region expansion, or raw payloads to EU object storage with metadata/pointers in PostgreSQL) is **recommended, not approved**. [cite:harbor-0021]
- Critical coupling: the outbox write is in the same transaction as the business change, so disk exhaustion halts business writes, not just event delivery. [cite:harbor-0012]

## Why the EUR 900 budget still needs validation

- **No vendor price quotes exist** for anything — this was true in the baseline and remains true; the user directed that no measured cost be claimed. [cite:harbor-0004] [cite:harbor-0011]
- Scope grew after the ceiling was set: nine consumer replicas (up from six) plus the ~500 GiB storage expansion are both **uncosted** against the EUR 900/month ceiling. [cite:harbor-0028]
- The user made "an actual cost estimate under EUR 900" an explicit launch gate precisely because feasibility is currently unverifiable. [cite:harbor-0029]

**Bottom line:** consumer count is conditional on contention validation, storage is a quantified but unremediated blocker, and budget compliance is unproven — all three must close before the 1 December 2026 launch can be called "go." [cite:harbor-0015]
