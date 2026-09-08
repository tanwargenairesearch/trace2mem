# Contributing

Trace2Mem is a self-hosted agent memory service inspired by Brain. Keep the domain independent of transports and providers; preserve per-user isolation, precise citations, bounded model tools, and atomic publication.

Use the Go version pinned in `go.mod`, Docker Compose, and the pinned generation tools in CI. Run `make test`, `make test-e2e`, and `make test-fuse` for changes affecting memory or filesystem behavior. Run `make terraform-check` and `terraform -chdir=infra/gcp test` for infrastructure changes. Terraform validation does not authorize an apply.

Protocol changes belong in `proto/trace2mem/v1/trace2mem.proto`; regenerate rather than editing generated Go. Reserve removed field numbers and names. This pre-release API has breaking ownership changes, so update clients, fixtures, examples, and upgrade notes together.

Keep changes focused and test observable behavior, including relevant cancellation and isolation failures. Review production behavior, architecture alignment, and unnecessary complexity before committing. Include checks run and remaining limitations in pull requests. Contributions are licensed under Apache-2.0.

Never commit `.env`, `.local`, tokens, provider credentials, Terraform state, real trajectories, or private evaluation reports. Synthetic fixtures are welcome. Do not claim Brain's quality, latency, or cost results without a reproducible measurement of this implementation.
