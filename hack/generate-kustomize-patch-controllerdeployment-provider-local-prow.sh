# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail
set -o nounset

patch_file=example/provider-local/garden/local/patch-controllerdeployment-prow.yaml

cat <<EOF > $patch_file
apiVersion: core.gardener.cloud/v1
kind: ControllerDeployment
metadata:
  name: provider-local
helm:
  values:
EOF

if [ -n "${CI:-}" ]; then
  cat <<EOF >> $patch_file
    webhooks:
      prometheus:
        remoteWriteURLs:
        - http://prometheus-performance.prow.gardener.cloud.local:9090/api/v1/write
        externalLabels:
          prow_job: "${JOB_NAME}"
          prow_build_id: "${BUILD_ID}"
EOF
else
  cat <<EOF >> $patch_file
    {}
EOF
fi
