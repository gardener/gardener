{{- define "gcp-infra.main" -}}
provider "google" {
  credentials = "${var.SERVICEACCOUNT}"
  project     = "{{ required "google.project is required" .Values.google.project }}"
  region      = "{{ required "google.region is required" .Values.google.region }}"
}

//=====================================================================
//= Service Account
//=====================================================================

resource "google_service_account" "serviceaccount" {
  account_id   = "{{ required "clusterName is required" .Values.clusterName }}"
  display_name = "{{ required "clusterName is required" .Values.clusterName }}"
}

//=====================================================================
//= Networks
//=====================================================================

{{ if .Values.create.vpc -}}
resource "google_compute_network" "network" {
  name                    = "{{ required "clusterName is required" .Values.clusterName }}"
  auto_create_subnetworks = "false"
}
{{- end}}

resource "google_compute_subnetwork" "subnetwork-nodes" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-nodes"
  ip_cidr_range = "{{ required "networks.worker is required" .Values.networks.worker }}"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  region        = "{{ required "google.region is required" .Values.google.region }}"
}

{{ if .Values.networks.internal -}}
resource "google_compute_subnetwork" "subnetwork-internal" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-internal"
  ip_cidr_range = "{{ required "networks.internal is required" .Values.networks.internal }}"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  region        = "{{ required "google.region is required" .Values.google.region }}"
}
{{- end}}
//=====================================================================
//= Firewall
//=====================================================================

// Allow traffic within internal network range.
resource "google_compute_firewall" "rule-allow-internal-access" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-internal-access"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = ["10.0.0.0/8"]

  allow {
    protocol = "icmp"
  }

  allow {
    protocol = "ipip"
  }

  allow {
    protocol = "tcp"
    ports    = ["1-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["1-65535"]
  }
}

resource "google_compute_firewall" "rule-allow-external-access" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-external-access"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = ["0.0.0.0/0"]

  allow {
    protocol = "tcp"
    ports    = ["80", "443"] // Allow ingress
  }
}

// Required to allow Google to perform health checks on our instances.
// https://cloud.google.com/compute/docs/load-balancing/internal/
// https://cloud.google.com/compute/docs/load-balancing/network/
resource "google_compute_firewall" "rule-allow-health-checks" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-health-checks"
  network       = "{{ required "vpc.name is required" .Values.vpc.name }}"
  source_ranges = [
    "35.191.0.0/16",
    "209.85.204.0/22",
    "209.85.152.0/22",
    "130.211.0.0/22",
  ]

  allow {
    protocol = "tcp"
    ports    = ["30000-32767"]
  }

  allow {
    protocol = "udp"
    ports    = ["30000-32767"]
  }
}

// We have introduced new output variables. However, they are not applied for
// existing clusters as Terraform won't detect a diff when we run `terraform plan`.
// Workaround: Providing a null-resource for letting Terraform think that there are
// differences, enabling the Gardener to start an actual `terraform apply` job.
resource "null_resource" "outputs" {
  triggers = {
    recompute = "outputs"
  }
}

//=====================================================================
//= Output variables
//=====================================================================

output "vpc_name" {
  value = "{{ required "vpc.name is required" .Values.vpc.name }}"
}

output "service_account_email" {
  value = "${google_service_account.serviceaccount.email}"
}

output "subnet_nodes" {
  value = "${google_compute_subnetwork.subnetwork-nodes.name}"
}
{{ if .Values.networks.internal -}}
output "subnet_internal" {
  value = "${google_compute_subnetwork.subnetwork-internal.name}"
}
{{- end}}
{{- end -}}
