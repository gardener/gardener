// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package core

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClass represents a control plane endpoint exposure strategy.
type ExposureClass struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta

	// Handler is the name of the handler which applies the control plane endpoint exposure strategy.
	// This field is immutable.
	Handler string
	// Scheduling holds information how to select applicable Seed's for ExposureClass usage.
	// This field is immutable.
	Scheduling *ExposureClassScheduling
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExposureClassList is a collection of ExposureClass.
type ExposureClassList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	metav1.ListMeta

	// Items is the list of ExposureClasses.
	Items []ExposureClass
}

// ExposureClassScheduling holds information to select applicable Seed's for ExposureClass usage.
type ExposureClassScheduling struct {
	// SeedSelector is an optional label selector for Seed's which are suitable to use the ExposureClass.
	SeedSelector *SeedSelector
	// Tolerations contains the tolerations for taints on Seed clusters.
	Tolerations []Toleration
}
