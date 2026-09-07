terraform {
  required_version = ">= 1.10, < 2.0"
  backend "gcs" {}
  required_providers {
    google = { source = "hashicorp/google", version = "~> 7.0" }
  }
}
provider "google" {
  project = var.project_id
  region  = var.region
}
resource "google_project_service" "api" {
  for_each           = toset(["run.googleapis.com", "sqladmin.googleapis.com", "compute.googleapis.com", "servicenetworking.googleapis.com", "artifactregistry.googleapis.com", "secretmanager.googleapis.com", "cloudkms.googleapis.com", "monitoring.googleapis.com", "iamcredentials.googleapis.com", "sts.googleapis.com"])
  service            = each.value
  disable_on_destroy = false
}
resource "google_compute_network" "main" {
  name                    = var.name
  auto_create_subnetworks = false
  depends_on              = [google_project_service.api]
}
resource "google_compute_subnetwork" "main" {
  name                     = var.name
  ip_cidr_range            = "10.42.0.0/24"
  network                  = google_compute_network.main.id
  region                   = var.region
  private_ip_google_access = true
}
resource "google_compute_global_address" "sql" {
  name          = "${var.name}-sql"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.main.id
}
resource "google_service_networking_connection" "sql" {
  network                 = google_compute_network.main.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.sql.name]
}
resource "google_service_account" "app" {
  for_each   = toset(["api", "worker", "migration", "deploy"])
  account_id = "${var.name}-${each.key}"
}
resource "google_sql_database_instance" "main" {
  name                = var.name
  region              = var.region
  database_version    = "POSTGRES_17"
  deletion_protection = var.deletion_protection
  settings {
    tier              = var.database_tier
    edition           = "ENTERPRISE"
    availability_type = var.ha ? "REGIONAL" : "ZONAL"
    disk_autoresize   = true
    disk_size         = 20
    database_flags {
      name  = "cloudsql.iam_authentication"
      value = "on"
    }
    ip_configuration {
      ipv4_enabled    = false
      private_network = google_compute_network.main.id
    }
    backup_configuration {
      enabled                        = true
      point_in_time_recovery_enabled = true
      transaction_log_retention_days = var.ha ? 7 : 3
    }
  }
  depends_on = [google_service_networking_connection.sql]
}
resource "google_sql_database" "brain" {
  name     = "brain"
  instance = google_sql_database_instance.main.name
}
resource "google_sql_user" "iam" {
  for_each = toset(["api", "worker", "migration"])
  name     = trimsuffix(google_service_account.app[each.key].email, ".gserviceaccount.com")
  instance = google_sql_database_instance.main.name
  type     = "CLOUD_IAM_SERVICE_ACCOUNT"
}
resource "google_project_iam_member" "sql_client" {
  for_each = toset(["api", "worker", "migration"])
  project  = var.project_id
  role     = "roles/cloudsql.client"
  member   = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_project_iam_member" "sql_login" {
  for_each = toset(["api", "worker", "migration"])
  project  = var.project_id
  role     = "roles/cloudsql.instanceUser"
  member   = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_compute_subnetwork_iam_member" "network" {
  for_each   = toset(["api", "worker", "migration"])
  subnetwork = google_compute_subnetwork.main.name
  role       = "roles/compute.networkUser"
  member     = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_storage_bucket" "blobs" {
  name                        = "${var.project_id}-${var.name}-blobs"
  location                    = var.region
  uniform_bucket_level_access = true
  public_access_prevention    = "enforced"
  force_destroy               = false
}
resource "google_storage_bucket_iam_member" "blobs" {
  for_each = toset(["api", "worker"])
  bucket   = google_storage_bucket.blobs.name
  role     = "roles/storage.objectUser"
  member   = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_artifact_registry_repository" "images" {
  repository_id = var.name
  location      = var.region
  format        = "DOCKER"
  depends_on    = [google_project_service.api]
}
resource "google_kms_key_ring" "main" {
  name       = var.name
  location   = var.region
  depends_on = [google_project_service.api]
}
resource "google_kms_crypto_key" "credentials" {
  name            = "credentials"
  key_ring        = google_kms_key_ring.main.id
  rotation_period = "7776000s"
  lifecycle { prevent_destroy = true }
}
resource "google_kms_crypto_key_iam_member" "credentials" {
  for_each      = toset(["api", "worker"])
  crypto_key_id = google_kms_crypto_key.credentials.id
  role          = "roles/cloudkms.cryptoKeyEncrypterDecrypter"
  member        = "serviceAccount:${google_service_account.app[each.key].email}"
}
resource "google_secret_manager_secret" "bootstrap" {
  secret_id = "${var.name}-bootstrap-token"
  replication {
    auto {}
  }
  depends_on = [google_project_service.api]
}
resource "google_secret_manager_secret_iam_member" "bootstrap" {
  secret_id = google_secret_manager_secret.bootstrap.id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.app["api"].email}"
}
locals {
  shared_env = {
    INSTANCE_CONNECTION_NAME = google_sql_database_instance.main.connection_name
    GCS_BUCKET               = google_storage_bucket.blobs.name
    KMS_KEY                  = google_kms_crypto_key.credentials.id
    PUBLIC_URL               = var.public_url
    OIDC_ISSUER              = var.oidc_issuer
    OIDC_CLIENT_ID           = var.oidc_client_id
    OAUTH_AUDIENCE           = var.oauth_audience
    BRAIN_MODEL_ENDPOINTS    = var.model_endpoints
  }
  db_urls = { for role in ["api", "worker", "migration"] : role => "postgres://${replace(google_sql_user.iam[role].name, "@", "%40")}@localhost/brain?sslmode=disable" }
}
resource "google_cloud_run_v2_service" "api" {
  count               = var.deploy_app ? 1 : 0
  name                = var.name
  location            = var.region
  deletion_protection = var.deletion_protection
  template {
    service_account = google_service_account.app["api"].email
    timeout         = "360s"
    scaling { max_instance_count = var.api_max_instances }
    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.main.name
        subnetwork = google_compute_subnetwork.main.name
      }
    }
    containers {
      image = var.image
      ports {
        name           = "h2c"
        container_port = 8080
      }
      resources { limits = { cpu = "1", memory = "512Mi" } }
      dynamic "env" {
        for_each = merge(local.shared_env, { DATABASE_URL = local.db_urls.api })
        content {
          name  = env.key
          value = env.value
        }
      }
      env {
        name = "BRAIN_BOOTSTRAP_TOKEN"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.bootstrap.secret_id
            version = "latest"
          }
        }
      }
      dynamic "env" {
        for_each = var.oidc_client_secret_id != "" ? [1] : []
        content {
          name = "OIDC_CLIENT_SECRET"
          value_source {
            secret_key_ref {
              secret  = var.oidc_client_secret_id
              version = var.oidc_client_secret_version
            }
          }
        }
      }
      startup_probe {
        http_get { path = "/readyz" }
      }
    }
  }
  lifecycle {
    precondition {
      condition     = var.image != "" && var.public_url != ""
      error_message = "Set an image digest and public_url before deploying the application."
    }
  }
}
resource "google_cloud_run_v2_service_iam_member" "public" {
  count    = var.deploy_app ? 1 : 0
  location = var.region
  name     = google_cloud_run_v2_service.api[0].name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
resource "google_cloud_run_v2_worker_pool" "worker" {
  count               = var.deploy_app ? 1 : 0
  name                = "${var.name}-worker"
  location            = var.region
  deletion_protection = var.deletion_protection
  scaling { manual_instance_count = var.worker_instances }
  template {
    service_account = google_service_account.app["worker"].email
    vpc_access {
      egress = "PRIVATE_RANGES_ONLY"
      network_interfaces {
        network    = google_compute_network.main.name
        subnetwork = google_compute_subnetwork.main.name
      }
    }
    containers {
      image   = var.image
      command = ["brain-worker"]
      resources { limits = { cpu = "1", memory = "1Gi" } }
      dynamic "env" {
        for_each = merge(local.shared_env, { DATABASE_URL = local.db_urls.worker })
        content {
          name  = env.key
          value = env.value
        }
      }
    }
  }
}
resource "google_cloud_run_v2_job" "migrate" {
  count               = var.image != "" ? 1 : 0
  name                = "${var.name}-migrate"
  location            = var.region
  deletion_protection = var.deletion_protection
  template {
    template {
      service_account = google_service_account.app["migration"].email
      max_retries     = 0
      vpc_access {
        egress = "PRIVATE_RANGES_ONLY"
        network_interfaces {
          network    = google_compute_network.main.name
          subnetwork = google_compute_subnetwork.main.name
        }
      }
      containers {
        image = var.image
        args  = ["migrate"]
        dynamic "env" {
          for_each = merge(local.shared_env, { DATABASE_URL = local.db_urls.migration })
          content {
            name  = env.key
            value = env.value
          }
        }
      }
    }
  }
}
output "service_url" { value = try(google_cloud_run_v2_service.api[0].uri, null) }
output "database_users" { value = { for k, u in google_sql_user.iam : k => u.name } }
output "image_repository" { value = "${var.region}-docker.pkg.dev/${var.project_id}/${var.name}/brain" }
output "bootstrap_secret" { value = google_secret_manager_secret.bootstrap.secret_id }
