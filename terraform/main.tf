terraform {
  required_providers {
    google = {
      source = "hashicorp/google"
      version = "4.12.0"
    }
  }
}

provider "google" {
  credentials = file("gcp.json")

  project = "eminent-hall-342816"
  region  = "us-central1"
  zone    = "us-central1-c"
}

resource "google_compute_network" "vpc_network" {
  name = "terraform-network"
}

module "vault" {
  source  = "picatz/vault/google"
  version = "0.0.8"
  # insert the 1 required variable here
  project = "eminent-hall-342816"
}