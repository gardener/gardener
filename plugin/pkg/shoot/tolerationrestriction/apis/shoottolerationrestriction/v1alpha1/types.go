// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Configuration provides configuration for the PodTolerationRestriction admission controller.
type Configuration struct {
	metav1.TypeMeta `json:",inline"`
	// Defaults is the Garden cluster-wide default tolerations list.
	Defaults []gardencorev1beta1.Toleration `json:"defaults,omitempty"`
	// Whitelist is the Garden cluster-wide whitelist of tolerations.
	Whitelist []gardencorev1beta1.Toleration `json:"whitelist,omitempty"`
}
