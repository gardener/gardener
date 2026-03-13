// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package v1alpha1 is the v1alpha1 version of the API.
// +k8s:deepcopy-gen=package,register
// +k8s:conversion-gen=github.com/gardener/gardener/pkg/apis/authentication
// +k8s:openapi-gen=true
// +k8s:defaulter-gen=TypeMeta
// +k8s:protobuf-gen=package
// +k8s:openapi-model-package=com.github.gardener.gardener.pkg.apis.authentication.v1alpha1

//go:generate crd-ref-docs --source-path=. --config=../../../../hack/api-reference/authentication-config.yaml --renderer=markdown --templates-dir=../../../../hack/api-reference/template --log-level=ERROR --output-path=../../../../docs/api-reference/authentication.md

// Package v1alpha1 is a version of the API.
// "authentication.gardener.cloud/v1alpha1" API is already used for CRD registration and must not be served by the API server.
// +groupName=authentication.gardener.cloud
package v1alpha1
