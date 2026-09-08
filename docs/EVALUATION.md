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

Deterministic Docker paired evaluation and fact/citation regression checks passed on 2026-09-08. A real OpenRouter Kimi K3 / Vertex Gemini embedding run is being exercised; no quality improvement, latency gain or cost saving is established. Final results and failures are recorded in the delivery log when that run finishes.

### Filesystem working-set measurement

On 2026-09-08, Apple M4 Pro / macOS arm64 / Go 1.26.6, `go test ./filesystem -run '^$' -bench BenchmarkWorkingSetSearch -benchtime=3x -count=1` searched 100 synthetic pages of approximately 1 KiB each:

| Path | Time per whole-corpus search |
|---|---:|
| Ordinary directory, OS cache warm | 1.108 ms |
| Verified memory cache, warm | 1.299 ms |
| New memory cache over loopback HTTP | 39.409 ms |

These three iterations are a small reproducible baseline, not a statistically robust benchmark or a kernel FUSE comparison. The cache was slower than the ordinary directory here. The cold result includes manifest/download/hash/write overhead and does not model WAN latency. No earlier comparable baseline exists, so no regression rate is claimed. Linux FUSE is separately smoke-tested; native macOS FUSE is not covered by this benchmark.
