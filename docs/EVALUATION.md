# Reproducible evaluation

The evaluation API compares the same questions against one pinned revision, with knowledge pages enabled and disabled. Notes and sessions remain available in both conditions. It rejects configuration changes across the run. Candidate generation uses development results, evaluates heldout cases, and requires explicit operator promotion against the tested revision/configuration. Promotion is blocked during a pending embedding transition.

POST `/api/evaluate` with a management token:

```json
{
  "cases": [{
    "query": "When does Project Alder launch?",
    "split": "heldout",
    "facts": [{
      "id": "current-deadline",
      "answer_pattern": "(?i)launches in October",
      "evidence_pattern": "(?i)launches in October",
      "citations": ["launch-correction"],
      "forbidden_patterns": ["(?i)launches in September"]
    }]
  }]
}
```

Patterns use Go RE2 syntax. Each expected fact must match the answer, have a supporting cited message/tool-result matching its evidence pattern, and attach that citation on the same line as the assertion. Forbidden patterns reject specified stale/incorrect assertions. Source IDs must occur in the pinned revision. The rubric handles message/tool-result evidence; artifact-only claim evaluation needs a separate rubric extension.

This is a lexical rubric, not semantic proof. Loose patterns can accept misleading wording and strict ones can miss correct paraphrases. `answer_citation_recall` measures expected source IDs printed in the answer, not total retrieved evidence or an independent judge's support assessment. Optional external judge assessments should be stored separately from deterministic scores.

Reports retain the questions, rubrics, source text, model configuration, revision/watermark, retrieval turns/tools, semantic status, latency and usage. A rejected answer or provider failure becomes a failed result with an error and its recorded usage; it does not silently disappear from the pair. Configuration changes, forgetting and whole-run cancellation still invalidate the report. They contain private memory content: access follows the user's management permissions, and forgetting removes stored reports. Do not publish reports from real user histories without their authorization.

Generation and embedding usage are counted per request. Failed dispatched calls retain conservative unresolved reservations, labelled estimated. Embedding tokens are estimated. Optional `prices` contains `generation_input_per_million_usd`, `generation_output_per_million_usd`, and `embedding_input_per_million_usd`; dollar fields are null without operator-supplied rates. Prices produce estimates, not provider invoices. Recorded compilation usage is cumulative metered work before the evaluation snapshot, including retries; it excludes compilation before operation tagging was introduced and is not allocated uniquely to a revision. Mixed historical model prices require separate billing analysis. Optimizer invocation and its expense are recorded separately from heldout retrieval.

## Synthetic live-model fixture

`tests/live/memory_test.go` imports a project deadline and language preference, compiles memory, runs paired development/heldout questions, imports a correction, evaluates again, then checks immediate forgetting suppression. It saves exact synthetic histories and both reports. It records poor answers as results; passing the harness is not a quality threshold.

Use a **disposable PostgreSQL database with vector support and no other queued jobs**, reachable from the test process. Provide an explicit YAML model configuration and credentials in the process environment. Set absolute paths because Go tests change their working directory:

```sh
export TRACE2MEM_LIVE_DATABASE='postgres://user:password@localhost:5432/disposable?sslmode=disable'
export TRACE2MEM_LIVE_CONFIG="$PWD/models.yaml"
export TRACE2MEM_LIVE_REPORT_DIR="$PWD/.local/evaluation"
go test -v ./tests/live -run TestMemoryAblation -count=1
```

Review the YAML's `max_steps`, `max_tokens`, and `daily_tokens` first. The test's memory and budgets are isolated per invocation; a failed test may leave a leased job in its disposable database. Recreate that test database before retrying. Host ADC works only when the test process can access it. No real provider is enabled by default.

## Recorded results

Deterministic Docker checks and the real-model synthetic scenario passed on 2026-09-08. Generation: `moonshotai/kimi-k3` through OpenRouter Responses. Embeddings: Vertex `gemini-embedding-001`, 768 dimensions, `us-central1`, using ADC. Limits: 12 Dream steps, 32,000 tokens per compilation, 500,000 daily tokens for the isolated test identity. The complete test took 169.978 seconds, including two compilations, eight retrievals and forgetting checks.

| Snapshot / condition | Rubric passes | Sum retrieval latency (2 questions) | Accounted retrieval tokens |
|---|---:|---:|---:|
| Initial, wiki | 2/2 | 14.110 s | 5,905 |
| Initial, notes + sessions | 2/2 | 10.548 s | 4,376 |
| Corrected, wiki | 2/2 | 11.027 s | 5,410 |
| Corrected, notes + sessions | 2/2 | 13.327 s | 4,031 |

Both conditions scored 4/4 overall with expected answer-citation recall of 1.0; no wiki quality advantage was demonstrated. Both changed the deadline from September to October. Two compilations used 16,727 reported generation tokens and 13,764 estimated embedding tokens cumulatively. Foreground retrieval used 19,722 accounted tokens including estimated embeddings. Dollar fields are null because no operator price schedule was supplied; these totals are not a provider invoice. The wiki used more retrieval tokens in this tiny fixture and latency varied, so no cost or speed saving is established.

Artifacts: [initial report](../reports/2026-09-08-initial.json), [corrected report](../reports/2026-09-08-corrected.json), [replayable synthetic history](../reports/2026-09-08-history.jsonl). The local project identifier is redacted; model IDs, region, budgets, prompts, revisions, outputs and metrics are preserved. The source corresponds to implementation commit `00a945c`; the subsequent history-export formatting change does not change the evaluated scenario.

