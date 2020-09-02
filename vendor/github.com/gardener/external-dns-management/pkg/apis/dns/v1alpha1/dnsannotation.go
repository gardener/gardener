/*
 * Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *      http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 *
 */

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSAnnotationList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []DNSAnnotation `json:"items"`
}

// +kubebuilder:storageversion
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Namespaced,path=dnsannotations,shortName=dnsa,singular=dnsannotation
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name=RefGroup,JSONPath=".spec.resourceRef.apiVersion",type=string
// +kubebuilder:printcolumn:name=RefKind,JSONPath=".spec.resourceRef.kind",type=string
// +kubebuilder:printcolumn:name=RefName,JSONPath=".spec.resourceRef.name",type=string
// +kubebuilder:printcolumn:name=RefNamespace,JSONPath=".spec.resourceRef.namespace",type=string
// +kubebuilder:printcolumn:name=Active,JSONPath=".status.active",type=boolean
// +kubebuilder:printcolumn:name=Age,JSONPath=".metadata.creationTimestamp",type=date
// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type DNSAnnotation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              DNSAnnotationSpec `json:"spec"`
	// +optional
	Status DNSAnnotationStatus `json:"status,omitempty"`
}

type DNSAnnotationSpec struct {
	ResourceRef ResourceReference `json:"resourceRef"`
	Annotations map[string]string `json:"annotations"`
}

type ResourceReference struct {
	// API Version of the annotated object
	APIVersion string `json:"apiVersion"`
	// Kind of the annotated object
	// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
	Kind string `json:"kind"`
	// Name of the annotated object
	// +optional
	Name string `json:"name,omitempty"`
	// Namspace of the annotated object
	// Defaulted by the namespace of the containing resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

type DNSAnnotationStatus struct {
	// Indicates that annotation is observed by a DNS sorce controller
	// +optional
	Active bool `json:"active,omitempty"`
	// In case of a configuration problem this field describes the reason
	// +optional
	Message string `json:"message,omitempty"`
}
