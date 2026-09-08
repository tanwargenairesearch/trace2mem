# What changes when an agent uses Trace2Mem?

Trace2Mem's intended benefit is that an agent can complete later work using relevant prior decisions and evidence, with less repeated explanation. This benefit must be measured on the consuming agent's tasks. More pages, more citations, or more tool calls do not by themselves mean better memory.

## What we can show today

| Evidence | Honest conclusion |
|---|---|
| Harbor: 29 planning events, two revisions, 53 exported memory files | A substantial synthetic trajectory was compiled incrementally |
| Fresh Kimi session: eight file reads and 14 resolved sources | A real agent progressively accessed memory without the planning messages |
| Corrected deadline, retention, capacity, and owners recalled | Concrete cross-session recall capability; there was no matched baseline for this exercise |
| One false “tentative → approved” historical interpretation | Current semantic verification can accept an unsupported interpretation |
| Tiny wiki ablation: 4/4 rubric passes in each condition | No measured correctness advantage |
| Wiki retrieval: 11,315 accounted tokens; notes/sessions: 8,407 | About 34.6% more foreground tokens in this tiny fixture; no saving demonstrated |

The last comparison covers four question/revision pairs, one sample each, and includes estimated embedding tokens. It is not a general overhead estimate. Source artifacts and limitations are in [Evaluation](EVALUATION.md) and the [Harbor report](../reports/2026-09-08-harbor/README.md). Compilation cost is additional. Harbor usage excludes original planning generation and earlier failed attempts, so do not publish it as total experiment cost.

## Latest memory-dependent task pilot

The [36-trial Harbor comparison](../reports/2026-09-08-harbor-comparison/README.md) used a custom Kimi agent and the existing pinned directory export. Exact-task success was **11/12 for full original history, 3/12 for notes/sessions, and 0/12 for full Trace2Mem**. The wiki agent made no memory-tool call in 10/12 trials. This demonstrates an integration reliability gap; it cannot establish the value of the wiki after successful evidence retrieval. Lower tokens or latency for failed work are not savings.

The six tasks and gold fields were frozen before calls. They cover configuration, temporal/authority history, operations, owners, unknowns and a single-fact control. All use one known synthetic two-session history and two repeats. No new Dream compilation or held-out generalization was measured.

## Measure two different questions

**Integration delta:** same external agent, model, tools, tasks and foreground budget, using its existing memory mechanism versus Trace2Mem. Specify the existing mechanism precisely: fresh conversation, rolling summary, full history within a fixed context limit, or an existing retrieval system. Do not silently choose a weak baseline.

**Wiki delta:** same Trace2Mem pipeline and agent, with the knowledge wiki available versus withheld. Keep notes, session summaries, raw evidence and non-memory tools available in both. Ensure the baseline receives no wiki index or cached wiki content. This isolates the wiki, as in Brain's published design.

Start with three conditions: the chosen existing-agent baseline, notes/sessions retrieval, and full Trace2Mem. If the existing baseline is already raw-history retrieval, retain it as a separately described condition rather than conflating it with distilled notes. An additional no-memory condition can illustrate basic persistence, but cannot establish superiority over other memory systems.

## A practical benchmark

Use fictional independent users with overlapping project names to exercise isolation. Each history spans at least six sessions, with irrelevant chatter, tool observations, proposals that were never accepted, changed decisions, and explicit unknowns. Freeze an authoritative fact/decision ledger with value, valid time, actor authority, status, source event IDs and expected conflicts. Derive ground truth from authored sources, not the generated wiki or answer.

| Task family | Example observable outcome |
|---|---|
| Cross-session synthesis | A release plan obeys dependencies and ownership recorded in different sessions |
| Correction and currentness | Deployment configuration uses the newly approved retention and capacity values |
| Preferences | An artifact follows explicitly stated format/language preferences without another reminder |
| Authority and uncertainty | An unapproved recommendation stays unapproved; an unknown price stays unknown |
| Long noisy histories | The agent locates a relevant decision without receiving the whole corpus |
| Negative controls | Unrelated memory does not change an answer; unsupported requests lead to abstention or clarification |

