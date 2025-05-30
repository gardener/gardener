// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

//go:generate extension-generator --name=provider-local --provider-type=local --component-category=provider-extension --component-category=operating-system --extension-oci-repository=local-skaffold/gardener-extension-provider-local/charts/extension:v0.0.0 --admission-runtime-oci-repository=local-skaffold/gardener-extension-admission-local/charts/runtime:v0.0.0 --admission-application-oci-repository=local-skaffold/gardener-extension-admission-local/charts/application:v0.0.0 --destination=$PWD/extension.yaml

package extension
