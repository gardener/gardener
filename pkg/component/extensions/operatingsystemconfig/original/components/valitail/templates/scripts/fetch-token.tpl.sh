#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

{
{{- if .pathNodeAgentConfig }}
server="$(cat "{{ .pathNodeAgentConfig }}" | sed -rn "s/  server: (.*)/\1/p")"
ca_bundle="$(mktemp)"
trap 'rm -f "$ca_bundle"' EXIT
cat "{{ .pathNodeAgentConfig }}" | sed -rn "s/  caBundle: (.*)/\1/p" | base64 -d > "$ca_bundle"
{{- else }}
server="$(cat "{{ .pathCredentialsServer }}")"
ca_bundle="{{ .pathCredentialsCACert }}"
{{- end }}

if ! SECRET="$(wget \
  -qO- \
  --header         "Accept: application/yaml" \
  --header         "Authorization: Bearer $(cat "{{ .pathCredentialsToken }}")" \
  --ca-certificate "$ca_bundle" \
  "$server/api/v1/namespaces/kube-system/secrets/{{ .secretName }}")"; then

  echo "Could not retrieve the valitail token secret"
  exit 1
fi

echo "$SECRET" | sed -rn "s/  {{ .dataKeyToken }}: (.*)/\1/p" | base64 -d > "{{ .pathAuthToken }}"

exit $?
}
