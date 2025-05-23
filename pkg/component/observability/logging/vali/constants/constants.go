// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package constants

const (
	// ValiPort is the port exposed by the vali.
	ValiPort = 3100
	// ServiceName is the name of the logging service.
	ServiceName = "logging"
	// ManagedResourceNameRuntime is the name of the managed resource which deploys Vali statefulSet.
	ManagedResourceNameRuntime = "vali"
	// ValitailTokenSecretName is the name of a secret in the kube-system namespace in the target cluster containing
	// valitail's token for communication with the kube-apiserver.
	ValitailTokenSecretName = "gardener-valitail"
)
