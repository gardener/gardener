# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

if [ ! -e example/gardener-local/controlplane/workload-identity-signing-key ]; then
  openssl genrsa -out example/gardener-local/controlplane/workload-identity-signing-key 4096
fi
