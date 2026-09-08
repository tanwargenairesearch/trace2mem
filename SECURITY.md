# Security reporting

Do not use public issues for credentials, personal trajectories, or exploitable vulnerabilities. Report vulnerabilities through [GitHub private vulnerability reporting](https://github.com/tanwargenairesearch/trace2mem/security/advisories/new). This channel is enabled for the public repository.

Include the affected commit, deployment mode, impact, and a minimal synthetic reproduction. Never include a live bearer token or provider key. There is no supported stable release or guaranteed response SLA yet.

Each verified user owns one memory. Agent tokens inherit that identity and are scoped to read, ingest, or manage. Operator access is not a universal memory reader. OIDC identities use issuer and subject; email is not an ownership key. Shared deployments require TLS, an approved identity provider, and an appropriately configured MCP OAuth authorization server.

Model endpoints are operator-approved. Imported text is untrusted evidence; model judgment remains fallible. Back up the credential-encryption key separately and restrict access to database, blob, spool, evaluation-report and cache files. Forgetting cannot recall exports or previous backups.
