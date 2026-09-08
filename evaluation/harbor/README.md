# Harbor memory-dependent task pilot

This is a development demonstration of a custom Go agent consuming Trace2Mem's pinned directory export. It tests six tasks across three conditions, twice each (36 fresh agent processes). It uses the existing synthetic Harbor checkpoint: 29 events across two conversations. It does not recompile memory or measure the latest Dream verifier. The known incorrect historical interpretation in that snapshot remains intact.

## Frozen conditions and scoring

- **existing_memory:** the complete original event envelopes present in the pinned revision, supplied in the prompt, ordered by source sequence. This is a full-history baseline, not a no-memory baseline. Later recall/handoff events are excluded.
- **notes_sessions:** an initial list of permitted notes and summaries; bounded keyword search and file reads over notes, summaries and original evidence. Knowledge files are inaccessible.
- **trace2mem:** the wiki index, with the same keyword search and file reads, adding knowledge pages. The agent can resolve original evidence and paginate reads.

The agent reads hash-verified local exports through Go-managed tools. This demonstrates directory integration, not live HTTP/MCP performance or semantic search. Snapshot loading is included in process latency. Initial orientation is injected once; it is not counted as a model tool call. No filesystem writes or shell tools are exposed to the model.

[The evidence ledger](evidence-ledger.json) records source IDs, authority, temporal status and known conflicts independently of the agent input. [Model settings](model-settings.json) freeze the secret-free generation configuration; mismatches fail before any provider call.

[suite.json](suite.json) freezes questions and exact expected artifact fields before live calls. Expected values are based on original sources: harbor-0004, 0011, 0013, 0015, 0022 and 0029. In particular, harbor-0011 already approves the outbox design; a later assistant recap must not change that historical status. Question wording names required output fields and allowed status vocabulary, but does not provide their expected values. This is intentionally a guided development pilot. Generalization requires independent histories and unprompted tasks.

Tasks cover cross-session configuration, temporal/authority history, operations, ownership, unknown costs/proposals, and a single-fact control. Exact artifact checks do not establish support for extra statements or the validity of all citations. Per-field accuracy and all-required-fields task success are reported separately. Missing values, malformed output, budget exhaustion and provider errors count as failures.

The generation model and prompt are identical across conditions. Condition order rotates by case/repeat. Every trial has a 64,000 accounted-token budget, conservative byte-based request preflight, at most 10 model rounds, 16 calls per round, and 150 seconds inside the agent. The outer runner timeout is 180 seconds. Provider accounting is recorded; unknown failed-call usage is conservatively reserved. No embedding calls are made. Monetary cost is unavailable without a price schedule; historical compilation usage is reported separately, not silently counted as free.

## Reproduce

Credentials remain in the environment. Set `TRACE2MEM_LIVE_CONFIG` to an absolute provider-neutral model YAML path with generation model `moonshotai/kimi-k3`, explicit endpoint, reasoning effort `none`, and its API-key environment reference. The existing [example profile](../../configs/models.kimi-agent.example.yaml) describes these settings. Embedding configuration is parsed but not invoked by this directory agent.

```sh
go build -o /tmp/trace2mem-memory-agent ./evaluation/cmd/memory-agent
export TRACE2MEM_AGENT_SNAPSHOT="$PWD/reports/2026-09-08-harbor"
python3 evaluation/agent_benchmark.py \
  --suite evaluation/harbor/suite.json \
  --output "$PWD/.local/harbor-comparison.json" \
  --repeats 2 --timeout 180 --agent /tmp/trace2mem-memory-agent
```

Do not overwrite an existing report. The saved report contains full synthetic sources/model traces; reports for your own histories must remain private. No real-user data is used here. The model identifier is explicit; an upstream provider may still revise its weights or routing.

## Delivery tracking

- [x] Freeze tasks and expected fields before live calls.
- [x] Implement directory agent with baseline/condition isolation and hash validation tests.
- [x] Production/design/principles review: fixed hidden retries, missing usage, configuration drift, failure classification and source-ledger gaps; moved direct Python test entry point after all test definitions.
- [x] Execute 36 live trials and retain failures: full history 11/12, notes/sessions 3/12, wiki 0/12.
- [x] Generate per-condition/task tables and delta chart; document limits. Results/report review recorded with delivery.


## Recorded pilot

[Results, chart and traces](../../reports/2026-09-08-harbor-comparison/README.md) show a negative result: Kimi frequently did not invoke memory tools. The full-history baseline outperformed both file-memory conditions. This is an agent integration failure to investigate, not a positive memory-quality claim. Lower token usage with failed tasks is not a cost benefit. Questions, gold fields and model configuration were committed at `72680c2` before trials; no favorable rerun replaces them.

To regenerate the report in a new directory (requires matplotlib):

```sh
MPLCONFIGDIR=/tmp/trace2mem-matplotlib python3 evaluation/report_agent_benchmark.py \
  --input "$PWD/.local/harbor-comparison.json" \
  --output "$PWD/.local/harbor-comparison-rendered"
```

This historical renderer accepts only the recorded report or its shared redacted copy, so its fixed qualitative commentary cannot be applied to unrelated reruns. Its summary function can be reused for new reports. The renderer verifies the complete 36-trial matrix, retains failed field checks in the denominator, separates missing/estimated usage, and redacts opaque provider continuation handles. Exact source report hashes are retained. Tests use no paid models.
