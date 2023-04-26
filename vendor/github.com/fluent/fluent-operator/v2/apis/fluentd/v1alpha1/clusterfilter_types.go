/*
Copyright 2022.

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

package v1alpha1

import (
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/filter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterFilterSpec defines the desired state of ClusterFilter
type ClusterFilterSpec struct {
	Filters []filter.Filter `json:"filters,omitempty"`
}

// ClusterFilterStatus defines the observed state of ClusterFilter
type ClusterFilterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cfdf,scope=Cluster
// +genclient
// +genclient:nonNamespaced

// ClusterFilter is the Schema for the clusterfilters API
type ClusterFilter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterFilterSpec   `json:"spec,omitempty"`
	Status ClusterFilterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterFilterList contains a list of ClusterFilter
type ClusterFilterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFilter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFilter{}, &ClusterFilterList{})
}
