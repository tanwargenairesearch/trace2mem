# Harbor unapproved recommendations and uncosted items

# Harbor unapproved recommendations and uncosted items

## Harbor unapproved recommendations and uncosted items

**Status: current, assistant-flagged as unapproved** — proposals and open cost questions, not accepted policy [cite:harbor-0021] [cite:harbor-0028].

**Unapproved recommendations (require explicit approval):**

- **Storage remediation:** provision ~500 GiB additional EU-region storage, or move raw payloads to EU object storage with PostgreSQL holding metadata plus pointers — the assistant judged the object-storage path likely cheaper under the EUR 900/month cap, but vendor quotes are still needed for either option [cite:harbor-0021]. This remains necessary because 7-day raw retention (~338 GiB) exceeds 100 GiB free [cite:harbor-0021].
- **Replay-window guardrails:** consumer-lag alerts at ~24 hours and hard escalation at ~3 days so the 7-day replay window is never silently consumed [cite:harbor-0021] [cite:harbor-0028].
- **Dead-letter archive:** a separate archive path for dead-lettered events that must outlive 7 days, since event_id dedup means re-delivery alone cannot recover them after expiry [cite:harbor-0021].
- **Contention-validation fallbacks:** if 9-poller outbox contention fails, batch-claim polling or a dispatcher/consumer split — both staying on PostgreSQL because Kafka is not approved [cite:harbor-0028].
- **Earlier unapproved items from the design session:** disk-based guardrails (warn <20% free; hard actions <10% free), a 7th replica for failure headroom (now moot at nine consumers), additional load-validation gates, and the Kafka decision gate [cite:harbor-0014] — none were approved by the change record or the memo directive [cite:harbor-0015] [cite:harbor-0022] [cite:harbor-0029].

**Uncosted items:** nine consumer replicas plus the ~500 GiB storage expansion against the EUR 900/month ceiling; no vendor quotes exist, so feasibility remains unverifiable and no cost figure may be stated as measured [cite:harbor-0028] [cite:harbor-0011]. The user's launch memo makes an actual cost estimate under EUR 900 an explicit launch gate [cite:harbor-0029].

Related: Harbor v1 architecture decision — the approved scope these items fall outside; Harbor launch review memo (user-finalized) — its unresolved-risk section and cost gate depend on these open items.

- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]
- [[knowledge/subjects/harbor-launch-review-memo--user-finalized-2eac219a.md]]
- [[knowledge/subjects/harbor-accepted-operational-policy-9143850a.md]]
- [[knowledge/subjects/harbor-implementation-sequence--assistant-proposal-a0f8d747.md]]