# Pi and Hermes adapter smoke evaluation — 2026-09-09

The real Pi CLI and real Hermes plugin dispatcher passed local deterministic transport checks. These checks do **not** measure live-model recall or memory-quality improvement. This report records the earlier local-only checks. The subsequent user-authorized [OpenRouter-only live evaluation](../2026-09-09-adapter-live/README.md) is recorded separately.

## Runtime checks

| Harness | Runtime | Result | Evidence |
| --- | --- | --- | --- |
| Pi | `@earendil-works/pi-coding-agent` 0.85.1, actual CLI | Pass | Local scripted OpenAI-compatible server emitted a file-read call and final response; six captured envelopes, matching call/result IDs, closed lifecycle, exactly one memory index in each of two provider requests |
| Hermes | 0.21.1, upstream commit `bf53ff00a7360826ec2c9e2949533160068a8fc8`, actual plugin dispatcher and tool observer | Pass after fix | Plugin discovery enabled Trace2Mem; initial context returned; six captured envelopes, successful tool result, closed lifecycle |

Hermes initially mapped success incorrectly: its actual tool observer emits `status="ok"`, whereas the adapter expected `"success"`. Corrected the mapping and added a regression using `ok`. The runtime smoke invokes Hermes's own tool-observer function rather than manufacturing its status field.

Ten core tests pass, covering normalized text/tool/lifecycle capture, initial context failure, revision forwarding, credential isolation, exact-envelope replay after a lost acknowledgment, capacity/conflict transaction rollback, oversized results, and Hermes success status. The preceding implementation validation also passed five LangChain tests and the Node bridge regression. The one-line status fix was reviewed for production, design and simplicity concerns; no remaining findings.

## Scope and limitations

- Pi used a deterministic localhost model fixture; Hermes used its real dispatcher with synthetic hook inputs. Neither establishes model task success.
- No live generation or embedding requests were executed. No live compilation, citation-supported recall, or service-level cross-user isolation result is claimed.
- Retry and credential isolation are covered by client tests, not a new full service outage or multi-user harness trial.
- The eval has a separate Docker Compose project, volumes, harness homes and spool files. The service was stopped after local checks; the volumes remain available for the authorized live follow-up.
- Private scripts and raw traces are under `.local/adapter-eval-20260909/`, excluded from Git. They use synthetic project data. The Pi and Hermes local runtime transcripts are `pi-runtime-smoke.json` and `hermes-runtime-smoke.json`.

This is a development smoke test, not a benchmark or production-readiness claim.
