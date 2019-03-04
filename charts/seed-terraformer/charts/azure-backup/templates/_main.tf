{{- define "azure-backup.main" -}}
provider "azurerm" {
  subscription_id = "{{ required "azure.subscriptionID is required" .Values.azure.subscriptionID }}"
  tenant_id       = "{{ required "azure.tenantID is required" .Values.azure.tenantID }}"
  client_id       = "${var.CLIENT_ID}"
  client_secret   = "${var.CLIENT_SECRET}"
}

resource "azurerm_resource_group" "rg" {
  name     = "{{ required "azure.resourceGroupName is required" .Values.azure.resourceGroupName }}"
  location = "{{ required "azure.region is required" .Values.azure.region }}"
}

resource "azurerm_storage_account" "storageAccount" {
  name                      = "{{ required "azure.storageAccountName is required" .Values.azure.storageAccountName }}"
  location                  = "{{ required "azure.region is required" .Values.azure.region }}"
  resource_group_name       = "${azurerm_resource_group.rg.name}"
  account_kind              = "BlobStorage"
  access_tier               = "Hot"
  account_tier              = "Standard"
  account_replication_type  = "LRS"
  enable_blob_encryption    = true
  enable_https_traffic_only = true
}

resource "azurerm_storage_container" "container" {
  name                  = "backup-store"
  resource_group_name   = "${azurerm_resource_group.rg.name}"
  storage_account_name  = "${azurerm_storage_account.storageAccount.name}"
  container_access_type = "private"
}

//=====================================================================
//= Output variables
//=====================================================================

output "storageAccountName" {
  value = "${azurerm_storage_account.storageAccount.name}"
}

output "storageAccessKey" {
  sensitive = true
  value     = "${azurerm_storage_account.storageAccount.primary_access_key}"
}

output "containerName" {
  value = "${azurerm_storage_container.container.name}"
}
{{- end -}}
