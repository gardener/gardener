// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:openapi-gen=true

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/resources-config.yaml --renderer=markdown --templates-dir=../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../docs/api-reference/resources.md

// Package v1alpha1 contains the configuration of the Gardener Resource Manager.
// +groupName=resources.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
