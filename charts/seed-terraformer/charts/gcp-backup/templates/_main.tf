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
//= Output variables
//=====================================================================

output "bucketName" {
  value = "${google_storage_bucket.bucket.name}"
}
{{- end -}}