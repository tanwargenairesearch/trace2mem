# Pi and Hermes live adapter evaluation — 2026-09-09

**Pi passed the small end-to-end scenario. Hermes recalled the correct facts with supporting search citations, but failed explicit evidence retrieval.** This is one development scenario per harness, not a comparative memory benchmark or a production-readiness claim.

Generation used `moonshotai/kimi-k3`; embeddings used `openai/text-embedding-3-small`, both through OpenRouter. No Gemini credential or endpoint was used. Models were probed before the trials. Pi was `@earendil-works/pi-coding-agent` 0.85.1; Hermes was 0.21.1 at `bf53ff00a7360826ec2c9e2949533160068a8fc8`.

## Scenario and outcomes

Each agent read the same synthetic fixture: project Cedar Lantern has an approved release date of **2026-11-19** and raw log retention of **13 days**. Capture was delivered to separate authenticated memories and compiled with live Dream. A fresh process in another working directory was asked to retrieve those two values and supporting event IDs using search and evidence tools. Pi had only memory tools enabled during recall. Hermes used its tool-discovery bridge; its recorded recall trace contains no local file read. Neither invocation resumed the capture conversation.

| Check | Pi | Hermes |
| --- | --- | --- |
| Accepted capture envelopes | 7 | 8 |
| Captured tool pairs | 1 | 2 |
| Capture spool drained | Pass | Pass |
| Live Dream publication | Pass | Pass after one failed attempt |
| Correct release date | Pass | Pass |
| Correct retention period | Pass | Pass |
| Answer cites supporting captured source | Pass | Pass |
| Successful explicit `memory_evidence` calls | 2 | 0 |
| Recall tool failures | 0 | 2 missing-argument calls |
| Corrected capture duration | 7.38 seconds | 14.19 seconds |
| Recall duration | 16.29 seconds | 26.26 seconds |
| Overall | Pass | Partial |

Pi searched at revision `97d13fc86b6d6fc58659b8c25f685c03`, then retrieved the original file-read result and assistant acknowledgment through `memory_evidence`. Its final answer called those two records independent confirmations; the acknowledgment is derivative, so that wording overstates corroboration.

Hermes searched at revision `ddde2cf54ef1961ca7e1ee2900f88e50`. It then twice requested `memory_evidence` through `tool_call` with empty arguments. Hermes rejected both before invoking the adapter because `eventId` was missing. It answered from search results, with the correct values and an original source citation, but also cited a user request and tool call that do not themselves contain the values. This is a model/tool-dispatch reliability failure; it is not an adapter transport failure. The CLI also warned `Unknown toolsets: trace2mem`; plugin tools remained discoverable through its bridge. No selective rerun was used to hide these recall failures.

## Failures found and fixes

The initial live capture trial failed for **both** adapters: colon-prefixed session IDs violated the service's `[A-Za-z0-9_-]`, 128-character ID contract. All seven Pi and eight Hermes envelopes remained in their private spools. The adapters now use a framework prefix plus SHA-256 of the native session ID and preserve the original in metadata. A regression covers long, Unicode and punctuation-bearing native IDs. Failed spools and outputs were archived unchanged; corrected capture was rerun in fresh sessions/spools. These initial failures remain in [results.json](results.json).

The earlier real-runtime smoke test also caught Hermes's actual successful-tool status, `ok`, being misclassified as failure. That mapping and its regression were fixed before these live trials.

Hermes's first Dream attempt failed with `invalid subject path`. The service rejected the model's invalid read request and retried; the second attempt published. Pi's additional recovery event produced a no-op compilation. The worker began recompiling Pi's recall after it closed; it was stopped after both capture revisions were published to avoid unnecessary subsequent compilation. Accounting includes that interrupted work.

## Recovery and isolation

For each user, a separate synthetic event was accepted by the real service while the client simulated a lost acknowledgment. The failed client retained the event, closed, and reopened its SQLite spool. Replay returned **one duplicate**, preserving the original envelope. Fetching that event with the other user's credential returned **404**. Both checks passed; details are in [results.json](results.json). This tests acknowledgment loss/restart and cross-user evidence access, not every network outage or authorization path.

## Evidence, accounting and limits

- [results.json](results.json) records final answers, source event IDs, tool failures, revisions, timings and recovery results.
- [usage.json](usage.json) retains available service and harness accounting, including initial failed captures. Service accounting totals 141,886 tokens, with estimated embedding counts and estimated/interrupted generation accounting. This is not an exact billed-token or dollar total. Pi costs and Hermes costs are harness estimates; Hermes reported no actual billed cost.
- Eleven core regressions, Pi type-checking, the real Node/Python bridge test, and whitespace checks passed after the session-ID fix. Production/design/simplicity review of the two fixes found no remaining code issues.
- No no-memory baseline, correction scenario, repetitions, or held-out dataset was run. Correct answers here establish a working path, not a measured improvement over native memory.
- Raw scripts, transcripts, retained failed spools and isolated harness state are under `.local/adapter-eval-20260909/` and excluded from Git. Only synthetic evidence is used. Eval services were stopped afterward; isolated volumes remain for investigation.
