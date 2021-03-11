#!/bin/bash -eu

SECRET_NAME="{{ .secretName }}"

PATH_CLOUDCONFIG_DOWNLOADER_SERVER="{{ .pathCredentialsServer }}"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="{{ .pathCredentialsCACert }}"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT="{{ .pathCredentialsClientCert }}"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY="{{ .pathCredentialsClientKey }}"
PATH_CLOUDCONFIG_CHECKSUM="{{ .pathDownloadedChecksum }}"

if ! SECRET="$(wget \
  -qO- \
  --header         "Accept: application/yaml" \
  --ca-certificate "$PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT" \
  --certificate    "$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT" \
  --private-key    "$PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY" \
  "$(cat "$PATH_CLOUDCONFIG_DOWNLOADER_SERVER")/api/v1/namespaces/kube-system/secrets/$SECRET_NAME")"; then

  echo "Could not retrieve the cloud config script in secret with name $SECRET_NAME"
  exit 1
fi

CHECKSUM="$(echo "$SECRET" | sed -rn 's/    {{ .annotationChecksum | replace "/" "\\/" }}: (.*)/\1/p' | sed -e 's/^"//' -e 's/"$//')"
echo "$CHECKSUM" > "$PATH_CLOUDCONFIG_CHECKSUM"

SCRIPT="$(echo "$SECRET" | sed -rn 's/  {{ .dataKeyScript }}: (.*)/\1/p')"
echo "$SCRIPT" | base64 -d | bash
