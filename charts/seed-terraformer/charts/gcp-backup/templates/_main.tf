{{- define "gcp-backup.main" -}}
provider "google" {
  credentials = "${var.SERVICEACCOUNT}"
  project     = "{{ required "google.project is required" .Values.google.project }}"
  region      = "{{ required "google.region is required" .Values.google.region }}"
}


//=====================================================================
//= GCS bucket
//=====================================================================

resource "google_storage_bucket" "bucket" {
  name          = "{{ required "bucket.name is required" .Values.bucket.name }}"
  location      = "{{ required "google.region is required" .Values.google.region }}"
  force_destroy = true
  storage_class = "COLDLINE"

}

//=====================================================================
//= Service Account
//=====================================================================

resource "google_service_account" "etcdBackup" {
  account_id   = "{{ required "clusterName is required" .Values.clusterName }}-b"
  display_name = "{{ required "clusterName is required" .Values.clusterName }}-b"
}

resource "google_service_account_key" "serviceAccountKey" {
  service_account_id = "${google_service_account.etcdBackup.id}"
}

//=====================================================================
//= GCP iam policy
//=====================================================================

resource "google_storage_bucket_iam_member" "memberRole" {
 bucket  = "${google_storage_bucket.bucket.name}"
 role    = "roles/storage.admin"
 member  = "serviceAccount:${google_service_account.etcdBackup.email}",
}

//=====================================================================
//= Output variables
//=====================================================================

output "serviceAccountJson" {
  value = "${base64decode(google_service_account_key.serviceAccountKey.private_key)}"
  sensitive = true
}

output "bucketName" {
  value = "${google_storage_bucket.bucket.name}"
}
{{- end -}}