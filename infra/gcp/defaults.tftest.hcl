mock_provider "google" {}
run "small_default" {
  command = plan
  variables { project_id = "trace2mem-test-project" }
  assert {
    condition     = google_sql_database_instance.main.settings[0].availability_type == "ZONAL"
    error_message = "Default database must remain zonal."
  }
  assert {
    condition     = length(google_cloud_run_v2_service.api) == 0
    error_message = "Bootstrap must not deploy the API before migration."
  }
}
