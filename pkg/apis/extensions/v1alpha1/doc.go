// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package

//go:generate gen-crd-api-reference-docs -api-dir . -config ../../../../hack/api-reference/extensions-config.json -template-dir ../../../../hack/api-reference/template -out-file ../../../../docs/api-reference/extensions.md

// Package v1alpha1 is the v1alpha1 version of the API.
// +groupName=extensions.gardener.cloud
package v1alpha1
