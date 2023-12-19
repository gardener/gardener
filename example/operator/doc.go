// SPDX-FileCopyrightText: 2022 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../hack/generate-crds.sh -p 10-crd- --allow-dangerous-types operator.gardener.cloud
//go:generate cp 10-crd-operator.gardener.cloud_gardens.yaml ../../charts/gardener/operator/templates/customresouredefintion.yaml

// Package operator contains example manifests for working on operator.
package operator
