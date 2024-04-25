// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

const (
	// SecretManagerIdentityOperator is the identity for the secret manager used inside gardener-operator.
	SecretManagerIdentityOperator = "gardener-operator"

	// SecretNameCARuntime is a constant for the name of a secret containing the CA for the garden runtime cluster.
	SecretNameCARuntime = "ca-garden-runtime"
	// SecretNameCAGardener is a constant for the name of a Kubernetes secret object that contains the CA
	// certificate of the Gardener control plane.
	SecretNameCAGardener = "ca-gardener"
)
