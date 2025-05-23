// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	// SecretNameWorkloadIdentityKey is a constant for the name of a Kubernetes secret object that contains a
	// PEM-encoded private RSA or ECDSA key used by the Gardener API Server to sign workload identity tokens.
	SecretNameWorkloadIdentityKey = "gardener-apiserver-workload-identity-signing-key"

	// LabelKeyGardenletAutoUpdates is a key for a label on seedmanagement.gardener.cloud/v1alpha1.Gardenlet resources.
	// If set to true, gardener-operator will automatically update the `.spec.deployment.helm.ociRepository.ref` field
	// to its own version after a successful operator.gardener.cloud/v1alpha1.Garden reconciliation.
	LabelKeyGardenletAutoUpdates = "operator.gardener.cloud/auto-update-gardenlet-helm-chart-ref"

	// OperationRotateWorkloadIdentityKeyStart is a constant for an annotation on a Garden indicating that the
	// rotation of the workload identity signing key shall be started.
	OperationRotateWorkloadIdentityKeyStart = "rotate-workload-identity-key-start"
	// OperationRotateWorkloadIdentityKeyComplete is a constant for an annotation on a Shoot indicating that the
	// rotation of the workload identity signing key shall be completed.
	OperationRotateWorkloadIdentityKeyComplete = "rotate-workload-identity-key-complete"
)
