// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../hack/generate-crds.sh -p 10-crd- --allow-dangerous-types operator.gardener.cloud monitoring.coreos.com_v1 monitoring.coreos.com_v1beta1 monitoring.coreos.com_v1alpha1
//go:generate cp 10-crd-operator.gardener.cloud_gardens.yaml ../../charts/gardener/operator/templates/crd-gardens.yaml
//go:generate cp 10-crd-operator.gardener.cloud_extensions.yaml ../../charts/gardener/operator/templates/crd-extensions.yaml

// Package operator contains example manifests for working on operator.
package operator
