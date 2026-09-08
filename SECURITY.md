# Security reporting

Trace2Mem is experimental. Do not use public issues for credentials, personal trajectories, or exploitable vulnerabilities. Use GitHub's private vulnerability reporting when enabled on the published repository; otherwise contact its maintainer privately before sending sensitive details. A public security contact must be configured before the first release.

Include the affected commit, deployment mode, impact, and a minimal synthetic reproduction. Never include a live bearer token or provider key. There is no supported stable release or guaranteed response SLA yet.

Each verified user owns one memory. Agent tokens inherit that identity and are scoped to read, ingest, or manage. Operator access is not a universal memory reader. OIDC identities use issuer and subject; email is not an ownership key. Shared deployments require TLS, an approved identity provider, and an appropriately configured MCP OAuth authorization server.

Model endpoints are operator-approved. Imported text is untrusted evidence; model judgment remains fallible. Back up the credential-encryption key separately and restrict access to database, blob, spool, evaluation-report and cache files. Forgetting cannot recall exports or previous backups.
