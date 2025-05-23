// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package v1 is the v1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/apis/core
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta
// +k8s:protobuf-gen=package

//go:generate gen-crd-api-reference-docs -api-dir . -config ../../../../hack/api-reference/core-config.json -template-dir ../../../../hack/api-reference/template -out-file ../../../../docs/api-reference/core-v1.md

// Package v1 is a version of the API.
// +groupName=core.gardener.cloud
package v1
