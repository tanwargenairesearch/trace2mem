variable "project_id" { type = string }
variable "region" {
  type    = string
  default = "europe-west2"
}
variable "name" {
  type    = string
  default = "brain"
}
variable "image" {
  type        = string
  description = "Immutable application image digest, required when deploy_app=true"
  default     = ""
  validation {
    condition     = var.image == "" || can(regex("@sha256:[a-f0-9]{64}$", var.image))
    error_message = "Use an immutable image digest."
  }
}
variable "deploy_app" {
  type    = bool
  default = false
}
variable "ha" {
  type    = bool
  default = false
}
variable "database_tier" {
  type    = string
  default = "db-custom-1-3840"
}
variable "api_max_instances" {
  type    = number
  default = 3
}
variable "worker_instances" {
  type    = number
  default = 1
}
variable "public_url" {
  type    = string
  default = ""
}
variable "oidc_issuer" {
  type    = string
  default = ""
}
variable "oidc_client_id" {
  type    = string
  default = ""
}
variable "oauth_audience" {
  type    = string
  default = ""
}
variable "model_endpoints" {
  type    = string
  default = ""
}
variable "github_repository" {
  type        = string
  description = "Optional owner/repository for workload identity federation"
  default     = ""
}
variable "deletion_protection" {
  type    = bool
  default = true
}
