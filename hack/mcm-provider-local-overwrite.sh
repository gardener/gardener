# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

repository=$(echo $SKAFFOLD_IMAGE | cut -d':' -f 1,2)
# Make sure to extract everything after the last ':'.
# E.g. 
# registry.io/repo:tag -> tag
# registry.io:5001/repo:tag -> tag
tag=$(echo $SKAFFOLD_IMAGE | rev | cut -d':' -f 1 | rev)

cat <<EOF > example/provider-local/garden/base/patch-imagevector-overwrite.yaml
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

