# Kimi learns and recalls Project Harbor

A real Kimi K3 tool loop planned a fictional webhook-platform migration, evaluated capacity and architecture choices, then incorporated revised requirements. A fresh conversation used Trace2Mem tools to produce a [handoff](handoff.md) and [operational checklist](checklist.md). This is a synthetic integration exercise, not a production deployment or a general quality benchmark.

## What happened

The preserved planning checkpoint contains 29 events across two conversations: six user messages, nine assistant messages, and seven tool calls with seven paired results. Kimi inspected a fictional system fixture twice and invoked an actual capacity calculator five times. The checkpoint ends with the user's memo directive; the original final assistant response was not preserved after an early runner interruption. The raw history is therefore a replay checkpoint, not a claim of lossless original-run capture.

Dream published the original design at watermark 14, then corrections at watermark 29. The final export contains 53 files: eight subject pages plus index/changelog, 12 Markdown notes, two session summaries, and 29 evidence records. Both [revision snapshots](revisions/) are retained; every exported file matches its manifest SHA-256 and byte size.

The new conversation started without the planning messages. It generated 50 additional captured events, making 79 total. Across nine generation calls it successfully used memory_index once, memory_read eight times, and memory_evidence 14 times, resolving 14 distinct sources. It did not need memory_search because the index identified the relevant paths. The exported revision predates this fresh conversation; its events remain pending for future compilation.

The handoff recalled the corrected December 1, 2026 launch, seven-day raw retention and 90-day metadata retention, peak 3,600 events/second, nine consumers with 50% headroom, PostgreSQL outbox, owners Mira/Leon/Saanvi, and 120/300-second operational thresholds. Storage expansion/offload and actual cost remain unresolved; EUR900 is a budget ceiling, not a cost estimate.

## Observed limitation

The [architecture subject](memory/knowledge/subjects/harbor-v1-architecture-decision-5d09681b.md) and handoff incorrectly describe the initial v1 decisions as tentative. User event harbor-0011 already approved them. A later assistant recap, harbor-0028, introduced this historical interpretation and the semantic verifier accepted it. Current-state facts were recalled, but source precedence and temporal interpretation still need stronger verification. Generated pages also contain repeated headings. Artifacts are preserved unchanged, including these defects; neither model verification nor term-presence checks establish perfect factual support.

## Verification and accounting

The successful replay completed publication and recall in 400.17 seconds. Its original test command exited with a failure solely because the harness required a search call. The harness now allows index-directed reading, and the offline saved-trace audit passes: tool calls/results pair, cited evidence was actually resolved, the saved handoff matches a captured assistant message, and the narrow fact smoke checks pass. This is not represented as a passing original live invocation, nor as a wiki-enabled/wiki-disabled comparison.

| Successful replay usage | Input | Output | Accounting |
|---|---:|---:|---|
| Fresh agent generation | 60,187 | 3,266 | Reported |
| Compilation generation | 87,159 | 13,443 | Reported |
| Compilation embeddings | 240,044 | 0 | Conservative estimate |

These numbers exclude original planning generation and earlier failed attempts. They are not total experiment usage or an invoice. No monetary total is claimed. Compilation/recall used moonshotai/kimi-k3 through OpenRouter, requested reasoning effort none, 8,192 output tokens and 180-second request timeout; embeddings used Vertex gemini-embedding-001, 768 dimensions. Original planning used the provider's default reasoning setting. The [example profile](../../configs/models.kimi-agent.example.yaml) contains no credentials or private project ID.

## Reproduce or inspect

Follow [the runner guide](../../docs/KIMI_AGENT.md) for an opt-in live run with your credentials and budgets. The exercise uses a disposable isolated user/database; fictional decisions are not added to the normal web-app user's memory.

- [Raw event envelopes](history.jsonl), [transcript](transcript.md), [memory index](memory/knowledge/index.md), [manifest](manifest.json).
- [Agent usage](agent-usage.json), [service usage](service-usage.json), [staged proposals and verifier judgments](proposals.json).
- The checklist is the final assistant message extracted verbatim from the captured history.

```sh
TRACE2MEM_AGENT_REPORT="$PWD/reports/2026-09-08-harbor" go test -v ./tests/live -run TestSavedHandoff -count=1
```
