# Per-user Brain alignment delivery

This tracks the accepted redesign. It is not a completion claim.

- Ownership/API: implemented; Go tests, race checks, Protobuf lint, Docker ownership/lifecycle checks, and Chrome model-readiness/guide checks passed. Linux Docker FUSE reads, recursive search, and write rejection passed. All ownership review findings resolved.
- Dream: implemented model-authored summaries and subject syntheses, readable Markdown notes with stable provenance metadata, explicit/preserved relationships, bounded orientation, semantic verification, and fenced no-op. Deterministic Docker scenarios passed; real-model quality remains unmeasured.
- Scheduling: durable automatic/daily/manual modes, missing-model blocking, credential recovery, and pinned approved ranges implemented. Automatic mode uses 60 seconds quiet and a five-minute cap; daily scheduling coalesces catch-up work and handles DST.
- LangChain: capture hooks, bounded durable spool, context helper, JSONL import, and a two-conversation example implemented. Five callback/durability tests passed. Live and saved-JSONL flows passed against Docker; a second LangChain conversation receives initial context and invokes retrieval.
- Console: per-user navigation, model readiness, guide, Connections tokens and Settings scheduling implemented; desktop/mobile Chrome controls verified.
- Evaluation: pinned paired runs, explicit fact/evidence rubrics, failed-answer records, request usage, optimizer accounting and fenced promotion implemented. Real-model learn/correct/retrieve/forget fixture passed; eight lexical rubric answers passed in both ablation conditions. No advantage established.
- Operations: Terraform validation/mock test passed; public guides and clean-checkout acceptance script added. Fresh-checkout Docker/FUSE validation of `00a945c` passed with private configuration excluded; cloud/release publication remains separate.

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

## Scheduling and console review tracking

Production: credential replacement previously left failed jobs stranded, and CloseSession reported scheduling even in manual mode. Both fixed with Docker regressions; resumed jobs retain their approved watermark. Principles: queued request watermark now determines the promoted target, and schedule normalization is explicit. Design: no findings. All three re-review lenses report zero findings. Go race tests, Protobuf lint, Docker timing/catch-up/manual-range tests, and Chrome navigation/schedule controls passed on 2026-09-08.

## Evaluation review tracking

All review commitments resolved: unresolved embedding reservations included; optimizer invocation/usage persisted separately; pending reindex and baseline generation changes block promotion. Citation recall was accurately renamed, the corrected-deadline rubric strengthened, and nested counters documented. Docker regression tests passed; all three final review lenses report zero findings. Live testing additionally exposed reasoning-item text leaking into answers; the Responses parser now returns only message/output_text, covered by a protocol regression. Failed answers remain visible in evaluation reports.

## Final local validation — 2026-09-08

Clean Git archive `00a945c` passed fresh-volume Docker ingestion/ownership/scheduling/evaluation tests and Linux FUSE, then removed only its uniquely named test resources. OpenRouter Kimi K3 + Vertex Gemini embeddings completed the real synthetic scenario in 169.978 seconds; reports are linked in EVALUATION.md. The fixture learned, corrected, answered and immediately suppressed forgotten evidence. The disposable credential container and database were removed. All final review lenses were clear before the milestone commit. Public cloud smoke deployment, external identity interoperability and publishing remain unperformed.

Final evidence review: production, design and principles reviewers all reported zero findings on the synthetic reports, measured claims, redaction and Protobuf-JSONL history export. No provider credentials or real user trajectories are included in the public artifacts.

## Kimi project-agent review tracking

Review in progress for the opt-in project-agent exercise and disposable Docker runner. Production, design, and principles lenses are checking capture fidelity, fresh-session retrieval, bounded execution, and credential cleanup. Findings and actual execution results will be recorded before completion.

Review findings resolved: compilation drains the accepted watermark; recall requires successful index/search/read/evidence calls and resolved cited IDs; search excerpts are bounded by Unicode characters; unreachable branch checks removed; fact checks described as term-presence smoke checks. The richer live history exposed mixed-role citations, so Dream now explicitly asks for one actor role per observation; validation remains strict and a regression covers mixed citations. Production and principles re-review report zero findings; design cleared the runner's ownership/retrieval flow. Go race tests passed. Live execution results remain pending.

Model request-limit review cleared after fixes: provider-specific omitted defaults stay in the adapter; incomplete responses cannot dispatch partial calls; nonzero usage on failure reconciles the reservation. YAML allows bounded explicit output/timeout settings. Docker tests cover reported failure accounting and unchanged Ollama defaults. Production, design and principles final reviews report zero findings. This validates the mechanics, not the pending live memory result.

Further live-driven controls: optional reasoning effort leaves omitted defaults unchanged. Verification/composition now request a required named tool; Responses forces tool_choice and both HTTP adapters reject nonconforming replies. The Ollama enforcement difference is documented. Race/protocol tests pass and production/principles review is clear; the design finding was documentation-only and has been resolved. The non-thinking Kimi replay has published its initial design revision and is processing the correction conversation.

Citation-repair gate: Docker regression verifies one repair maximum, preservation of rejected diagnostics and usage, and mandatory semantic verification after repair. Production, design and principles reviews report zero findings. The latest live run saved initial revision 139bba4cc575d4e69b59590d539c864c at watermark 14 before processing corrections.
