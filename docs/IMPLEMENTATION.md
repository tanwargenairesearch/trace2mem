# Implementation tracking

Accepted design: Go service, Protobuf/Connect, PostgreSQL revisions and jobs, three memory layers, bounded Dream and retrieval agents, MCP, read-only cached FUSE, local Docker, GCP Terraform, BYOK/Ollama, scoped tokens/OIDC, Apache-2.0.

## Milestones
- [ ] Durable contracts, ingestion, adapters, local setup
- [ ] Dream, evidence, revisions, providers
- [ ] Retrieval, MCP, context, cache and FUSE
- [ ] Console, identity, credentials, forgetting
- [ ] Evaluation and candidate promotion
- [ ] Terraform, release, documentation
- [ ] Deterministic Docker acceptance tests
- [ ] Three-lens code review and findings resolved

## External validation
Real provider tests require explicitly configured models/credentials. GCP smoke deployment is separate from local validation and requires a project and a reviewed cost baseline. No deployment is implied by Terraform validation.

## Review ledger
Pending implementation and tests.

### Review triage (active)
Must-fix:
- [in progress] Fence proposal/evaluation persistence against forgetting; purge referenced artifacts.
- [pending] Incremental bounded Dream inputs, preserve unchanged knowledge, verify empty proposals.
- [pending] Native tool call/result pairing in both providers with protocol tests.
- [pending] Concurrent provider budget reservation and embedding accounting.
- [pending] Validate retrieval citations against inspected sources.
- [pending] Bound cache across revisions and protect immutable manifest identity.
- [pending] Cancel requests and close connections during shutdown.
- [pending] Add valid-token membership/isolation tests.
- [pending] Remove dead fields/helpers and unused-import placeholders.
Nice-to-have: broaden semantic benchmark corpus after initial fixtures.
