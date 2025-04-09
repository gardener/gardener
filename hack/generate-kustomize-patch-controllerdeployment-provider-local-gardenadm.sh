# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

source $(dirname "${0}")/lockfile.sh
acquire_lockfile "/tmp/generate-kustomize-patch-provider-local-gardenadm.sh.lock"

dir="$(dirname $0)/../example/provider-local/gardenadm"
patch_file="$dir/patch-controllerdeployment-provider-local.yaml"
ref="$SKAFFOLD_IMAGE"

cat <<EOF > "$patch_file"
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: provider-local
helm:
  ociRepository:
    ref: $ref
EOF
