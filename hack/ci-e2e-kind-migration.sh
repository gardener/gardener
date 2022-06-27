#!/usr/bin/env bash

set -o nounset
set -o pipefail
set -o errexit

if [[ "$OSTYPE" != "darwin"* ]]; then
# https://github.com/kubernetes/test-infra/issues/23741
iptables -t mangle -A POSTROUTING -p tcp --tcp-flags SYN,RST SYN -j TCPMSS --clamp-mss-to-pmtu
fi

# test setup
make kind-up
export KUBECONFIG=$PWD/gardener-local/kind/kubeconfig
make gardener-up

# setup second kind cluster
make kind2-up
make gardenlet-kind2-up

# run test
make test-e2e-local-migration
