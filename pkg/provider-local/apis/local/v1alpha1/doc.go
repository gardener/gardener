// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/provider-local/apis/local
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate gen-crd-api-reference-docs -api-dir . -config ../../../../../hack/api-reference/provider-local-config.json -template-dir ../../../../../hack/api-reference/template -out-file ../../../../../docs/api-reference/provider-local.md

// Package v1alpha1 contains the local provider API resources.
// +groupName=local.provider.extensions.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
