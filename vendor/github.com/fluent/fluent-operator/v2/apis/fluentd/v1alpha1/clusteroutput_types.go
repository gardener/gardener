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
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/output"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterOutputSpec defines the desired state of ClusterOutput
type ClusterOutputSpec struct {
	Outputs []output.Output `json:"outputs,omitempty"`
}

// ClusterOutputStatus defines the observed state of ClusterOutput
type ClusterOutputStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cfdo,scope=Cluster
// +genclient
// +genclient:nonNamespaced

// ClusterOutput is the Schema for the clusteroutputs API
type ClusterOutput struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterOutputSpec   `json:"spec,omitempty"`
	Status ClusterOutputStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ClusterOutputList contains a list of ClusterOutput
type ClusterOutputList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterOutput `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterOutput{}, &ClusterOutputList{})
}
