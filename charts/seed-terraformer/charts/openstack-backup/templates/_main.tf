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
  region        = "{{ required "openstack.region is required" .Values.openstack.region }}"
  name          = "{{ required "container.name is required" .Values.container.name }}"
  force_destroy = true
}

//=====================================================================
//= Output variables
//=====================================================================

output "containerName" {
  value = "${openstack_objectstorage_container_v1.container.name}"
}
{{- end -}}
