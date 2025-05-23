// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoottolerationrestriction

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/apis/core"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration provides configuration for the ShootTolerationRestriction admission controller.
type Configuration struct {
	metav1.TypeMeta
	// Defaults is the Garden cluster-wide default tolerations list.
	Defaults []core.Toleration
	// Whitelist is the Garden cluster-wide whitelist of tolerations.
	Whitelist []core.Toleration
}
