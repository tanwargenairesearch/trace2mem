# Harbor capacity calculations (tool, 50% headroom)

# Harbor capacity calculations (tool, 50% headroom)

## Harbor capacity calculations (tool, 50% headroom)

**Status: current, tool-measured** [cite:harbor-0020] [cite:harbor-0025] [cite:harbor-0027].

The `capacity` tool returned the following at the supplied rates and headroom factors:

- **400 events/s (average), headroom 1:** 51,840,000,000 raw bytes/day; 1 replica [cite:harbor-0020].
- **3,600 events/s (new peak), headroom 1.5:** 466,560,000,000 raw bytes/day if sustained; 9 replicas — matching 5,400 events/s ÷ 600 events/s measured per replica [cite:harbor-0025].
- **400 events/s (average), headroom 1.5:** 51,840,000,000 raw bytes/day; 1 replica [cite:harbor-0027].

**Tool-stated assumption (all runs):** constant supplied rate for 24 hours; excludes indexes, replicas, compression, and metadata [cite:harbor-0020] [cite:harbor-0025] [cite:harbor-0027].

**Derived figures (assistant arithmetic, not tool output):** at 7-day retention, average load implies ~362.88 GB (~338 GiB) raw payload, down ~76.7% from the superseded 30-day figure (~1,448 GiB), but still above the 100 GiB free on existing PostgreSQL, leaving a nominal ~238 GiB gap before indexes/WAL/metadata [cite:harbor-0021]. Because the historical peak profile is bursty (~20 minutes), each 3,600/s peak episode adds only ~5.4 GB, so the ~338 GiB 7-day estimate stands [cite:harbor-0028].

**Earlier superseded runs:** at 2,400 events/s with 1.5 headroom the tool returned 311,040,000,000 bytes/day and 6 replicas [cite:harbor-0009]; that peak value was later replaced by the user with 3,600 events/s [cite:harbor-0022].

Related: Harbor system baseline (tool inspection) — source of the 600 events/s-per-replica and payload measurements feeding these runs; Harbor v1 architecture decision — the user decision that adopted nine consumers on the basis of these results.

- [[knowledge/subjects/harbor-system-baseline--tool-inspection-e2624996.md]]
- [[knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md]]