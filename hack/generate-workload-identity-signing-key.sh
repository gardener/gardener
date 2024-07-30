# SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0

set -e

signing_key_path="${1:-"example/gardener-local/controlplane/workload-identity-signing-key"}"

if [ ! -e "$signing_key_path" ]; then
  openssl genrsa -out "$signing_key_path" 4096
fi
