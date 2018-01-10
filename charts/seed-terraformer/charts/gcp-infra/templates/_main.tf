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
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
  region        = "{{ required "google.region is required" .Values.google.region }}"
}

//=====================================================================
//= Firewall
//=====================================================================

// Allow traffic within internal network range.
resource "google_compute_firewall" "rule-allow-internal-access" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-internal-access"
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
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

// Allow traffic between Kubernetes services.
resource "google_compute_firewall" "rule-allow-cluster-ips" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-cluster-ips"
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
  source_ranges = ["{{ required "networks.services is required" .Values.networks.services }}"]

  allow {
    protocol = "tcp"
    ports    = ["1-65535"]
  }

  allow {
    protocol = "udp"
    ports    = ["1-65535"]
  }
}

// Allow traffic between Kubernetes pods.
resource "google_compute_firewall" "rule-allow-pod-cidr" {
  name          = "{{ required "clusterName is required" .Values.clusterName }}-allow-pod-cidr"
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
  source_ranges = ["{{ required "networks.pods is required" .Values.networks.pods }}"]

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
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
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
  network       = "{{ required "vpc.name is required" $.Values.vpc.name }}"
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

//=====================================================================
//= Autoscaler, instance group manager and instance templates
//=====================================================================
{{ range $j, $worker := .Values.workers }}
resource "google_compute_instance_template" "shoot-template-{{ required "worker.name is required" $worker.name }}" {
  name                 = "{{ required "clusterName is required" $.Values.clusterName }}-shoot-template-{{ required "worker.name is required" $worker.name }}"
  description          = "Shoot worker group {{ required "worker.name is required" $worker.name }}"
  instance_description = "Kubernetes Shoot node"
  tags                 = ["{{ required "clusterName is required" $.Values.clusterName }}"]
  machine_type         = "{{ required "worker.machineType is required" $worker.machineType }}"
  region               = "{{ required "google.region is required" $.Values.google.region }}"
  can_ip_forward       = true // Allows this instance to send and receive packets with non-matching destination or source IPs

  scheduling {
    automatic_restart   = true // Restarts if instance was terminated by gcp not a user
    on_host_maintenance = "MIGRATE" // Google live migrates vm during maintenance
  }

  disk {
    source_image = "{{ required "coreOSImage is required" $.Values.coreOSImage }}"
    disk_type    = "{{ required "worker.volumeType is required" $worker.volumeType }}"
    disk_size_gb = {{ regexFind "^(\\d+)" (required "worker.volumeSize is required" $worker.volumeSize) }}
    auto_delete  = true
    boot         = true
  }

  service_account {
    scopes = [
      "https://www.googleapis.com/auth/compute",
    ]
    email = "${google_service_account.serviceaccount.email}"
  }

  network_interface {
    subnetwork = "${google_compute_subnetwork.subnetwork-nodes.name}"
    access_config {
      // Ephemeral IP
    }
  }
  metadata {
    user-data = <<EOF
{{ include "terraformer-common.cloud-config.user-data" (set $.Values "workerName" $worker.name) }}
EOF
  }
  lifecycle {
    create_before_destroy = true
  }
}
{{- end}}

{{ range $j, $worker := .Values.workers }}
{{ range $zoneIndex, $zone := $worker.zones }}
resource "google_compute_instance_group_manager" "igm-{{ $zoneIndex }}-{{ $j }}" {
  name               = "{{ required "clusterName is required" $.Values.clusterName }}-igm-{{ required "worker.name is required" $worker.name }}-z{{ $zoneIndex }}"
  base_instance_name = "{{ required "clusterName is required" $.Values.clusterName }}-zone-{{ required "zone.name is required" $zone.name }}"
  zone               = "{{ required "zone.name is required" $zone.name }}"
  instance_template  = "${google_compute_instance_template.shoot-template-{{ required "worker.name is required" $worker.name }}.self_link}"
}

resource "google_compute_autoscaler" "as-{{ $zoneIndex }}-{{ $j }}" {
  name   = "{{ required "clusterName is required" $.Values.clusterName }}-as-{{ required "worker.name is required" $worker.name }}-z{{ $zoneIndex }}"
  zone   = "{{ required "zone.name is required" $zone.name }}"
  target = "${google_compute_instance_group_manager.igm-{{ $zoneIndex }}-{{ $j }}.self_link}"

  autoscaling_policy = {
    min_replicas    = {{ required "zone.autoScalerMin is required" $zone.autoScalerMin }}
    max_replicas    = {{ required "zone.autoScalerMax is required" $zone.autoScalerMax }}
    cooldown_period = 60

    cpu_utilization {
      target = 0.8
    }
  }
}
{{- end -}}
{{- end -}}
{{- end -}}
