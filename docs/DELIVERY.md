# Per-user Brain alignment delivery

This tracks the accepted redesign. It is not a completion claim.

- Ownership/API: implemented; Go tests, race checks, Protobuf lint, Docker ownership/lifecycle checks, and Chrome model-readiness/guide checks passed. Linux Docker FUSE reads, recursive search, and write rejection passed. All ownership review findings resolved.
- Dream: pending actual summaries, Markdown notes, synthesized pages, explicit relationships, orientation, and fenced no-op.
- Scheduling: pending automatic/daily/manual modes and missing-model blocking/resumption.
- LangChain: pending capture hooks, bounded durable spool, context bootstrap, and end-to-end example.
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
