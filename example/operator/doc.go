// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate ../../hack/generate-crds.sh -p 10-crd- --allow-dangerous-types operator.gardener.cloud
//go:generate cp 10-crd-operator.gardener.cloud_gardens.yaml ../../charts/gardener/operator/templates/crd-gardens.yaml
//go:generate cp 10-crd-operator.gardener.cloud_extensions.yaml ../../charts/gardener/operator/templates/crd-extensions.yaml
//go:generate ../../hack/generate-extension.sh --name=provider-local --provider-type=local --component-name=provider-extension --extension-oci-repository=local-skaffold/gardener-extension-provider-local/charts/extension:v0.0.0 --admission-runtime-oci-repository=local-skaffold/gardener-extension-admission-local/charts/runtime:v0.0.0 --admission-application-oci-repository=local-skaffold/gardener-extension-admission-local/charts/application:v0.0.0 --destination=./15-extension.yaml

// Package operator contains example manifests for working on operator.
package operator
