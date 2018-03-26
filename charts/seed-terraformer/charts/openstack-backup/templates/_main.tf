{{- define "openstack-backup.main" -}}
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
//= Swift container
//=====================================================================

resource "openstack_objectstorage_container_v1" "container" {
  region = "{{ required "openstack.region is required" .Values.openstack.region }}"
  name   = "{{ required "container.name is required" .Values.container.name }}"
}

/*resource "openstack_identity_user_v3" "user" {
  name =  "{{ required "clusterName is required" .Values.clusterName }}-etcd-backup" 
  description = "{{ required "clusterName is required" .Values.clusterName }}-backup user"
  ignore_change_password_upon_first_use = true
  ignore_password_expiry = true
  multi_factor_auth_enabled = false
  password = "password123"
}

//=====================================================================
//= Output variables
//=====================================================================

/*
output "username" {
  value =  "${openstack_identity_user_v3.user.name}"
}

output "password" {
  sensitive = true
  value     = "${openstack_identity_user_v3.user.password}"
}
*/
output "containerName" {
  value = "${openstack_objectstorage_container_v1.container.name}"
}
{{- end -}}