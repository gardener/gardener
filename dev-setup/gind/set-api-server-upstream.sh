#!/usr/bin/env bash
# SPDX-FileCopyrightText: Contributors to the Gardener project
#
# SPDX-License-Identifier: Apache-2.0

# Points the GinD API server load balancer (gind-apiserver-lb) to the given upstream host or IP.
# Usage: set-api-server-upstream.sh <host-or-ip> [directory]

set -o errexit
set -o nounset
set -o pipefail

upstream="${1:?usage: $0 <host-or-ip> [directory]}"
dir="${2:-/etc/gind-lb}"

mkdir -p "$dir"
tmp="$(mktemp "$dir/.cds.XXXXXX")"
trap 'rm -f "$tmp"' EXIT

cat > "$tmp" <<EOF
version_info: "$(date +%s%N)"
resources:
- "@type": type.googleapis.com/envoy.config.cluster.v3.Cluster
  name: kube_apiserver
  connect_timeout: 3s
  type: STRICT_DNS
  dns_lookup_family: V4_ONLY
  lb_policy: ROUND_ROBIN
  load_assignment:
    cluster_name: kube_apiserver
    endpoints:
    - lb_endpoints:
      - endpoint:
          address:
            socket_address:
              address: ${upstream}
              port_value: 443
EOF

# Write atomically (temp file + mv) so that Envoy never polls a half-written file.
chmod 0644 "$tmp"
mv "$tmp" "$dir/cds.yaml"
trap - EXIT
