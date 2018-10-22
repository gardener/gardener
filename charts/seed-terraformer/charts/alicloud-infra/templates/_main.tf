{{- define "alicloud-infra.main" -}}
provider "alicloud" {
  access_key = "${var.ACCESS_KEY_ID}"
  secret_key = "${var.ACCESS_KEY_SECRET}"
  region = "{{ required "alicloud.region is required" .Values.alicloud.region }}"
}

// Import an existing public key to build a alicloud key pair
resource "alicloud_key_pair" "publickey" {
    key_name = "{{ required "clusterName is required" .Values.clusterName }}-ssh-publickey"
    public_key = "{{ required "sshPublicKey is required" .Values.sshPublicKey }}"
}

resource "alicloud_vpc" "vpc" {
  name       = "{{ required "clusterName is required" .Values.clusterName }}-vpc"
  cidr_block = "{{ required "vpc.cidr is required" .Values.vpc.cidr }}"
}


// Loop zones
{{ range $index, $zone := .Values.zones }}

resource "alicloud_vswitch" "vsw_z{{ $index }}" {
  name              = "{{ required "clusterName is required" $.Values.clusterName }}-{{ required "zone.name is required" $zone.name }}-vsw"
  vpc_id            = "${alicloud_vpc.vpc.id}"
  cidr_block        = "{{ required "zone.cidr.worker is required" $zone.cidr.worker }}"
  availability_zone = "{{ required "zone.name is required" $zone.name }}"
}

// Output
output "vswitch_id_z{{ $index }}" {
  value = "${alicloud_vswitch.vsw_z{{ $index }}.id}"
}

{{end}}
// End of loop zones

resource "alicloud_security_group" "sg" {
  name   = "{{ required "clusterName is required" .Values.clusterName }}-sg"
  vpc_id = "${alicloud_vpc.vpc.id}"
}

resource "alicloud_security_group_rule" "allow_k8s_tcp_in" {
  type              = "ingress"
  ip_protocol       = "tcp"
  policy            = "accept"
  port_range        = "30000/32767"
  priority          = 1
  security_group_id = "${alicloud_security_group.sg.id}"
  cidr_ip           = "0.0.0.0/0"
}

resource "alicloud_security_group_rule" "allow_all_internal_tcp_in" {
  type              = "ingress"
  ip_protocol       = "tcp"
  policy            = "accept"
  port_range        = "1/65535"
  priority          = 1
  security_group_id = "${alicloud_security_group.sg.id}"
  cidr_ip           = "{{ required "pod is required" .Values.vpc.cidr }}"
}

resource "alicloud_security_group_rule" "allow_all_internal_udp_in" {
  type              = "ingress"
  ip_protocol       = "udp"
  policy            = "accept"
  port_range        = "1/65535"
  priority          = 1
  security_group_id = "${alicloud_security_group.sg.id}"
  cidr_ip           = "{{ required "pod is required" .Values.vpc.cidr }}"
}
 
//=====================================================================
//= Output variables
//=====================================================================

output "sg_id" {
  value = "${alicloud_security_group.sg.id}"
}

output "vpc_id" {
  value = "${alicloud_vpc.vpc.id}"
}

output "key_pair_name" {
  value = "${alicloud_key_pair.publickey.key_name}"
}
{{- end -}}