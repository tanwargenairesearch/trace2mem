# Harbor unapproved recommendations and uncosted items

## Harbor unapproved recommendations and uncosted items

**Status: current, assistant-flagged as unapproved** — these are proposals and open cost questions, explicitly not part of the accepted policy [cite:harbor-0014].

**Unapproved recommendations** (require explicit approval before action) [cite:harbor-0014]:

- **Raw-payload offload to EU object storage.** Motivated by the calculation that ~48.3 GiB/day average raw volume makes 30-day raw retention (~1.45 TiB minimum) infeasible against 100 GiB free [cite:harbor-0007] [cite:harbor-0004]. The accepted 300s producer pause bounds damage but does not solve steady-state retention [cite:harbor-0014].
- **Disk-based guardrails:** warn below 20% free; hard actions below 10% free (stop noncritical consumers, return 503 on ingest, disable lagging replication slots pinning WAL). The accepted policy uses age-based triggers only [cite:harbor-0014] [cite:harbor-0013].
- **7th consumer replica** to preserve 50% headroom during a single-replica loss (losing one of six leaves 3,000 events/s, above the 2,400 events/s peak but below the 3,600 events/s headroom target) [cite:harbor-0014] [cite:harbor-0012].
- **Additional pre-launch load-validation gates** beyond the two tests in the implementation sequence, such as fill-testing disk thresholds [cite:harbor-0014].
- **Kafka decision gate** — remains deferred; no action approved [cite:harbor-0014] [cite:harbor-0010].

**Uncosted items:** consumer-host infrastructure for 6-7 replicas, EU object-storage volume and egress, metadata-only PostgreSQL growth rate, and monitoring tooling [cite:harbor-0014] [cite:harbor-0012]. Feasibility within the EUR 900/month ceiling is unverifiable without vendor quotes, and per the user's decision no cost figures may be stated as measured [cite:harbor-0011] [cite:harbor-0004].

- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]
- [[knowledge/subjects/harbor-accepted-operational-policy-9143850a.md]]
- [[knowledge/subjects/harbor-implementation-sequence--assistant-proposal-a0f8d747.md]]