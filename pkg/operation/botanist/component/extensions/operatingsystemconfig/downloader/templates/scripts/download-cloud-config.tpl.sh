#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

{
SECRET_NAME="{{ .secretName }}"
TOKEN_SECRET_NAME="{{ .tokenSecretName }}"

PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT="{{ .pathCredentialsClientCert }}"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY="{{ .pathCredentialsClientKey }}"
PATH_CLOUDCONFIG_DOWNLOADER_TOKEN="{{ .pathCredentialsToken }}"
PATH_BOOTSTRAP_TOKEN="{{ .pathBootstrapToken }}"
PATH_EXECUTOR_SCRIPT="{{ .pathDownloadedExecutorScript }}"
PATH_EXECUTOR_SCRIPT_CHECKSUM="{{ .pathDownloadedChecksum }}"

mkdir -p "{{ .pathDownloadsDirectory }}"

function readSecret() {
  wget \
    -qO- \
    --ca-certificate "{{ .pathCredentialsCACert }}" \
    "${@:2}" "$(cat "{{ .pathCredentialsServer }}")/api/v1/namespaces/kube-system/secrets/$1"
}

function readSecretFull() {
  readSecret "$1" "--header=Accept: application/yaml" "${@:2}"
}

function readSecretMeta() {
  readSecret "$1" "--header=Accept: application/yaml;as=PartialObjectMetadata;g=meta.k8s.io;v=v1,application/yaml;as=PartialObjectMetadata;g=meta.k8s.io;v=v1" "${@:2}"
}

function readSecretMetaWithToken() {
  readSecretMeta "$1" "--header=Authorization: Bearer $2"
}

function readSecretWithToken() {
  readSecretFull "$1" "--header=Authorization: Bearer $2"
}

function readSecretWithClientCertificate() {
  readSecretFull "$1" "--certificate=$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT" "--private-key=$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY"
}

function extractDataKeyFromSecret() {
  echo "$1" | sed -rn "s/  $2: (.*)/\1/p" | base64 -d
}

function extractChecksumFromSecret() {
  echo "$1" | sed -rn 's/    {{ .annotationChecksum | replace "/" "\\/" }}: (.*)/\1/p' | sed -e 's/^"//' -e 's/"$//'
}

function writeToDiskSafely() {
  local data="$1"
  local file_path="$2"

  if echo "$data" > "$file_path.tmp" && ( [[ ! -f "$file_path" ]] || ! diff "$file_path" "$file_path.tmp">/dev/null ); then
    mv "$file_path.tmp" "$file_path"
  elif [[ -f "$file_path.tmp" ]]; then
    rm -f "$file_path.tmp"
  fi
}

# download shoot access token for cloud-config-downloader
if [[ -f "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN" ]]; then
  if ! SECRET="$(readSecretWithToken "$TOKEN_SECRET_NAME" "$(cat "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN")")"; then
    echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with existing token"
    exit 1
  fi
else
  if [[ -f "$PATH_BOOTSTRAP_TOKEN" ]]; then
    if ! SECRET="$(readSecretWithToken "$TOKEN_SECRET_NAME" "$(cat "$PATH_BOOTSTRAP_TOKEN")")"; then
      echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with bootstrap token"
      exit 1
    fi
  else
    if ! SECRET="$(readSecretWithClientCertificate "$TOKEN_SECRET_NAME")"; then
      echo "Could not retrieve the shoot access secret with name $TOKEN_SECRET_NAME with client certificate"
      exit 1
    fi
  fi
fi

TOKEN="$(extractDataKeyFromSecret "$SECRET" "{{ .dataKeyToken }}")"
if [[ -z "$TOKEN" ]]; then
  echo "Token in shoot access secret $TOKEN_SECRET_NAME is empty"
  exit 1
fi
writeToDiskSafely "$TOKEN" "$PATH_CLOUDCONFIG_DOWNLOADER_TOKEN"

# download and run the cloud config execution script
if ! SECRET_META="$(readSecretMetaWithToken "$SECRET_NAME" "$TOKEN")"; then
  echo "Could not retrieve the metadata in secret with name $SECRET_NAME"
  exit 1
fi
NEW_CHECKSUM="$(extractChecksumFromSecret "$SECRET_META")"

OLD_CHECKSUM="<none>"
if [[ -f "$PATH_EXECUTOR_SCRIPT_CHECKSUM" ]]; then
  OLD_CHECKSUM="$(cat "$PATH_EXECUTOR_SCRIPT_CHECKSUM")"
fi

if [[ "$NEW_CHECKSUM" != "$OLD_CHECKSUM" ]]; then
  echo "Checksum of cloud config script has changed compared to what I had downloaded earlier (new: $NEW_CHECKSUM, old: $OLD_CHECKSUM). Fetching new script..."

  if ! SECRET="$(readSecretWithToken "$SECRET_NAME" "$TOKEN")"; then
    echo "Could not retrieve the cloud config script in secret with name $SECRET_NAME"
    exit 1
  fi

  SCRIPT="$(extractDataKeyFromSecret "$SECRET" "{{ .dataKeyScript }}")"
  if [[ -z "$SCRIPT" ]]; then
    echo "Script in cloud config secret $SECRET is empty"
    exit 1
  fi

  writeToDiskSafely "$SCRIPT" "$PATH_EXECUTOR_SCRIPT" && chmod +x "$PATH_EXECUTOR_SCRIPT"
  writeToDiskSafely "$(extractChecksumFromSecret "$SECRET")" "$PATH_EXECUTOR_SCRIPT_CHECKSUM"
fi

# TODO(rfranzke): Delete in future release.
rm -f "{{ .pathDownloadsDirectory }}/downloaded_checksum"

"$PATH_EXECUTOR_SCRIPT"
exit $?
}
