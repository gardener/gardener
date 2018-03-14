{{- define "shoot-cloud-config.cloud-config-downloader" -}}
#!/bin/bash -eu

SECRET_NAME="{{ required "secretName is required" .Values.secretName }}"

DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG="/var/run/coreos"
DIR_KUBELET="/var/lib/kubelet"

PATH_YAML2JSON="$DIR_CLOUDCONFIG_DOWNLOADER/yaml2json"
PATH_KUBECONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/kubeconfig"
PATH_CLOUDCONFIG="$DIR_CLOUDCONFIG/cloud_config.yml"
PATH_CLOUDCONFIG_OLD="$DIR_CLOUDCONFIG/cloud_config.old.yml"

mkdir -p "$DIR_CLOUDCONFIG" "$DIR_KUBELET"

if [ ! -f "$PATH_YAML2JSON" ]; then
  curl -L "https://github.com/bronze1man/yaml2json/raw/master/builds/linux_amd64/yaml2json" -o "$PATH_YAML2JSON"
  chmod +x "$PATH_YAML2JSON"
fi

if ! CLOUD_CONFIG_SECRET="$(/bin/docker run \
  --rm \
  --net host \
  -v "$DIR_CLOUDCONFIG"/:"$DIR_CLOUDCONFIG" \
  -v "$DIR_CLOUDCONFIG_DOWNLOADER"/:"$DIR_CLOUDCONFIG_DOWNLOADER" \
  k8s.gcr.io/hyperkube:v1.9.3\
  kubectl --kubeconfig="$PATH_KUBECONFIG" --namespace=kube-system get secret "$SECRET_NAME" -o jsonpath='{.data.cloudconfig}{"\t"}{.data.bootstrapToken}')"; then
  echo "Could not retrieve the cloud config secret with name $SECRET_NAME"
  exit 1
fi

echo $CLOUD_CONFIG_SECRET | awk '{print $1}' | base64 -d > "$PATH_CLOUDCONFIG"
BOOTSTRAP_TOKEN="$(echo $CLOUD_CONFIG_SECRET | awk '{print $2}' | base64 -d)"

if [ ! -f "$PATH_CLOUDCONFIG" ]; then
  echo "No cloud config file found at location $PATH_CLOUDCONFIG"
  exit 1
fi

if [[ ! -f "$DIR_KUBELET/kubeconfig-real" ]]; then
  CLUSTER_INFO="$("$PATH_YAML2JSON" < "$PATH_KUBECONFIG" | jq -r '.clusters[0].cluster')"
  CA_CRT="$(echo $CLUSTER_INFO | jq -r '."certificate-authority-data"')"
  SERVER="$(echo $CLUSTER_INFO | jq -r '.server')"

  cat <<EOF > "$DIR_KUBELET/kubeconfig-bootstrap"
---
apiVersion: v1
kind: Config
current-context: kubelet-bootstrap@default
clusters:
- cluster:
    certificate-authority-data: $CA_CRT
    server: $SERVER
  name: default
contexts:
- context:
    cluster: default
    user: kubelet-bootstrap
  name: kubelet-bootstrap@default
users:
- name: kubelet-bootstrap
  user:
    as-user-extra: {}
    token: $BOOTSTRAP_TOKEN
EOF

else
  rm -f "$DIR_KUBELET/kubeconfig-bootstrap"
fi

if [ ! -f "$PATH_CLOUDCONFIG_OLD" ]; then
  touch "$PATH_CLOUDCONFIG_OLD"
fi

if ! diff "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD" >/dev/null; then
  echo "Seen newer cloud config version"
  if /usr/bin/coreos-cloudinit -from-file="$PATH_CLOUDCONFIG"; then
    echo "Successfully applied new cloud config version"
    if [ "$(stat -c%s "$PATH_CLOUDCONFIG_OLD")" -ne "0" ]; then
      systemctl daemon-reload
      "$PATH_YAML2JSON" < "$PATH_CLOUDCONFIG" | jq -r '.coreos.units[] | select(.name != "docker.service") | .name' | xargs systemctl restart
      echo "Successfully restarted all units referenced in the cloud config."
    fi
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
  fi
fi

rm "$PATH_CLOUDCONFIG"
{{- end -}}
