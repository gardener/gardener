{{- define "shoot-cloud-config.cloud-config-downloader" -}}
#!/bin/bash -eu

SECRET_NAME="{{ required "secretName is required" .Values.secretName }}"

DIR_CLOUDCONFIG_DOWNLOADER="/var/lib/cloud-config-downloader"
DIR_CLOUDCONFIG="/var/run/coreos"
DIR_KUBELET="/var/lib/kubelet"

PATH_KUBECONFIG="$DIR_CLOUDCONFIG_DOWNLOADER/kubeconfig"
PATH_CLOUDCONFIG="$DIR_CLOUDCONFIG/cloud_config.yml"
PATH_CLOUDCONFIG_OLD="$DIR_CLOUDCONFIG/cloud_config.old.yml"

mkdir -p "$DIR_CLOUDCONFIG" "$DIR_KUBELET"

function kubectl() {
  /bin/docker run \
    --rm \
    --net host \
    -v "$DIR_CLOUDCONFIG"/:"$DIR_CLOUDCONFIG" \
    -v "$DIR_CLOUDCONFIG_DOWNLOADER"/:"$DIR_CLOUDCONFIG_DOWNLOADER" \
    -e "KUBECONFIG=$PATH_KUBECONFIG" \
    k8s.gcr.io/hyperkube:v1.9.3 \
    kubectl "$@"
}

if ! CLOUD_CONFIG="$(kubectl --namespace=kube-system get secret "$SECRET_NAME" -o jsonpath='{.data.cloudconfig}')"; then
  echo "Could not retrieve the cloud config in secret with name $SECRET_NAME"
  exit 1
fi
if ! BOOTSTRAP_TOKEN="$(kubectl --namespace=kube-system get secret "$SECRET_NAME" -o jsonpath='{.data.bootstrapToken}')"; then
  echo "Could not retrieve the bootstrap in secret with name $SECRET_NAME"
  exit 1
fi

IMAGES="$(kubectl --namespace=kube-system get secret "$SECRET_NAME" -o jsonpath='{.data.images}')";
if [[ "$IMAGES" != "" ]]; then
  echo "Preloading docker images"
  echo "$IMAGES" | base64 -d | docker run -i {{ index .Values.images "ruby" }} ruby -e 'require "json"; puts JSON::load(STDIN.read).values.compact' | xargs -I {} sh -c '[ ! -z $(docker images -q {}) ] || docker pull {}'
fi

base64 -d <<< $CLOUD_CONFIG > "$PATH_CLOUDCONFIG"
if [ ! -f "$PATH_CLOUDCONFIG" ]; then
  echo "No cloud config file found at location $PATH_CLOUDCONFIG"
  exit 1
fi

if [[ ! -f "$DIR_KUBELET/kubeconfig-real" ]]; then
  if ! SERVER="$(kubectl config view -o go-template='{{ "{{" }}index .clusters 0 "cluster" "server"{{ "}}" }}' --raw)"; then
    echo "Could not retrieve the kube-apiserver address."
    exit 1
  fi
  if ! CA_CRT="$(kubectl config view -o go-template='{{ "{{" }}index .clusters 0 "cluster" "certificate-authority-data"{{ "}}" }}' --raw)"; then
    echo "Could not retrieve the CA certificate."
    exit 1
  fi

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
    token: $(echo $BOOTSTRAP_TOKEN | base64 -d)
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
      cat "$PATH_CLOUDCONFIG" | docker run -i {{ index .Values.images "ruby" }} ruby -e "require 'yaml'; puts YAML::load(STDIN.read)['coreos']['units'].map{|v| v['name'] if v['name'] != 'docker.service'}.compact" | xargs systemctl restart
      echo "Successfully restarted all units referenced in the cloud config."
    fi
    cp "$PATH_CLOUDCONFIG" "$PATH_CLOUDCONFIG_OLD"
  fi
fi

rm "$PATH_CLOUDCONFIG"
{{- end -}}
