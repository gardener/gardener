#!/usr/bin/env bash
# SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
#
# SPDX-License-Identifier: Apache-2.0


set -o errexit
set -o nounset
set -o pipefail

# delete stuff gradually in the right order, otherwise several dependencies will prevent the cleanup from succeeding
kubectl delete seed \
  local \
  local2 \
  local-ha-single-zone \
  local2-ha-single-zone \
  local-ha-multi-zone \
  --ignore-not-found \
  --wait \
  --timeout 5m

kubectl annotate project local garden confirmation.gardener.cloud/deletion=true
skaffold delete -m provider-local,gardenlet

kubectl delete validatingwebhookconfiguration/gardener-admission-controller --ignore-not-found
skaffold delete -m etcd,controlplane

# workaround for https://github.com/gardener/gardener/issues/5164
kubectl delete ns \
  seed-local \
  seed-local2 \
  seed-local-ha-single-zone \
  seed-local2-ha-single-zone \
  seed-local-ha-multi-zone \
  --ignore-not-found

# cleanup namespaces that don't get deleted automatically
kubectl delete ns gardener-system-seed-lease gardener-system-shoot-issuer gardener-system-info --ignore-not-found
