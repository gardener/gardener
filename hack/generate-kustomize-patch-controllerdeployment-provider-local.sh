# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

source $(dirname "${0}")/lockfile.sh
acquire_lockfile "/tmp/generate-kustomize-patch-controllerdeployment-provider-local.sh.lock"

repository=$(echo $SKAFFOLD_IMAGE | rev | cut -d':' -f 2- | rev)
tag=$(echo $SKAFFOLD_IMAGE | rev | cut -d':' -f 1 | rev)

cat <<EOF > example/provider-local/garden/local/patch-imagevector-overwrite.yaml
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: provider-local
helm:
  values:
    imageVectorOverwrite: |
      images:
      - name: machine-controller-manager-provider-local
        repository: ${repository}
        tag: ${tag}
EOF

