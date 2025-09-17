# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

source "$(dirname "$0")/../../../hack/lockfile.sh"
acquire_lockfile "/tmp/generate-imagevector-overwrite.sh.lock"

dir="$(dirname "$0")/generated"
image_name="$1"
patch_file="$dir/${2:-.imagevector-overwrite.yaml}"
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
