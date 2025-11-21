# Terraform configuration for Cloud Run with IAP
# This is a reference implementation - adjust for your infrastructure

variable "project_id" {
  description = "GCP Project ID"
  type        = string
}

variable "region" {
  description = "GCP Region"
  type        = string
  default     = "us-central1"
}

variable "domain" {
  description = "Custom domain for the service"
  type        = string
}

variable "iap_members" {
  description = "List of members to grant IAP access"
  type        = list(string)
  default     = []
}

# Enable required APIs
resource "google_project_service" "iap" {
  project = var.project_id
  service = "iap.googleapis.com"
}

resource "google_project_service" "compute" {
  project = var.project_id
  service = "compute.googleapis.com"
}

# Serverless NEG for Cloud Run
resource "google_compute_region_network_endpoint_group" "cloudrun_neg" {
  name                  = "aixgo-cloudrun-neg"
  project               = var.project_id
  region                = var.region
  network_endpoint_type = "SERVERLESS"

  cloud_run {
    service = "aixgo-mcp"
  }
}

# Backend service with IAP enabled
resource "google_compute_backend_service" "default" {
  name        = "aixgo-backend"
  project     = var.project_id
  protocol    = "HTTP"
  port_name   = "http"
  timeout_sec = 300

  backend {
    group = google_compute_region_network_endpoint_group.cloudrun_neg.id
  }

  iap {
    oauth2_client_id     = google_iap_client.default.client_id
    oauth2_client_secret = google_iap_client.default.secret
  }

  log_config {
    enable = true
  }
}

# IAP OAuth client
resource "google_iap_client" "default" {
  display_name = "aixgo-iap-client"
  brand        = google_iap_brand.default.name
}

# IAP brand (OAuth consent screen)
resource "google_iap_brand" "default" {
  support_email     = "admin@${var.domain}"
  application_title = "aixgo MCP Server"
  project           = var.project_id
}

# URL map
resource "google_compute_url_map" "default" {
  name            = "aixgo-url-map"
  project         = var.project_id
  default_service = google_compute_backend_service.default.id
}

# HTTPS proxy
resource "google_compute_target_https_proxy" "default" {
  name             = "aixgo-https-proxy"
  project          = var.project_id
  url_map          = google_compute_url_map.default.id
  ssl_certificates = [google_compute_managed_ssl_certificate.default.id]
}

# Managed SSL certificate
resource "google_compute_managed_ssl_certificate" "default" {
  name    = "aixgo-ssl-cert"
  project = var.project_id

  managed {
    domains = [var.domain]
  }
}

# Global forwarding rule
resource "google_compute_global_forwarding_rule" "default" {
  name       = "aixgo-forwarding-rule"
  project    = var.project_id
  target     = google_compute_target_https_proxy.default.id
  port_range = "443"
  ip_address = google_compute_global_address.default.address
}

# Static IP address
resource "google_compute_global_address" "default" {
  name    = "aixgo-ip"
  project = var.project_id
}

# IAP access policy
resource "google_iap_web_backend_service_iam_binding" "default" {
  project             = var.project_id
  web_backend_service = google_compute_backend_service.default.name
  role                = "roles/iap.httpsResourceAccessor"
  members             = var.iap_members
}

# Outputs
output "load_balancer_ip" {
  description = "IP address of the load balancer"
  value       = google_compute_global_address.default.address
}

output "iap_audience" {
  description = "IAP audience for JWT verification"
  value       = "/projects/${var.project_id}/global/backendServices/${google_compute_backend_service.default.name}"
}

output "oauth_client_id" {
  description = "OAuth client ID for IAP"
  value       = google_iap_client.default.client_id
  sensitive   = true
}
