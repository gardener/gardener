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

resource "azurerm_subnet" "subnet_workers" {
  name                      = "workers"
  resource_group_name       = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  virtual_network_name      = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  address_prefix            = "{{ required "networks.worker is required" .Values.networks.worker }}"
  route_table_id            = "${azurerm_route_table.route_table.id}"
  network_security_group_id = "${azurerm_network_security_group.nodes.id}"
}

{{ if .Values.networks.public -}}
resource "azurerm_subnet" "subnet_public_utility" {
  name                      = "public_utility"
  resource_group_name       = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  virtual_network_name      = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
  address_prefix            = "{{ required "networks.public is required" .Values.networks.public }}"
  network_security_group_id = "${azurerm_network_security_group.bastion.id}"
}

resource "azurerm_network_security_group" "bastion" {
  name                = "bastion"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"

  security_rule {
    name                       = "ssh"
    priority                   = 100
    direction                  = "Inbound"
    access                     = "Allow"
    protocol                   = "Tcp"
    source_port_range          = "*"
    destination_port_range     = "22"
    source_address_prefix      = "Internet"
    destination_address_prefix = "*"
  }
}
{{- end}}

resource "azurerm_route_table" "route_table" {
  name                = "worker_route_table"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

resource "azurerm_network_security_group" "nodes" {
  name                = "nodes"
  location            = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

#=====================================================================
#= Availability Set
#=====================================================================

resource "azurerm_availability_set" "workers_av" {
  name                         = "workers-avset"
  resource_group_name          = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
  location                     = "{{ required "azure.region is required" .Values.azure.region }}"
  platform_update_domain_count = "{{ required "azure.countUpdateDomains is required" .Values.azure.countUpdateDomains }}"
  platform_fault_domain_count  = "{{ required "azure.countFaultDomains is required" .Values.azure.countFaultDomains }}"
  managed                      = true
}

//=====================================================================
//= Workers
//=====================================================================

{{ range $j, $worker := .Values.workers }}
resource "azurerm_network_interface" "{{ $worker.name }}_nci" {
  name                 = "{{ $worker.name }}-${count.index}-nci"
  count                = "{{ required "worker.autoScalerMin is required" $worker.autoScalerMin }}"
  location             = "{{ required "azure.region is required" $.Values.azure.region }}"
  resource_group_name  = "{{ required "resourceGroup.name is required" $.Values.resourceGroup.name }}"
  enable_ip_forwarding = "true"

  ip_configuration {
    name                          = "{{ $worker.name }}-${count.index}-ip-conf"
    subnet_id                     = "${azurerm_subnet.subnet_workers.id}"
    private_ip_address_allocation = "dynamic"
  }
}

resource "azurerm_virtual_machine" "{{ $worker.name }}_node" {
  name                          = "{{ $worker.name }}-${count.index}"
  count                         = "{{ required "worker.autoScalerMin is required" $worker.autoScalerMin }}"
  location                      = "{{ required "azure.region is required" $.Values.azure.region }}"
  resource_group_name           = "{{ required "resourceGroup.name is required" $.Values.resourceGroup.name }}"
  vm_size                       = "{{ required "worker.machineType is required" $worker.machineType }}"
  network_interface_ids         = ["${element(azurerm_network_interface.{{ $worker.name }}_nci.*.id, count.index)}"]
  availability_set_id           = "${azurerm_availability_set.workers_av.id}"
  delete_os_disk_on_termination = true

  storage_image_reference {
    publisher = "CoreOS"
    offer     = "CoreOS"
    sku       = "{{ required "coreOSImage.sku is required" $.Values.coreOSImage.sku }}"
    version   = "{{ required "coreOSImage.version is required" $.Values.coreOSImage.version }}"
  }

  storage_os_disk {
    name              = "{{ $worker.name }}-${count.index}-os-disk"
    caching           = "None"
    create_option     = "FromImage"
    disk_size_gb      = "{{ regexFind "^(\\d+)" (required "worker.volumeSize is required" $worker.volumeSize) }}"
    managed_disk_type = "{{if eq $worker.volumeType "premium"}}Premium_LRS{{else}}Standard_LRS{{end}}"
  }

  tags {
    "Name"                                                                                = "nodes-{{ required "worker.machineType is required" $worker.machineType }}-{{ required "clusterName is required" $.Values.clusterName }}"
    "kubernetes.io-cluster-{{ required "clusterName is required" $.Values.clusterName }}" = "1"
    "kubernetes.io-role-node"                                                             = "1"
  }

  os_profile_linux_config {
    disable_password_authentication = true
    ssh_keys                        = [{
      path     = "/home/core/.ssh/authorized_keys"
      key_data = "{{ required "sshPublicKey is required" $.Values.sshPublicKey }}"
    }]
  }

  os_profile {
    computer_name  = "{{ $worker.name }}-${count.index}"
    admin_username = "core"
    custom_data    = <<EOF
{{ include "terraformer-common.cloud-config.user-data" (set $.Values "workerName" $worker.name) }}
EOF
  }
}
{{- end}}

//=====================================================================
//= Output variables
//=====================================================================

output "resourceGroupName" {
  value = "{{ required "resourceGroup.name is required" .Values.resourceGroup.name }}"
}

output "vnetName" {
  value = "{{ required "resourceGroup.vnet.name is required" .Values.resourceGroup.vnet.name }}"
}

{{- end -}}
