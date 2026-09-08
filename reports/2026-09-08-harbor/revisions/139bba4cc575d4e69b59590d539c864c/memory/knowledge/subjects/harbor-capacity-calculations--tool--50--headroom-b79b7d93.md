# Harbor capacity calculations (tool, 50% headroom)

## Harbor capacity calculations (tool, 50% headroom)

**Status: current, tool-measured** [cite:harbor-0007] [cite:harbor-0009].

The `capacity` tool was run twice with 1.5x headroom [cite:harbor-0006] [cite:harbor-0008]:

- **Average load (400 events/s):** 51,840,000,000 raw bytes/day (~48.3 GiB/day); 1 consumer replica required [cite:harbor-0007].
- **Peak load (2,400 events/s):** 311,040,000,000 raw bytes/day equivalent; 6 consumer replicas required (6 x 600 events/s = 3,600 events/s, matching the 50%-headroom target) [cite:harbor-0009] [cite:harbor-0004].

**Tool-stated assumption:** constant supplied rate for 24 hours; figures exclude indexes, replicas, compression, and metadata [cite:harbor-0007] [cite:harbor-0009]. Because actual peaks last only 20 minutes, real daily volume stays near the average figure, as the assistant noted [cite:harbor-0010].

**Derived implication (assistant analysis):** 30-day raw-payload retention would require ~1.45 TiB minimum against only 100 GiB free, making PostgreSQL-only raw retention infeasible; at average rate, 100 GiB buffers roughly two days of traffic [cite:harbor-0010] [cite:harbor-0012] [cite:harbor-0004]. The user incorporated this retention risk into the v1 decision [cite:harbor-0011].

- [[knowledge/subjects/harbor-system-baseline--tool-inspection-e2624996.md]]
- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]