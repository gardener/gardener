// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/provider-local/apis/local
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate crd-ref-docs --source-path=. --config=../../../../../hack/api-reference/provider-local-config.yaml --renderer=markdown --templates-dir=../../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../../docs/api-reference/provider-local.md

// Package v1alpha1 contains the local provider API resources.
// +groupName=local.provider.extensions.gardener.cloud
package v1alpha1 // import "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
