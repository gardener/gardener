// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 is the v1alpha1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/apis/settings
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta
// +k8s:protobuf-gen=package

//go:generate gen-crd-api-reference-docs -api-dir github.com/gardener/gardener/pkg/apis/settings/v1alpha1 -config ../../../../hack/api-reference/settings-config.json -template-dir ../../../../hack/api-reference/template -out-file ../../../../docs/api-reference/settings.md

// Package v1alpha1 is a version of the API.
// +groupName=settings.gardener.cloud
package v1alpha1
