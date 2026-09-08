# Brain-principles review of Trace2Mem

Reviewed implementation: `bc00e31`. Perspective: the published Brain design, not its author's identity or endorsement. Read-only review of Dream, publication/retrieval, SDK/cache, evaluations, and preserved synthetic outputs. This is a targeted review, not a full security audit. Earlier passing tests remain valid; they did not cover all issues below.

**Verdict:** the architecture is a credible implementation inspired by Brain. The primary remaining work is reliable evidence interpretation and genuinely progressive investigation. The repository currently demonstrates cross-session memory, not an established improvement in task success or cost. Prioritize these gaps before expanding the product surface.

## production [3 findings]

1. **[sdk/client.go:45](../../sdk/client.go#L45) — Event reuse can lose captured history.** Import buffers the adapter's pointer. A temporary Go test overlay, using a local HTTP server and an adapter emitting `first` then `second` through one reused object, observed wire IDs `second, second`. Clone each event on capture; add a regression expecting both original envelopes. The bundled JSONL adapter allocates separately, so this concerns custom adapters.
2. **[internal/store/store.go:452](../../internal/store/store.go#L452) — Reading one file loads the entire revision.** Manifest, ReadFile and Search call Snapshot, which selects every page's content. This is confirmed code behavior; load impact was not benchmarked here. Introduce metadata-only manifests, path-specific reads, bounded database keyword search, and preserve identity/revision/forgetting checks. Measure database bytes and allocations as corpus size grows.
3. **[filesystem/cache.go:92](../../filesystem/cache.go#L92) — Waiting readers cannot cancel promptly.** Per-hash mutex acquisition ignores the waiting caller's context; a stalled first read blocks its followers. The later blocking file lock also holds the cache mutex. Use cancellable coalescing and lock acquisition. Add a stalled-download/canceled-follower regression. This failure path was inspected, not executed in this review.

## design [4 findings]

1. **[internal/dream/compose.go:179](../../internal/dream/compose.go#L179) — One whole-draft verdict missed false historical interpretation.** Harbor's wiki accepted an assistant recap describing already-approved decisions as tentative. Require per-claim source/status/conflict findings for decision transitions; distinguish an explicit decision from an agent's interpretation. Authority is claim-specific: user intent, observed system state, and assistant recommendations need different treatment. Keep unresolved claims disputed. Add Harbor's approval/recap conflict as a regression.
2. **[internal/dream/incremental.go:102](../../internal/dream/incremental.go#L102) — Corrections depend on exact subject identity.** Merge loads old observations only under subjects named by new observations. A differently named subject can supersede an old ID without loading and updating the original subject. This is a code-path risk, not an executed reproduction. Resolve supersession targets across the pinned user revision; reject dangling/cyclic links and stage every affected subject. Test renamed and cross-subject corrections.
3. **[internal/dream/incremental.go:30](../../internal/dream/incremental.go#L30) — Investigation silently stops at 64 matching notes.** The tool uses substring matching and returns neither a continuation nor a truncation signal. Add bounded pagination, subject reads, and relationship traversal. Verify changes against relevant unchanged neighbors, not just link-target existence. Test a contradictory observation beyond the first result page.
4. **[internal/server/memory.go:192](../../internal/server/memory.go#L192) — Built-in synthesis is search-only, not the demonstrated progressive reader.** It has no index/read/evidence tools; page citation IDs are counted as inspected without resolving their original events. Large search results abort above 128 KiB. Add bounded excerpts, pinned file reads, and explicit source resolution; distinguish “citation seen” from “source inspected.” The Kimi wrapper already demonstrates this workflow, but the service's GetContext does not yet implement it.

## principles [2 findings]

1. **[internal/server/evaluation_scoring.go:87](../../internal/server/evaluation_scoring.go#L87) — Lexical scores cannot establish support.** Matching a phrase and citation on one line may accept negation or misleading historical framing. Harbor demonstrates that expected terms can coexist with a false interpretation. Score current value, status, authority and supporting sources explicitly; retain lexical checks as smoke tests and report semantic/human judgments separately.
2. **[internal/server/evaluate.go:152](../../internal/server/evaluate.go#L152) — The current pair does not measure external-agent integration benefit.** Both conditions use service-generated memory and GetContext; wiki always runs first. Counterbalance conditions and repeat matched external-agent tasks over independent held-out histories. Separate integration benefit from wiki ablation; include failure/retry and amortized compilation expense.

## Triage and open actions

These are open recommendations, not completed fixes. Review findings remain unresolved; this review does not clear a release gate.

**Must-fix before a reliable-MVP release claim:**

- [ ] R1: clone captured adapter events and prove exact envelope preservation (production 1).
- [ ] R2: resolve and validate cross-subject supersession transactionally (design 2).
- [ ] R3: regress the observed authority/status failure and record claim-level verification outcomes (design 1). This narrows the failure mode; it cannot prove universal truthfulness.
- [ ] R4: make cache waits cancellation-aware before promoting FUSE reliability (production 3).

**Must-fix before publishing performance/quality-delta claims:**

- [ ] R5: implement matched external-agent conditions, counterbalanced repeats, and stronger task/claim scoring (principles 1–2).

**Next MVP improvements:** metadata/path-specific retrieval; service-side progressive tools; paginated Dream investigation and bounded neighborhood verification (production 2, design 3–4). These can remain documented experimental limits for a small-corpus preview.

**Defer:** more cloud integrations, external Dream connectors/subagents, richer console visualizations, and automatic optimization. They do not resolve the currently demonstrated failure modes.

The source separation, durable idempotent ingestion, per-user ownership, explicit model configuration, fenced publication, revision-pinned access, and honest reports are foundations to retain. PostgreSQL replacing Git as transactional coordinator is a defensible adaptation; reproducing Brain's internal infrastructure is not the goal.

See [the outcome measurement plan](../OUTCOMES.md) for what to show publicly and what remains unmeasured.
