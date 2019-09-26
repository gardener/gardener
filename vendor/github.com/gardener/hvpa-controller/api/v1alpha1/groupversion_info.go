/*
Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package v1alpha1 contains API Schema definitions for the autoscaling v1alpha1 API group
// +kubebuilder:object:generate=true
// +groupName=autoscaling.k8s.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	vpa_api "k8s.io/autoscaler/vertical-pod-autoscaler/pkg/apis/autoscaling.k8s.io/v1beta2"
)

var (
	// GroupName is the group name use in this package
	GroupName = "autoscaling.k8s.io"

	// SchemeGroupVersionVpa is group version used to register these objects
	SchemeGroupVersionVpa = schema.GroupVersion{Group: GroupName, Version: "v1beta2"}

	// SchemeGroupVersionHvpa is group version used to register these objects
	SchemeGroupVersionHvpa = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	localSchemeBuilder = &SchemeBuilder

	// AddToScheme adds the types in this group-version to the given scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)

func init() {
	// We only register manually written functions here. The registration of the
	// generated functions takes place in the generated files. The separation
	// makes the code compile even when the generated files are missing.
	localSchemeBuilder.Register(addKnownTypes)
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersionHvpa,
		&Hvpa{},
		&HvpaList{},
	)
	scheme.AddKnownTypes(SchemeGroupVersionVpa,
		&vpa_api.VerticalPodAutoscaler{},
		&vpa_api.VerticalPodAutoscalerList{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersionVpa)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersionHvpa)
	return nil
}
