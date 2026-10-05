// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package security

// Field path constants that are specific to the internal API
// representation.
const (
	// CredentialsBindingCredentialsRefName is the field selector path for finding
	// the credentials name of a security.gardener.cloud/v1alpha1 CredentialsBinding.
	CredentialsBindingCredentialsRefName = "spec.credentialsRef.name"
	// CredentialsBindingCredentialsRefNamespace is the field selector path for finding
	// the credentials namespace of a security.gardener.cloud/v1alpha1 CredentialsBinding.
	CredentialsBindingCredentialsRefNamespace = "spec.credentialsRef.namespace"
	// CredentialsBindingCredentialsRefKind is the field selector path for finding
	// the credentials kind of a security.gardener.cloud/v1alpha1 CredentialsBinding.
	CredentialsBindingCredentialsRefKind = "spec.credentialsRef.kind"
)
