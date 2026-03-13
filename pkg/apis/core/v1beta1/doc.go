// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package v1beta1 is the v1beta1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/apis/core
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta
// +k8s:protobuf-gen=package
// +k8s:openapi-model-package=com.github.gardener.gardener.pkg.apis.core.v1beta1

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/core-config.yaml --renderer=markdown --templates-dir=../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../docs/api-reference/core.md

// Package v1beta1 is a version of the API.
// +groupName=core.gardener.cloud
package v1beta1
