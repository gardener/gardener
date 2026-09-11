# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e
set -o pipefail

source "$(dirname "$0")/../../../../hack/lockfile.sh"
acquire_lockfile "/tmp/generate-patch-managedseed.sh.lock"

dir="$(dirname $0)"
type="${1:-unmanaged-infra}"

if [[ "$type" == "unmanaged-infra" ]]; then
  patch_file="$dir/patch-managedseed.yaml"
  cat <<EOF > "$patch_file"
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: root
  namespace: garden
spec:
  gardenlet:
    config:
      seedConfig:
        spec:
          networks:
            nodes: 172.18.0.0/24
            pods: 10.0.0.0/15
            services: 10.2.0.0/16
            shootDefaults:
              pods: 10.3.0.0/16
              services: 10.4.0.0/16
EOF
fi

if [[ "$type" == "managed-infra" ]]; then
  patch_file="$dir/patch-managedseed.yaml"
  cat <<EOF > "$patch_file"
apiVersion: seedmanagement.gardener.cloud/v1alpha1
kind: ManagedSeed
metadata:
  name: root
  namespace: garden
spec:
  gardenlet:
    config:
      seedConfig:
        spec:
          networks:
            nodes: 10.0.0.0/16
            pods: 10.3.0.0/16
            services: 10.4.0.0/16
            shootDefaults:
              pods: 10.5.0.0/16
              services: 10.6.0.0/16
EOF
fi
