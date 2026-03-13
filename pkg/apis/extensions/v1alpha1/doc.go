// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/extensions-config.yaml --renderer=markdown --templates-dir=../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../docs/api-reference/extensions.md

// Package v1alpha1 is the v1alpha1 version of the API.
// +groupName=extensions.gardener.cloud
package v1alpha1
