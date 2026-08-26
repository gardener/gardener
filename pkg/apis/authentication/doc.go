// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

// +k8s:deepcopy-gen=package

// Package authentication is the internal version of the API.
// "authentication.gardener.cloud/v1alpha1" API is already used for CRD registration and must not be served by the API server.
// +groupName=authentication.gardener.cloud
package authentication
