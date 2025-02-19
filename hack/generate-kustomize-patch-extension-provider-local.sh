# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

source $(dirname "${0}")/lockfile.sh
acquire_lockfile "/tmp/generate-kustomize-patch-extension-provider-local.sh.lock"

repository=$(echo $SKAFFOLD_IMAGE | rev | cut -d':' -f 2- | rev)
tag=$(echo $SKAFFOLD_IMAGE | rev | cut -d':' -f 1 | rev)

cat <<EOF > example/provider-local/garden/operator/patch-imagevector-overwrite.yaml
apiVersion: operator.gardener.cloud/v1alpha1
kind: Extension
metadata:
  name: provider-local
spec:
  deployment:
    extension:
      values:
        imageVectorOverwrite: |
          images:
          - name: machine-controller-manager-provider-local
            repository: ${repository}
            tag: ${tag}
EOF
