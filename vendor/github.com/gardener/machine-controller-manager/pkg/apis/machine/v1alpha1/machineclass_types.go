// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// WARNING!
// IF YOU MODIFY ANY OF THE TYPES HERE COPY THEM TO ../types.go
// AND RUN `make generate`

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineClass can be used to templatize and re-use provider configuration
// across multiple Machines / MachineSets / MachineDeployments.
// +k8s:openapi-gen=true
// +resource:path=machineclasses
type MachineClass struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Provider-specific configuration to use during node creation.
	ProviderSpec runtime.RawExtension `json:"providerSpec"`
	// SecretRef stores the necessary secrets such as credetials or userdata.
	SecretRef *corev1.SecretReference `json:"secretRef,omitempty"`
	// Provider is the combination of name and location of cloud-specific drivers.
	Provider string `json:"provider,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true

// MachineClassList contains a list of MachineClasses
type MachineClassList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MachineClass `json:"items"`
}
