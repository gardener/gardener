{{- define "azure-infra.main" -}}
provider "azurerm" {
  subscription_id = "{{ required "azure.subscriptionID is required" .Values.azure.subscriptionID }}"
  tenant_id       = "{{ required "azure.tenantID is required" .Values.azure.tenantID }}"
  client_id       = "${var.CLIENT_ID}"
  client_secret   = "${var.CLIENT_SECRET}"
}

{{ if .Values.create.resourceGroup -}}
resource "azurerm_resource_group" "rg" {
  name     = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  location = "{{ required "azure.region is required" .Values.azure.region }}"
}
{{- end}}

#=====================================================================
#= VNet, Subnets, Route Table, Security Groups
#=====================================================================

{{ if .Values.create.vnet -}}
resource "azurerm_virtual_network" "vnet" {
  name                = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  address_space       = ["{{ required "resourceGroup.vnet.cidr is required" .Values.resourceGroup.vnet.cidr }}"]
}
{{- end}}

resource "azurerm_subnet" "workers" {
  name                      = "{{ required "clusterName is required" .Values.clusterName }}-nodes"
  resource_group_name       = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  virtual_network_name      = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  address_prefix            = "{{ required "networks.worker is required" .Values.networks.worker }}"
  route_table_id            = "${azurerm_route_table.workers.id}"
  network_security_group_id = "${azurerm_network_security_group.workers.id}"
}

resource "azurerm_route_table" "workers" {
  name                = "worker_route_table"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

resource "azurerm_network_security_group" "workers" {
  name                = "{{ required "clusterName is required" .Values.clusterName }}-workers"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

#=====================================================================
#= Availability Set
#=====================================================================

resource "azurerm_availability_set" "workers" {
  name                         = "{{ required "clusterName is required" .Values.clusterName }}-avset-workers"
  resource_group_name          = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  location                     = "{{ required "azure.region is required" .Values.azure.region }}"
  platform_update_domain_count = "{{ required "azure.countUpdateDomains is required" .Values.azure.countUpdateDomains }}"
  platform_fault_domain_count  = "{{ required "azure.countFaultDomains is required" .Values.azure.countFaultDomains }}"
  managed                      = true
}

//=====================================================================
//= Output variables
//=====================================================================

output "resourceGroupName" {
  value = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

output "vnetName" {
  value = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
}

output "subnetName" {
  value = "${azurerm_subnet.workers.name}"
}

output "availabilitySetID" {
  value = "${azurerm_availability_set.workers.id}"
}

output "availabilitySetName" {
  value = "${azurerm_availability_set.workers.name}"
}

output "routeTableName" {
  value = "${azurerm_route_table.workers.name}"
}

output "securityGroupName" {
  value = "${azurerm_network_security_group.workers.name}"
}
{{- end -}}
