# code-review: Pi and Hermes adapters

## production [0 findings]

No remaining findings after re-review.

## design [0 findings]

No remaining findings after re-review.

## principles [0 findings]

No remaining findings after re-review.

## triage

**Must-fix (block phase/slice completion):** None remaining.

**Nice-to-have (fix opportunistically, don't block):** None remaining.

**Defer (logged but intentionally skipped):** None.

## Review tracking

Task tools were unavailable; commitments are recorded here.

- [x] Run production, design and principles review on the adapters and client extraction.
- [x] Fix partial callback capture: validate and commit Pi/Hermes envelope batches in a single SQLite transaction. Regression tests cover oversized results, queue exhaustion and conflicting IDs.
- [x] Fix repeated Pi index accumulation: supply one transient context message, leaving persisted history untouched. Exercise multiple context calls and extension reload/resume.
- [x] Replace private-database assertions with a recording client and public transport observations.
- [x] Document session namespacing and occurrence/retry identity in shared capture helpers.
- [x] Re-review all fixes: each reviewer returned zero findings.

Validation: nine core adapter/queue tests, five LangChain tests, one Node integration test exercising the real Python bridge and Pi callback registration/context/lifecycle, Pi TypeScript checking against 0.85.1, and `git diff --check`. HTTP fixture tests require localhost sockets. Core and LangChain wheels built and installed in an isolated environment. Live Pi/Hermes conversations against a running Trace2Mem service remain unverified; compatibility scope is stated in each adapter README.
