resource "google_compute_network" "dev-network" {
  name                     = var.name
  auto_create_subnetworks  = false
  enable_ula_internal_ipv6 = false
}

resource "google_compute_subnetwork" "dev-subnetwork" {
  name   = var.name
  region = var.region

  network          = google_compute_network.dev-network.id
  stack_type       = "IPV4_IPV6"
  ip_cidr_range    = "10.0.0.0/22"
  ipv6_access_type = "EXTERNAL"
}

resource "google_compute_firewall" "dev-firewall" {
  name    = var.name
  network = google_compute_network.dev-network.name

  source_ranges = ["0.0.0.0/0"]
  allow {
    protocol = "icmp"
  }
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }

  target_tags = ["allow-ssh"]
}
