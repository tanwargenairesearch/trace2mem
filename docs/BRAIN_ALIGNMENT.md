# Brain alignment

Trace2Mem is an experimental agent memory service inspired by Brain. The comparison below concerns principles, not equivalent implementation or measured results.

| Brain principle | Trace2Mem behavior | Status and adaptation |
|---|---|---|
| Accumulated user history | Verified identity automatically resolves one memory across agents and sessions | Implemented; local tokens and issuer/subject OIDC mapping are our adaptations |
| Three complementary layers | Original event envelopes and session summaries; readable notes; synthesized Markdown subjects | Implemented; bounded composition can reject oversized workloads |
| Context and evidence links | Explicit subject relationships and stable event citations; corrections retain temporal status | Implemented; model semantic verification is recorded judgment, not proof |
| Initial orientation | Compact index through MCP, API and the LangChain context helper | Implemented; connector attachment alone does not inject it |
| Background investigation | Orient, summarize, inspect memory/artifacts, compose, verify, publish/no-op | Implemented; external connector investigation and subagent delegation deferred |
| Coherent staged updates | Proposal validation followed by atomic revision publication | Implemented using PostgreSQL parent checks, renewable leases and fencing |
| Foreground retrieval | Search, bounded cited synthesis, pinned directories and read-only FUSE | Implemented; Linux FUSE tested, native macOS experimental |
| Measure the wiki's contribution | Paired runs with/without knowledge pages, retaining notes and sessions | Evaluation harness implemented; see the evaluation report for actual measurements |

PostgreSQL, blob storage, authenticated APIs, durable schedules, token scopes, provider adapters, Terraform and the web console are Trace2Mem engineering choices. Git export provides portability; Git does not coordinate publication. Internal database columns named `space` identify owned memories and are not user-selectable containers.

The service investigates only evidence made available through ingestion and referenced artifacts. It cannot inspect arbitrary applications, run a shell, or discover external context by itself. Dream's bounded input limits and semantic model failures are explicit errors. There are no claims of reproducing Brain's reported performance gains.
