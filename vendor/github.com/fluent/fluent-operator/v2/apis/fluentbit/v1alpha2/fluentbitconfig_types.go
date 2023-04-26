/*
Copyright 2023.

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

package v1alpha2

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// NamespacedFluentBitCfgSpec defines the desired state of FluentBit
type NamespacedFluentBitCfgSpec struct {
	// Select filter plugins
	FilterSelector metav1.LabelSelector `json:"filterSelector,omitempty"`
	// Select output plugins
	OutputSelector metav1.LabelSelector `json:"outputSelector,omitempty"`
	// Select parser plugins
	ParserSelector metav1.LabelSelector `json:"parserSelector,omitempty"`
	// Select cluster level parser config
	ClusterParserSelector metav1.LabelSelector `json:"clusterParserSelector,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fbc
// +genclient

// FluentBitConfig is the Schema for the API
type FluentBitConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec NamespacedFluentBitCfgSpec `json:"spec,omitempty"`
}

// +kubebuilder:object:root=true

// FluentBitConfigList contains a list of Collector
type FluentBitConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluentBitConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluentBitConfig{}, &FluentBitConfigList{})
}
