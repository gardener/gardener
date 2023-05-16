#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

{
if ! SECRET="$(wget \
  -qO- \
  --header         "Accept: application/yaml" \
  --header         "Authorization: Bearer $(cat "{{ .pathCredentialsToken }}")" \
  --ca-certificate "{{ .pathCredentialsCACert }}" \
  "$(cat "{{ .pathCredentialsServer }}")/api/v1/namespaces/kube-system/secrets/{{ .secretName }}")"; then

  echo "Could not retrieve the promtail token secret"
  exit 1
fi

echo "$SECRET" | sed -rn "s/  {{ .dataKeyToken }}: (.*)/\1/p" | base64 -d > "{{ .pathAuthToken }}"

exit $?
}
