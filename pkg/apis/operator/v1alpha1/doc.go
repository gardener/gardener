// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:openapi-gen=true

//go:generate gen-crd-api-reference-docs -api-dir . -config ../../../../hack/api-reference/operator-config.json -template-dir ../../../../hack/api-reference/template -out-file ../../../../docs/api-reference/operator.md

// Package v1alpha1 contains the configuration of the Gardener Operator.
// +groupName=operator.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
