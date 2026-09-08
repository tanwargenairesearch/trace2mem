# Trace2Mem visuals

Inspired by [Brain: Agentic Memory as a Knowledge Wiki, by Perplexity](https://www.perplexity.ai/hub/blog/brain-agentic-memory-as-a-knowledge-wiki). These are original Trace2Mem diagrams and measurements; they do not represent Perplexity benchmark results or endorsement.

| Visual | PNG for sharing | SVG for editing | PDF for print |
|---|---|---|---|
| Architecture | [PNG](assets/trace2mem-architecture.png) | [SVG](assets/trace2mem-architecture.svg) | [PDF](assets/trace2mem-architecture.pdf) |
| Performance | [PNG](assets/trace2mem-performance.png) | [SVG](assets/trace2mem-performance.svg) | [PDF](assets/trace2mem-performance.pdf) |

The architecture groups foreground use, durable memory, and Dream maintenance. Dependency labels are cloud-neutral. API authentication mediates capture and retrieval; agents do not write storage directly. All three layers can be inspected; the wiki arrow illustrates published memory access, not a wiki-only access restriction. Citation edges and detailed network routes are omitted.

The performance chart reads the committed held-out summary directly. It compares full history, notes plus sessions, and the full wiki on the same two synthetic personas, eight questions each, and two repeats. This is 32 attempts per condition, including failures. The agent used Kimi through OpenRouter; compilation embeddings used Vertex. Retrieval used exported files and bounded keyword matching, not live semantic search. Exact-task scores are strict JSON/expected-value checks, not a semantic quality judge. The small observed gain includes execution and formatting differences.

Token totals and answer latency cover foreground inference; snapshot preparation and compilation are excluded. Across all retained development and held-out compilation attempts, 402,516 tokens were recorded, including estimated embeddings. This total is not allocated to any chart condition. No statistical significance, monetary savings, or broad semantic improvement is claimed.

## Reproduce

With Python and Matplotlib 3.10.8 installed, run from the repository root:

```sh
MPLCONFIGDIR=/tmp/trace2mem-matplotlib python3 scripts/render_public_visuals.py
```

| Source | What it supports |
|---|---|
| [Architecture](ARCHITECTURE.md) | Components, three layers, access paths and publication |
| [Integration guide](INTEGRATION.md) | Capture, initial index and progressive retrieval |
| [Held-out summary](../reports/2026-09-08-persona-evaluation/evaluation/heldout-summary.json) | Chart values and calculated deltas |
| [Evaluation report](../reports/2026-09-08-persona-evaluation/README.md) | Protocol, compilation accounting and limitations |
| [Failure audit](../reports/2026-09-08-persona-evaluation/RCA.md) | Interpretation of the score differences |