Include actual deliverables: generate a config file validated against required values, update a Go fixture with deterministic acceptance tests, and produce a launch checklist with scored decision constraints. Evaluate substantive task completion, not only fact questions. Use safe fixture tools with identical outputs across conditions.

A budgeted pilot could use four histories × eight tasks × three conditions × two repeats = 192 foreground trials, plus compilation. This is a proposed experiment, not performed work or a guarantee of statistical power. Use a small pilot to estimate variance/cost, then choose held-out sample size. Freeze prompts and scorer before the held-out run; use entirely new histories/personas, not merely new questions against a development history. Never tune on held-out results. Repeat on a second harness before generalizing beyond the first tested integration.

Counterbalance condition order, use fresh agent contexts and isolated caches, record provider/model settings, and define warm versus cold runs. Freeze the task's memory revision. Evaluate online freshness separately using controlled event timing. Keep failures/timeouts in the denominator; distinguish infrastructure errors from incorrect answers. Human or optional model judges should be blind to the condition; publish their rubric and disagreement, separate from executable checks.

## The result table worth publishing

| Metric | Definition | Delta to report |
|---|---|---|
| Task success | Deliverables satisfying all required checks / attempted tasks | Percentage-point change |
| Currentness | Correct use of latest applicable decisions / scored decisions | Percentage-point change |
| Unsupported-claim rate | Unsupported factual claims / assessed factual claims | Reduction, with claim counts |
| Evidence recall | Gold supporting events actually read / required gold events | Percentage-point change; N/A for a baseline without evidence access |
| Citation support | Cited claims supported by their cited evidence / assessed cited claims | Rate with coverage; abstention is not perfect support |
| Clarification burden | Necessary follow-up requests for already-recorded information | Difference per task; preserve appropriate uncertainty questions |
| Foreground latency | Task start to validated final deliverable | Median and p95; paired difference |
| Freshness lag | Durable acceptance to usable published correction | Median/p95, separately from foreground latency |
| End-to-end cost | Agent + retrieval + amortized compilation/indexing + retries | Cost per successful task |
| Memory growth | New/changed pages and bytes per ingested session | Supporting operational metric, not quality |

Report sample counts and paired uncertainty intervals, grouping resampling by independent user history because questions from one history are correlated. Show negative regressions and results by task family. Do not label a small single-run difference significant. For baseline tasks without historical access, explain that correctness measures the benefit of persistence; their unavailable citation metric is N/A, not zero.

For Q downstream tasks over a frozen history:

`total cost = ingestion/indexing/compilation cost + sum(external-agent and service retrieval costs)`

`cost per successful task = total cost / number of successful tasks`

Count each provider request once—service counters and nested tool counters may overlap. Record retries, model ID, input/output/cache token categories when available, rate/date, and whether usage is estimated. Report the initial compilation expense separately and its amortization at several Q values. Only quote a break-even point when measured foreground savings are positive. Keep provider spend distinct from hosting/storage cost.

## Public demonstration package

Present one before/after task side by side: identical fresh-agent request, baseline artifact, Trace2Mem artifact, acceptance-check results, and the exact memory sources responsible for the difference. Show a later correction changing the result, plus an unapproved proposal that the agent correctly refuses to call a decision. Include the trace, revision diff, model/configuration, reproduction command, and a failure example.

Lead with measured task success/currentness and total cost per successful task once results exist. Until then, say: “Trace2Mem compiled a synthetic multi-session project history and enabled a fresh Kimi session to retrieve corrected requirements through cited memory. Broader agent-quality and cost gains remain unproven.” This describes today's evidence without borrowing Brain's published gains or implying support for every harness.

Implementation entry points: the [external-agent benchmark runner](../evaluation/README.md) and repeated `/api/evaluate` pairs described in [Evaluation](EVALUATION.md). The small Harbor directory-agent pilot has run; the broader independent-history and multi-harness experiments proposed above remain unperformed.
