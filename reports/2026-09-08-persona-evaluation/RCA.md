# What the hill-climb improved—and what it did not prove

The development change improved combined notes/wiki exact-task success from 14/32 to 21/32. On held-out users, full wiki passed 27/32 versus 26/32 for notes/sessions and 23/32 for full history. The wiki used 24.7% fewer foreground tokens than notes/sessions, but 37% more than full history. These are observed results for this configured consuming agent, not general semantic-quality or monetary-cost gains.

## Why the earlier integration failed

The Harbor pilot often answered without invoking memory tools. In the new optional development run, 14/32 memory-condition trials made no successful file read. Some simply returned the literal field types from the requested JSON shape. Other trials found the correct facts but prefixed their JSON with prose, causing parsing to fail.

The controlled candidate requires an explicit action, a successful memory read in file-memory conditions, and complete reads of cited original evidence. It submits the answer through the same structured action tool. Trials without a successful read fell to 2/32 in development. Tool use and finalization changed together, so this experiment does not identify their separate contributions. The stricter protocol also introduced repair loops and substantially higher development token consumption.

A separate Dream preparation failure put a prose explanation in a source-ID array. The verifier schema now enumerates available evidence IDs and directs explanations to `reason`. Nadia's retry passed the unchanged publication checks. All five compilation attempts—including that failure—are retained.

## Held-out failure audit

| Condition | Attempted | Passed | Execution failures | Exact-value mismatches after execution |
|---|---:|---:|---:|---:|
| Full history | 32 | 23 | 6 | 3 |
| Notes + sessions | 32 | 26 | 5 | 1 |
| Full wiki | 32 | 27 | 4 | 1 |

The wiki's one-task advantage over notes/sessions corresponds to one fewer execution failure. It is not evidence that a missing fact was recovered only by the wiki. The four-task advantage over full history comprises two fewer execution failures and two fewer exact-value mismatches.

Manual inspection of all five non-execution failures found punctuation or additional factual detail rather than an obviously different underlying fact:

| Trial | Expected field | Returned field |
|---|---|---|
| Owen date change, wiki, repeat 1 | Reason clause without final punctuation | Same clause with a final period |
| Owen date change, notes, repeat 2 | Reason clause without final punctuation | Same clause with a final period |
| Owen briefing, history, repeat 1 | `Reed Rescue`; `HTML` | `Reed Rescue in Galway`; `HTML, large text, Irish` |
| Leena briefing, history, repeat 2 | `Birch Museum` | `Birch Museum in Turku` |
| Owen briefing, history, repeat 2 | `Reed Rescue`; `HTML` | `Reed Rescue (Galway)`; `HTML, large text, Irish` |

The frozen scores remain unchanged. This is a post-run diagnostic, not a replacement semantic score or validation of every claim/citation. An answer can pass all expected fields while adding an unsupported extra claim; gold-citation agreement is reported separately. Conversely, a prose-prefixed answer may contain correct facts but fail parsing.

`provider_failure` is a coarse generation-error category, not proof of a remote service outage. Its underlying transport/provider/protocol cause is not retained in these reports. Step and conservative token-budget failures remain failures, including when useful evidence had already been read.

## Why this does not settle memory quality or cost

All 18 source events fit comfortably in the full-history baseline. Wiki navigation adds model round trips, and the controlled policy requires original-source reads even when published pages already cite them. This tests verified-source answering, not the fastest policy that trusts published pages.

Only two held-out personas were used, with task templates shared with development. There were no external-tool execution tasks, second harness, independent semantic judge, or incremental publication/freshness measurement. Background compilation and estimated embedding usage are additional to foreground tokens; export preparation is outside foreground latency. No price schedule was supplied.

The concrete result is improved consuming-agent reliability after a bounded development change, followed by a modest positive held-out wiki comparison. Broad memory-quality gains, statistical generalization and monetary savings remain unproven.

## Next experiments, not completed work

1. Reduce repair loops with schema-level read-batch limits and a typed final-answer contract. Preserve failure categories without logging secrets. Develop on development data; use a new held-out set for selecting further changes.
2. Pre-register semantic-equivalence checks that tolerate harmless punctuation and separate entity fields from display text. Retain raw exact checks and citation-support assessments alongside them.
3. Vary history length and distractor volume, compare against both full history and a strong source-retrieval baseline, and keep task-specific gold evidence fixed before inference.
4. Test actual deliverables, a second harness, incremental corrections, and compilation-plus-retrieval cost across repeated conversations. Do not select the most favorable task or omit failed attempts.

The [development decision](../../evaluation/personas/frozen/DEVELOPMENT_DECISION.md) was committed as `9bf4e94` before held-out inference. Histories were authored in `10fa17b`, the evidence/protocol repair in `5060927`, the evaluation driver and consuming binary source in `5f09938`, and the pre-inference suite freeze in `11d20d1`. The [report](README.md) links complete shared traces, generated memory, compilation diagnostics and offline verification instructions.
