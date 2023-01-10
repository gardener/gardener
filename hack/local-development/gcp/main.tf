terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "~> 4.38.0"
    }
    local = {
      source = "hashicorp/local"
      version = "~> 2.2.3"
    }
  }
}

provider "google" {
  credentials = var.serviceaccount_file

  project = var.project
  region  = var.region
  zone    = var.zone
}
