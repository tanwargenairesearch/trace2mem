resource "google_project_service" "vertex" {
  project            = var.project_id
  service            = "aiplatform.googleapis.com"
  disable_on_destroy = false
}
resource "google_project_iam_member" "vertex" {
  for_each = toset(["api", "worker"])
  project  = var.project_id
  role     = "roles/aiplatform.user"
  member   = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_iam_workload_identity_pool" "github" {
  count                     = var.github_repository != "" ? 1 : 0
  workload_identity_pool_id = "${var.name}-github"
  depends_on                = [google_project_service.api]
}
resource "google_iam_workload_identity_pool_provider" "github" {
  count                              = var.github_repository != "" ? 1 : 0
  workload_identity_pool_id          = google_iam_workload_identity_pool.github[0].workload_identity_pool_id
  workload_identity_pool_provider_id = "github"
  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
  }
  attribute_condition = "assertion.repository == '${var.github_repository}' && assertion.ref == 'refs/heads/main'"
  oidc { issuer_uri = "https://token.actions.githubusercontent.com" }
}
resource "google_service_account_iam_member" "github" {
  count              = var.github_repository != "" ? 1 : 0
  service_account_id = google_service_account.app["deploy"].name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github[0].name}/attribute.repository/${var.github_repository}"
}
resource "google_project_iam_member" "deploy_run" {
  project = var.project_id
  role    = "roles/run.developer"
  member  = "serviceAccount:${google_service_account.app["deploy"].email}"
}
resource "google_service_account_iam_member" "deploy_as" {
  for_each           = toset(["api", "worker", "migration"])
  service_account_id = google_service_account.app[each.key].name
  role               = "roles/iam.serviceAccountUser"
  member             = "serviceAccount:${google_service_account.app["deploy"].email}"
}
resource "google_artifact_registry_repository_iam_member" "deploy_push" {
  repository = google_artifact_registry_repository.images.name
  location   = var.region
  role       = "roles/artifactregistry.writer"
  member     = "serviceAccount:${google_service_account.app["deploy"].email}"
}
resource "google_logging_metric" "compilation_failures" {
  name   = "${var.name}/compilation_failures"
  filter = "resource.type=\"cloud_run_worker_pool\" AND resource.labels.worker_pool_name=\"${var.name}-worker\" AND jsonPayload.msg=\"compilation failed\""
  metric_descriptor {
    metric_kind = "DELTA"
    value_type  = "INT64"
  }
}
resource "google_monitoring_alert_policy" "compilation_failures" {
  display_name = "${var.name}: repeated compilation failures"
  combiner     = "OR"
  conditions {
    display_name = "More than 3 failures in five minutes"
    condition_threshold {
      filter          = "metric.type=\"logging.googleapis.com/user/${google_logging_metric.compilation_failures.name}\" AND resource.type=\"cloud_run_worker_pool\""
      comparison      = "COMPARISON_GT"
      threshold_value = 3
      duration        = "0s"
      aggregations {
        alignment_period   = "300s"
        per_series_aligner = "ALIGN_SUM"
      }
    }
  }
  notification_channels = var.notification_channels
}
output "database_bootstrap_sql" {
  value = <<-SQL
    CREATE EXTENSION IF NOT EXISTS vector;
    GRANT CREATE ON DATABASE trace2mem TO "${google_sql_user.iam["migration"].name}";
    GRANT USAGE, CREATE ON SCHEMA public TO "${google_sql_user.iam["migration"].name}";
    GRANT USAGE ON SCHEMA public TO "${google_sql_user.iam["api"].name}", "${google_sql_user.iam["worker"].name}";
    ALTER DEFAULT PRIVILEGES FOR ROLE "${google_sql_user.iam["migration"].name}" IN SCHEMA public GRANT SELECT,INSERT,UPDATE,DELETE ON TABLES TO "${google_sql_user.iam["api"].name}", "${google_sql_user.iam["worker"].name}";
    ALTER DEFAULT PRIVILEGES FOR ROLE "${google_sql_user.iam["migration"].name}" IN SCHEMA public GRANT USAGE,SELECT ON SEQUENCES TO "${google_sql_user.iam["api"].name}", "${google_sql_user.iam["worker"].name}";
  SQL
}
output "workload_identity_provider" { value = try(google_iam_workload_identity_pool_provider.github[0].name, null) }
output "deployment_service_account" { value = google_service_account.app["deploy"].email }
resource "google_secret_manager_secret_iam_member" "oidc" {
  count     = var.oidc_client_secret_id != "" ? 1 : 0
  secret_id = var.oidc_client_secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app["api"].email}"
}
