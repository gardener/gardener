# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

source $(dirname "${0}")/lockfile.sh
acquire_lockfile "/tmp/generate-gardenadm-imagevector-overwrite.sh.lock"

dir="$(dirname $0)/../dev-setup/gardenadm"
patch_file="$dir/.imagevector-overwrite.yaml"
image_name="$1"
ref="$SKAFFOLD_IMAGE"

if [[ ! -f "$patch_file" ]]; then
  cat <<EOF > "$patch_file"
images: []
EOF
fi

images="$(yq e '.' "$patch_file" | yq -o json)"

images="$(echo "$images" | jq -r \
  --arg name "$image_name" \
  --arg ref "$ref" \
  '.images |= if any(.name == $name) then
      map(if .name == $name then .ref = $ref else . end)
    else
      . + [{name: $name, ref: $ref}] end' |\
 yq eval -P)"

yq eval ". = \"$images\"" -i "$patch_file"
