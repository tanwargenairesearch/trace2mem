# Per-user Brain alignment delivery

This tracks the accepted redesign. It is not a completion claim.

- Ownership/API: implemented; Go tests, race checks, Protobuf lint, Docker ownership/lifecycle checks, and Chrome model-readiness/guide checks passed. Linux Docker FUSE reads, recursive search, and write rejection passed. All ownership review findings resolved.
- Dream: implemented model-authored summaries and subject syntheses, readable Markdown notes with stable provenance metadata, explicit/preserved relationships, bounded orientation, semantic verification, and fenced no-op. Deterministic Docker scenarios passed; real-model quality remains unmeasured.
- Scheduling: missing-model blocking/resumption implemented and tested. Automatic/daily/manual timing modes remain pending.
- LangChain: capture hooks, bounded durable spool, context helper, JSONL import, and a two-conversation example implemented. Five callback/durability tests passed. Live and saved-JSONL flows passed against Docker; a second LangChain conversation receives initial context and invokes retrieval.
- Console: per-user navigation/model readiness/guide implemented; final navigation and connections/settings remain.
- Evaluation: pending fact-based metrics, pinned ablations and opt-in real-model report.
- Operations: pending final Terraform, fresh-checkout and release checks.

## Ownership review tracking

Three-lens review ran against the ownership changes. Must-fix findings and resolution:

- Legacy `spaceId` management query silently accepted: fixed; all aliases rejected and integration evidence verified unchanged.
- Lifecycle tests reused user data: fixed; every run creates a unique test identity and scoped token.
- Obsolete space creation/type/admin bypass scaffolding: removed; regression fixtures provision owned memories.
- Compilation scope differed between API transports: fixed; ingestion scope applies to both.
- Missing cross-interface ownership coverage: added colliding event/session IDs over HTTP/gRPC/MCP and concurrent provisioning.
- Public quickstart used removed commands: corrected and breaking upgrade documented.
- Dead model reset helper and stale space wording: removed.

Existing local volumes remain untouched; fresh `trace2mem_user_*` volumes hold this installation. Prior private configuration is backed up in ignored `.local/before-per-user.env`.

## Dream review tracking

All three review lenses cleared after fixes: no-op verification now receives inspected memory/tool results; composition includes previous subject pages and preserves omitted relationships unless explicit removal reasons pass verification; missing-model transitions lock memory before jobs to match configuration writes; source rendering no longer duplicates composed summaries/pages. Go and Docker tests include no-op watermark/fencing, configuration resumption, temporal corrections, forgetting, and large histories. Linux FUSE checks passed with the new Markdown layers.

Composition is bounded to 2 MiB input and provider request envelopes to 3 MiB; summaries/pages have a 32 KiB limit and prior summary evidence is bounded to 1,024 references. Oversized workloads fail explicitly rather than silently dropping evidence. Further memory-quality and scale evaluation is required.

## LangChain review tracking

Review findings fixed: serialize close with capture transactions; cap consecutive delivery failures; retain sanitized exception categories; demonstrate a distinct second conversation with index and retrieval tool; replace stale forthcoming documentation with locked installation/test commands. All three re-review lenses reported zero findings. Five adapter tests and Docker live/offline demonstrations passed.
