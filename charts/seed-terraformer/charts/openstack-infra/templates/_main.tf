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
  name                = "{{ required "clusterName is required" .Values.clusterName }}"
  region              = "{{ required "openstack.region is required" .Values.openstack.region }}"
  external_network_id = "${data.openstack_networking_network_v2.fip.id}"
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
  {{- if .Values.dnsServers }}
  dns_nameservers = [{{- include "openstack-infra.dnsServers" . | trimSuffix ", " }}]
  {{- else }}
  dns_nameservers = []
  {{- end }}
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
//= Output Variables
//=====================================================================

output "router_id" {
  value = "{{ required "router.id is required" .Values.router.id }}"
}

output "network_id" {
  value = "${openstack_networking_network_v2.cluster.id}"
}

output "key_name" {
  value = "${openstack_compute_keypair_v2.ssh_key.name}"
}

output "security_group_id" {
  value = "${openstack_networking_secgroup_v2.cluster.id}"
}

output "security_group_name" {
  value = "${openstack_networking_secgroup_v2.cluster.name}"
}

output "floating_network_id" {
  value = "${data.openstack_networking_network_v2.fip.id}"
}

output "subnet_id" {
  value = "${openstack_networking_subnet_v2.cluster.id}"
}
{{- end -}}
