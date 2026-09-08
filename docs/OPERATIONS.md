# Operations

## Local storage and identity

Compose owns `trace2mem_postgres`, `trace2mem_blobs`, and `trace2mem_secrets`. `make bootstrap-token` prints the authoritative token. The encryption key is in the secrets volume; losing it makes stored provider credentials unrecoverable. Keep a restricted backup alongside a PostgreSQL dump and blob backup. Never commit keys, access tokens, provider secrets, or Terraform state.

Only localhost receives published application traffic. Shared installations must configure an external OIDC issuer/client and OAuth service audience. Browser login uses authorization code + PKCE and nonce. MCP OAuth needs an authorization server with discovery, scopes, audience, and PKCE; an OIDC login alone does not provide MCP authorization. Per-user ownership and token scopes are enforced by the same service logic across transports.

Forgetting immediately suppresses the user’s published memory, removes affected evidence and revisions, then queues rebuilding from remaining events. Tombstones reject replay of deleted IDs. Artifact deletion is queued when no live reference remains. Database backups, blob-store retained versions, and client exports have separate retention and cannot be instantly recalled. Configure retention to match your policy and record purge completion.

## Google Cloud

Terraform is under `infra/bootstrap` and `infra/gcp`. No apply or cloud deployment is part of local tests.

1. Select a project and region (default `europe-west2`). Review the always-on baseline: zonal Cloud SQL, provisioned worker instance, network/storage/logging; HA adds regional database availability and recovery cost. Use current Google pricing for your selected sizes before applying.
2. Initialize/apply `infra/bootstrap` for a versioned state bucket. Configure a GCS backend in a local backend file; don't store state in Git.
3. Initialize `infra/gcp` with `deploy_app=false`, project, and identity settings. Run `terraform plan`, review IAM/resources, and apply deliberately.
4. Build/push the image to the created Artifact Registry repository and select its immutable digest. Provision bootstrap-token and optional OIDC-client-secret versions directly in Secret Manager, outside Terraform. Terraform references secret IDs only.
5. Execute the `database_bootstrap_sql` Terraform output as a Cloud SQL administrator from a private-network client. It installs vector support and grants the migration identity schema ownership privileges and API/worker data privileges.
6. Set the image digest with `deploy_app=false` to provision the migration Cloud Run job. Execute that job and inspect successful completion before enabling API/worker with `deploy_app=true`. Set the final public URL and OIDC redirect URI.
7. Run HTTP/gRPC/MCP ingestion, compilation, evidence, GCS read/write/delete, worker restart, and private SQL IAM checks in a disposable project. Record the exact image, Terraform lock, region, output, and cost. This cloud smoke run is separately invoked and has not been performed here.

GitHub federation can be enabled with `github_repository`; trust is restricted to that repository's main ref. No downloaded service-account keys are needed. Migration execution is explicit. Schema migrations are additive; roll back only to an image compatible with every applied migration. These initial migrations have no tested cross-release rollback window yet.

## Recovery

Enable Cloud SQL backups/PITR; the HA input selects regional availability. Practice restoration into a new instance: stop writes, restore database, restore matching blobs, retain KMS key versions, point a migration-compatible application at the restored instance, verify citations and event watermarks, then reopen traffic. Never destroy the KMS key while encrypted credentials remain in backups. Local recovery likewise needs the original master key. Take fresh snapshots before schema upgrades and test restoration on a disposable copy.

## Observability and evaluation

Health: `/healthz`; readiness: `/readyz`. Jobs expose status, last error, and freshness through ingestion status/MCP. Structured worker logs contain IDs and errors rather than trajectory content. Terraform includes a compilation-failure alert; broader latency, queue, cache, and budget dashboards remain release work.

Evaluation runs matched wiki/without-wiki cases, records revision/configuration/usage/latency, and scores explicit fact patterns, attached supporting citations and source evidence. It is a reproducible baseline, not an independent correctness judge. Prompt candidates use development results and are evaluated on heldout cases; promotion requires an explicit owner action. Real providers require configured models, credentials, and budgets. Do not infer the blog's claimed improvements from scripted fixtures.

## Rename from Brain to Trace2Mem

This pre-release rename changes the Protobuf namespace to `trace2mem.v1`, Go imports to `github.com/trace2mem/trace2mem`, CLI to `trace2mem`, configuration prefix to `TRACE2MEM_`, and browser cookie name. Regenerate clients and update environment variables; sign in again with the existing token.

The per-user redesign preserves the old `brain_*` volumes as rollback data and uses fresh `trace2mem_user_*` development volumes. The previous private environment is backed up in ignored `.local/before-per-user.env`. Fresh installs use default Trace2Mem volume names. Do not delete old volumes or merge test histories automatically.

The local checkout directory remains `brain-implementation` so existing workspace references continue to work. Published repository and module naming use Trace2Mem; a repository owner still needs to be selected before publication.
