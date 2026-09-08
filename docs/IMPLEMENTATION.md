# Implementation and validation status

Trace2Mem is a self-hosted agent memory service inspired by Brain. The [alignment matrix](BRAIN_ALIGNMENT.md) distinguishes its principles from our engineering adaptations. This is not a production-readiness or equivalent-performance claim.

## Implemented

- One memory per authenticated user, inherited scoped agent credentials, issuer/subject identity mapping, and space-free HTTP/gRPC/MCP/CLI contracts.
- Durable idempotent event ingestion with original envelopes, tool relationships, source times, tombstones and per-user isolation.
- Actual session summaries, readable notes, synthesized subjects, explicit relationships, stable citations, incremental Dream investigation, staged verification and fenced publication/no-op.
- Automatic, daily and manual durable schedules; quiet-period cap, timezone/DST behavior, catch-up and missing-model blocking/recovery.
- Independent explicit generation/embedding configuration, encrypted credentials, endpoint approval, budgets and staged embedding reindex.
- Search, cited synthesis, initial index, pinned directory materialization and cached read-only Linux FUSE.
- LangChain callback capture, bounded persistent retry spool, initial context helper and live/saved-history examples. Pi/Hermes documentation defines the adapter contract only.
- Memory, Import, Activity, Models, Connections and Settings web screens, provider-neutral onboarding and an in-app integration guide.
- Pinned paired evaluation with fact/citation rubrics, per-request usage/transcripts and explicit operator candidate promotion.
- Docker, optional GCP Terraform, CI/release definitions and operational documentation.

## Recorded validation

2026-09-08, Apple Silicon host with Linux Docker:

- Go unit/race checks, Protobuf lint, and deterministic Docker lifecycle tests passed.
- HTTP/gRPC/MCP identity isolation with colliding IDs, concurrent first-use provisioning, correction/forgetting, pinned directory/offline reads and lease/budget regressions passed.
- Scheduling tests cover 60-second quiet period, five-minute cap, daily catch-up/DST, manual accepted-range boundaries, credential recovery and accurate session-close scheduling status.
- Chrome desktop/mobile navigation, missing-model Ask readiness, guide and saved scheduling settings passed.
- LangChain's five callback/spool tests and live/JSONL demonstrations passed in the preceding integration milestone.
- Terraform formatting, initialization/validation and one mock small-deployment test passed. No cloud apply was performed.
- Clean-checkout Docker and Linux FUSE acceptance passed for `00a945c`, using fresh disposable volumes and no private configuration.
- Real Kimi K3 / Vertex Gemini embeddings completed learning, correction, paired retrieval and immediate forgetting suppression. All eight small lexical rubric checks passed; both ablation conditions scored equally. See the measured report, including its limitations.

See [evaluation](EVALUATION.md) for the reproducible fixture, scoring limits, and real-model results. The deterministic model tests plumbing and cannot establish memory quality.

The [richer Kimi project-agent exercise](../reports/2026-09-08-harbor/README.md) published two revisions and demonstrated fresh-session index/read/evidence retrieval. Its saved-trace audit passes; manual inspection found an incorrect tentative-versus-approved historical interpretation that semantic verification missed. This is an observed quality gap, not merely an untested possibility.

## Review-driven improvements — 2026-09-08

Five reviewed implementation slices now preserve reused SDK event envelopes, cancel blocked cache waits, validate cross-subject corrections, provide progressive index/search/read/evidence tools, and paginate Dream investigation. Publication requires complete per-passage support, attribution and temporal judgments, with bounded neighboring subject evidence. Judgment remains fallible: the Harbor real-model historical error has not been replayed against this change.

Evaluation supports repeated counterbalanced pairs and resolved-source recall. The [external-agent benchmark](../evaluation/README.md) adds three matched conditions, explicit artifact expectations, bounded subprocess execution and failure records. The subsequent [Harbor directory-agent pilot](../reports/2026-09-08-harbor-comparison/README.md) performed 36 live trials over the older export and exposed tool-use failures: full history 11/12 tasks, notes/sessions 3/12, wiki 0/12. No quality or cost advantage was demonstrated; the new Dream verifier was not exercised by that pilot.

Fresh-checkout Docker integration/Dream and Linux FUSE acceptance passed at `11df64e`. Go vet, ordinary race tests, focused database race tests and five Python benchmark tests passed. These checks establish implementation behavior, not semantic quality or cloud readiness.

## User-history evaluation — 2026-09-08

A [four-persona dataset and bounded hill-climb](../reports/2026-09-08-persona-evaluation/README.md) now record 192 live Kimi foreground trials and all five Dream compilation attempts. The controlled demonstration-agent protocol improved development memory-task completion from 14/32 to 21/32. Held-out scores were history 23/32, notes/sessions 26/32, wiki 27/32. The wiki used fewer foreground tokens than notes/sessions but more than history. The [failure audit](../reports/2026-09-08-persona-evaluation/RCA.md) finds execution and exact-format differences; semantic-quality gains remain unproven.

The production verifier now constrains evidence-ID fields to supplied IDs. The stricter consuming-agent action protocol is an example, not automatically imposed on every API/MCP client. Dataset validation, resumable checkpoints, frozen candidate selection, complete accounting and offline score verification are implemented. The normal application and historical reports were preserved.

## Remaining limitations

External connector investigation and Dream subagent delegation are deferred. Native macOS FUSE, external OIDC/OAuth interoperability, cloud IAM/private database access, cloud restore, release publication, and public CI execution are not verified by local Docker. No GitHub or LinkedIn publication has been performed.

Foreground manifests load metadata only; file reads target a pinned path and search retrieves bounded candidates. Full snapshots used by background/export/evaluation paths still load corpus pages into memory. Composition has explicit bounded inputs rather than unbounded scaling. Large-corpus performance, broad contradiction cases, exhaustive crash/failure injection, and independent semantic evaluation need further evidence. Lexical rubric scores can miss paraphrases or accept misleading phrasing; they are not semantic proof. Embedding token usage is estimated.

The development application is at `http://localhost:18787`. Use `make bootstrap-token` for its current credential. Old `brain_*` volumes were preserved; fresh per-user `trace2mem_user_*` volumes hold current development data. No private model keys or ADC credentials are tracked.
