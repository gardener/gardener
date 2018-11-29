{{- define "shoot-cloud-config.cloud-config-downloader" -}}
#!/bin/bash -eu

SECRET_NAME="{{ required "secretName is required" .Values.secretName }}"
PATH_KUBECONFIG="/var/lib/cloud-config-downloader/kubeconfig"

function kubectl() {
  /bin/docker run \
    --rm \
    --net host \
    -v "$PATH_KUBECONFIG":"$PATH_KUBECONFIG" \
    -e "KUBECONFIG=$PATH_KUBECONFIG" \
    k8s.gcr.io/hyperkube:v1.12.3 \
    kubectl "$@"
}

if ! SCRIPT="$(kubectl --namespace=kube-system get secret "$SECRET_NAME" -o jsonpath='{.data.script}')"; then
  echo "Could not retrieve the cloud config script in secret with name $SECRET_NAME"
  exit 1
fi

echo "$SCRIPT" | base64 -d | bash
{{- end }}