There are only two questions per revision and one sample per condition. The deadline question's wording about the “current month” elicited date inferences from event timestamps; the rubric does not assess all extra assertions. No independent semantic judge was run, and the heldout language question is a heldout question within the same small history, not a separate unseen corpus. Earlier development runs found missing structured-response instructions, invalid session-citation placement and reasoning text mixed into final answers; the implementation was corrected and this final run repeated. Failed development attempts are not included in the final-run token totals. These results justify continued experimentation, not production-quality or Brain-equivalence claims.

### Filesystem working-set measurement

On 2026-09-08, Apple M4 Pro / macOS arm64 / Go 1.26.6, `go test ./filesystem -run '^$' -bench BenchmarkWorkingSetSearch -benchtime=3x -count=1` searched 100 synthetic pages of approximately 1 KiB each:

| Path | Time per whole-corpus search |
|---|---:|
| Ordinary directory, OS cache warm | 1.108 ms |
| Verified memory cache, warm | 1.299 ms |
| New memory cache over loopback HTTP | 39.409 ms |

These three iterations are a small reproducible baseline, not a statistically robust benchmark or a kernel FUSE comparison. The cache was slower than the ordinary directory here. The cold result includes manifest/download/hash/write overhead and does not model WAN latency. No earlier comparable baseline exists, so no regression rate is claimed. Linux FUSE is separately smoke-tested; native macOS FUSE is not covered by this benchmark.

## Next evaluation scope

The [outcome measurement plan](OUTCOMES.md) separates external-agent integration gains from the wiki-only ablation, with matched baselines, task-level checks, and complete cost accounting. The broader deliverable/long-history/multi-harness experiments remain proposed work. The narrower completed user-history pilot is recorded below.

## Implemented repeated comparisons

`POST /api/evaluate` accepts optional `repeats` (1–5; omission means one). Order alternates by case and repeat instead of always executing wiki first. Reports include repeat numbers and `retrieved_evidence_recall`, which counts expected IDs actually returned by successful source-resolution tools, separately from citation IDs printed in the answer. The route retains its five-minute whole-run deadline; split expensive repeated suites into smaller requests.

Expected facts additionally support `status_pattern` and `attribution_pattern`, required on the same answer line as a supporting citation when supplied. Optional `forbid_negation:true` conservatively rejects English negation words on that assertion line. This can reject correct sentences containing unrelated negation; use precise case-specific rubrics and structured task artifacts. These are lexical controls, not a semantic correctness guarantee.

The [external-agent runner](../evaluation/README.md) implements three-condition process orchestration and exact structured-artifact checks. Its tests are deterministic. The small live Harbor pilot below now records matched directory-agent outcomes. Broad generalization and independently assessed semantic/cost gains remain unestablished; the subsequent persona pilot below records a narrow positive task-score comparison.


## Memory-dependent agent pilot — 2026-09-08

The [Harbor comparison report](../reports/2026-09-08-harbor-comparison/README.md) adds six structured tasks × three memory conditions × two repeats. The full-history baseline passed 11/12 tasks, notes/sessions 3/12, and full wiki 0/12. Expected fields and configuration were committed before calls. In 10/12 wiki trials the agent did not invoke a memory tool; malformed outputs and one conservative-budget failure also occurred. This is an unfavorable consuming-agent result, not a claim that the wiki's content is worse when retrieved. No positive quality/cost delta is established.

The 240,454 reported foreground tokens cover all 36 trials; compilation of the reused snapshot is additional historical work. No new embedding or Dream calls occurred. The report includes tables, a chart, artifacts, traces and specific scoring/accounting limits. The [protocol](../evaluation/harbor/README.md) uses an explicit Kimi model and hash-verified directory snapshot, not live service search, and is a development pilot over one known history. New independent histories and a reliable retrieval/finalization protocol are needed before generalizing.


## User-history dataset and RCA hill-climb — 2026-09-08

The [dataset](../evaluation/personas/README.md) has four authored synthetic users, six conversations and 18 message events per user, and eight questions per user. Real Kimi/Vertex Dream compilation produced all four memories; the failed initial Nadia attempt and schema-repair retry are retained. This compiles the six conversations as one batch, not an incremental freshness test.

Two development candidates completed 48 trials each. Requiring explicit actions, memory reads and structured answer submission improved combined notes/wiki task success from 14/32 to 21/32. Selection was committed before the selected candidate ran 96 held-out trials over two other users.

| Held-out condition | Exact tasks | Foreground tokens | Median latency |
|---|---:|---:|---:|
| Full history | 23/32 | 277,483 | 17.85 s |
| Notes + sessions | 26/32 | 504,726 | 19.83 s |
| Full wiki | 27/32 | 380,018 | 14.96 s |

Full wiki was +3.1 percentage points and used 24.7% fewer foreground tokens than notes/sessions; versus full history it was +12.5 points and used 37% more tokens. The one-task wiki/notes gap corresponds to one fewer execution failure. All five non-execution failures across conditions were punctuation or additional-detail mismatches. Exact-value task scores are separate from gold-citation agreement; no semantic improvement or monetary saving is established.

All 192 foreground trials accounted for 1,973,310 tokens, plus 402,516 compilation tokens including estimated embeddings and the failed attempt. The shared-template pilot has only two held-out personas, no independent semantic judge and no second harness. The [report and chart](../reports/2026-09-08-persona-evaluation/README.md), [RCA](../reports/2026-09-08-persona-evaluation/RCA.md), frozen suites, generated memories and shared traces retain the complete evidence. `evaluation/persona_verify.py` recomputes artifact scores and metrics offline; it does not prove inference execution or billing.
