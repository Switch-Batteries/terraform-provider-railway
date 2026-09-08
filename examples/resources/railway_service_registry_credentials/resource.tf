resource "railway_service_registry_credentials" "example" {
  environment_id = railway_project.example.default_environment.id
  service_id     = "b56f3b11-cf1a-455a-b6e8-8f69d29d8dc3"
  username       = "github-user"
  password       = var.ghcr_token
}
