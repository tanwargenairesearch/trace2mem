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
