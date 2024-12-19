# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

# TODO(marc1404): Remove when https://github.com/gardener/gardener/issues/11075 is resolved.
# Start temp debugging
error_handler() {
    echo "An error occurred on line $1."
    echo "$SKAFFOLD_IMAGE"
    echo "$@"
}

trap 'error_handler $LINENO $@' ERR
# End temp debugging

dir="$(dirname $0)/../example/gardener-local/gardenlet/operator"
type="${1:-image}"
ref="$SKAFFOLD_IMAGE"

if [[ "$type" == "helm" ]]; then
  patch_file="$dir/patch-helm-ref.yaml"
  cat <<EOF > "$patch_file"
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: local
  namespace: garden
spec:
  deployment:
    helm:
      ociRepository:
        ref: $ref
EOF
fi

if [[ "$type" == "image" ]]; then
  image_name="$2"
  repository="$(echo "$ref" | rev | cut -d':' -f 2- | rev)"
  tag="$(echo "$ref" | rev | cut -d':' -f 1 | rev)"

  patch_file="$dir/patch-imagevector-overwrite.yaml"
  if [[ ! -f "$patch_file" ]]; then
    cat <<EOF > "$patch_file"
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: Gardenlet
metadata:
  name: local
  namespace: garden
spec:
  deployment:
    imageVectorOverwrite: |
      images: []
EOF
  fi

  images="$(yq e '.spec.deployment.imageVectorOverwrite' "$patch_file" | yq -o json)"

  images="$(echo "$images" | jq -r \
    --arg name "$image_name" \
    --arg repository "$repository" \
    --arg tag "$tag" \
    '.images |= if any(.name == $name) then
        map(if .name == $name then .repository = $repository | .tag = $tag else . end)
      else
        . + [{name: $name, repository: $repository, tag: $tag}] end' |\
   yq eval -P)"

  yq eval ".spec.deployment.imageVectorOverwrite = \"$images\"" -i "$patch_file"
fi
