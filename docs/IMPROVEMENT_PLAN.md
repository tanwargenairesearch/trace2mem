# Review-driven MVP implementation

Source review: [Brain-principles assessment](reviews/2026-09-08-brain-review.md). Keep the service cloud-neutral and maintain per-user isolation, bounded work, immutable publication, and honest evaluation claims.

1. **Capture and cancellation:** clone adapter envelopes; reject nil events; make per-content and cross-process cache waits cancellable. Gate: reused-event and stalled-reader regressions.
2. **Coherent corrections:** resolve supersession across the pinned revision, reject dangling/cyclic relationships, include all affected subjects. Gate: cross-subject correction and invalid graph tests.
3. **Progressive retrieval:** metadata-only manifests and targeted reads, bounded search results, index/read/evidence tools with real source inspection. Gate: pinned isolation/forgetting, large-page recovery, source-resolution tests.
4. **Dream investigation and verification:** explicit paginated prior search, bounded subject-neighborhood inspection, per-claim verification diagnostics for attribution/currentness. Gate: truncation, neighboring contradictions, approval versus assistant recap regressions.
5. **Evaluation:** counterbalanced repeated pairs, explicit decision-status and negation scoring, external-agent baseline runner with frozen tasks/configuration and recorded failures. Gate: deterministic scoring and matched-condition tests; live-model quality deltas remain opt-in and unproven until measured.

Each slice receives focused tests and the production/design/principles review before a local commit. Existing synthetic artifacts remain unchanged. Cloud deployment and public posting are outside this work. Broad paid experiments are not run automatically.

## Progress

- [x] Capture and cancellation — reused-envelope and cancellation race tests passed; all three review lenses cleared.
- [ ] Coherent corrections
- [ ] Progressive retrieval
- [ ] Dream investigation and verification
- [ ] Evaluation
