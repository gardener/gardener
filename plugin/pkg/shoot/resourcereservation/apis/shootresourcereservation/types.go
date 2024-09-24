// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shootresourcereservation

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration provides configuration for the ShootResourceReservation admission controller.
type Configuration struct {
	metav1.TypeMeta
	// UseGKEFormula enables the calculation of resource reservations based on
	// the CPU and memory resources available for a machine type.
	UseGKEFormula bool `json:"useGKEFormula"`
	// LabelSelector optionally defines a label selector for which the GKE formula should be applied
	LabelSelector string `json:"labelSelector,omitempty"`
}
