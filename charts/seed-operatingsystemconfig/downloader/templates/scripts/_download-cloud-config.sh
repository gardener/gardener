{{- define "seed-operatingsystemconfig.downloader.download-script" -}}
#!/bin/bash -eu

SECRET_NAME="{{ required "secretName is required" .Values.secretName }}"

DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG_DOWNLOADER_CREDENTIALS="$DIR_CLOUDCONFIG_DOWNLOADER/credentials"
PATH_CLOUDCONFIG_DOWNLOADER_SERVER="$DIR_CLOUDCONFIG_DOWNLOADER_CREDENTIALS/server"
PATH_CLOUDCONFIG_DOWNLOADER_CA_CERT="$DIR_CLOUDCONFIG_DOWNLOADER_CREDENTIALS/ca.crt"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_CERT="$DIR_CLOUDCONFIG_DOWNLOADER_CREDENTIALS/client.crt"
PATH_CLOUDCONFIG_DOWNLOADER_CLIENT_KEY="$DIR_CLOUDCONFIG_DOWNLOADER_CREDENTIALS/client.key"

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

CHECKSUM="$(echo "$SECRET" | sed -rn 's/    checksum\/data-script: (.*)/\1/p' | sed -e 's/^"//' -e 's/"$//')"
echo "$CHECKSUM" > "$DIR_CLOUDCONFIG_DOWNLOADER/downloaded_checksum"

SCRIPT="$(echo "$SECRET" | sed -rn 's/  script: (.*)/\1/p')"
echo "$SCRIPT" | base64 -d | bash
{{- end }}
