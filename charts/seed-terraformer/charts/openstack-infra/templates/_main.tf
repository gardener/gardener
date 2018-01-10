{{- define "openstack-infra.main" -}}
provider "openstack" {
  auth_url    = "{{ required "openstack.authURL is required" .Values.openstack.authURL }}"
  domain_name = "{{ required "openstack.domainName is required" .Values.openstack.domainName }}"
  tenant_name = "{{ required "openstack.tenantName is required" .Values.openstack.tenantName }}"
  region      = "{{ required "openstack.region is required" .Values.openstack.region }}"
  user_name   = "${var.USER_NAME}"
  password    = "${var.PASSWORD}"
  insecure    = true
}

//=====================================================================
//= Networking: Router/Interfaces/Net/SubNet/SecGroup/SecRules
//=====================================================================

data "openstack_networking_network_v2" "fip" {
  name = "{{ required "openstack.floatingPoolName is required" .Values.openstack.floatingPoolName }}"
}

{{ if .Values.create.router -}}
resource "openstack_networking_router_v2" "router" {
  name             = "{{ required "clusterName is required" .Values.clusterName }}"
  region           = "{{ required "openstack.region is required" .Values.openstack.region }}"
  external_gateway = "${data.openstack_networking_network_v2.fip.id}"
}
{{- end}}

resource "openstack_networking_network_v2" "cluster" {
  name           = "{{ required "clusterName is required" .Values.clusterName }}"
  admin_state_up = "true"
}

resource "openstack_networking_subnet_v2" "cluster" {
  name            = "{{ required "clusterName is required" .Values.clusterName }}"
  cidr            = "{{ required "networks.worker is required" .Values.networks.worker }}"
  network_id      = "${openstack_networking_network_v2.cluster.id}"
  ip_version      = 4
  dns_nameservers = []
}

resource "openstack_networking_router_interface_v2" "router_nodes" {
  router_id = "{{ required "router.id is required" $.Values.router.id }}"
  subnet_id = "${openstack_networking_subnet_v2.cluster.id}"
}

resource "openstack_networking_secgroup_v2" "cluster" {
  name                 = "{{ required "clusterName is required" .Values.clusterName }}"
  description          = "Cluster Nodes"
  delete_default_rules = true
}

resource "openstack_networking_secgroup_rule_v2" "cluster_self" {
  direction         = "ingress"
  ethertype         = "IPv4"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
  remote_group_id   = "${openstack_networking_secgroup_v2.cluster.id}"
}

resource "openstack_networking_secgroup_rule_v2" "cluster_egress" {
  direction         = "egress"
  ethertype         = "IPv4"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
}

resource "openstack_networking_secgroup_rule_v2" "cluster_tcp_all" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 1
  port_range_max    = 65535
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = "${openstack_networking_secgroup_v2.cluster.id}"
}

//=====================================================================
//= SSH Key for Nodes (Bastion and Worker)
//=====================================================================

resource "openstack_compute_keypair_v2" "ssh_key" {
  name       = "{{ required "clusterName is required" .Values.clusterName }}"
  public_key = "{{ required "sshPublicKey is required" .Values.sshPublicKey }}"
}

//=====================================================================
//= Bastion Host
//=====================================================================

data "openstack_images_image_v2" "bastion" {
  name        = "{{ required "coreOSImage is required" $.Values.coreOSImage }}"
  most_recent = true
}

resource "openstack_networking_floatingip_v2" "bastion" {
  pool = "{{ required "openstack.floatingPoolName is required" .Values.openstack.floatingPoolName }}"
}

resource "openstack_compute_floatingip_associate_v2" "bastion" {
  floating_ip = "${openstack_networking_floatingip_v2.bastion.address}"
  instance_id = "${openstack_compute_instance_v2.bastion.id}"
}

resource "openstack_compute_instance_v2" "bastion" {
  name              = "{{ required "clusterName is required" .Values.clusterName }}-bastion"
  availability_zone = "{{ required "zones is required" (index .Values.zones 0).name }}"
  flavor_name       = "m1.small"
  image_id          = "${data.openstack_images_image_v2.bastion.id}"
  key_pair          = "${openstack_compute_keypair_v2.ssh_key.name}"
  security_groups   = ["${openstack_networking_secgroup_v2.cluster.name}"]
  force_delete      = true

  network {
    uuid = "${openstack_networking_network_v2.cluster.id}"
  }
}

//=====================================================================
//= Worker Nodes
//=====================================================================

data "openstack_images_image_v2" "worker" {
  name        = "{{ required "coreOSImage is required" $.Values.coreOSImage }}"
  most_recent = true
}

{{ range $j, $worker := .Values.workers }}
{{ range $zoneIndex, $zone := $worker.zones }}
resource "openstack_compute_instance_v2" "worker-{{ $zoneIndex }}-{{ $j }}" {
  name              = "{{ required "clusterName is required" $.Values.clusterName }}-{{ required "worker.name is required" $worker.name }}-${count.index}"
  count             = "{{ required "zone.autoScalerMin is required" $zone.autoScalerMin }}"
  flavor_name       = "{{ required "worker.machineType is required" $worker.machineType }}"
  availability_zone = "{{ required "zone.name is required" $zone.name }}"
  image_id          = "${data.openstack_images_image_v2.worker.id}"
  key_pair          = "${openstack_compute_keypair_v2.ssh_key.name}"
  security_groups   = ["${openstack_networking_secgroup_v2.cluster.name}"]
  force_delete      = true

  network {
    uuid = "${openstack_networking_network_v2.cluster.id}"
  }

  user_data = <<EOF
{{ include "terraformer-common.cloud-config.user-data" (set $.Values "workerName" $worker.name) }}
EOF
}
{{- end }}
{{- end }}

output "cloud_config" {
  value = <<EOF
[Global]
auth-url={{ required "openstack.authURL is required" .Values.openstack.authURL }}
domain-name={{ required "openstack.domainName is required" .Values.openstack.domainName }}
tenant-name={{ required "openstack.tenantName is required" .Values.openstack.tenantName }}
username=${var.USER_NAME}
password=${var.PASSWORD}
[LoadBalancer]
lb-version=v2
lb-provider={{ required "openstack.loadBalancerProvider is required" .Values.openstack.loadBalancerProvider }}
floating-network-id=${data.openstack_networking_network_v2.fip.id}
subnet-id=${openstack_networking_subnet_v2.cluster.id}
create-monitor=true
monitor-delay=60s
monitor-timeout=30s
monitor-max-retries=5
EOF
}
{{- end -}}
