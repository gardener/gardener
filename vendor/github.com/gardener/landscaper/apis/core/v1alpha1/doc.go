// SPDX-FileCopyrightText: 2019 SAP SE or an SAP affiliate company and Gardener contributors.
//
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 is the v1alpha1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/landscaper/apis/core
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta

//go:generate gen-crd-api-reference-docs -api-dir . -config ../../../hack/api-reference/core-config.json -template-dir ../../../hack/api-reference/template -out-file ../../../docs/api-reference/core.md

// Package v1alpha1 is a version of the API.
// +groupName=landscaper.gardener.cloud
// +kubebuilder:storageversion
package v1alpha1
