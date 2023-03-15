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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type StatusState string

const (
	InvalidState StatusState = "invalid"
	ValidState   StatusState = "valid"
)

// ClusterFluentdConfigSpec defines the desired state of ClusterFluentdConfig
type ClusterFluentdConfigSpec struct {
	// Emit mode. If batch, the plugin will emit events per labels matched. Enum: record, batch.
	// will make no effect if EnableFilterKubernetes is set false.
	// +kubebuilder:validation:Enum:=record;batch
	EmitMode string `json:"emit_mode,omitempty"`
	// Sticky tags will match only one record from an event stream. The same tag will be treated the same way.
	// will make no effect if EnableFilterKubernetes is set false.
	StickyTags string `json:"stickyTags,omitempty"`
	// A set of namespaces. The whole namespaces would be watched if left empty.
	WatchedNamespaces []string `json:"watchedNamespaces,omitempty"`
	// A set of hosts. Ignored if left empty.
	WatchedHosts []string `json:"watchedHosts,omitempty"`
	// A set of container names. Ignored if left empty.
	WatchedContainers []string `json:"watchedConstainers,omitempty"`
	// Use this field to filter the logs, will make no effect if EnableFilterKubernetes is set false.
	WatchedLabels map[string]string `json:"watchedLabels,omitempty"`
	// Select cluster filter plugins
	ClusterFilterSelector *metav1.LabelSelector `json:"clusterFilterSelector,omitempty"`
	// Select cluster output plugins
	ClusterOutputSelector *metav1.LabelSelector `json:"clusterOutputSelector,omitempty"`
}

// ClusterFluentdConfigStatus defines the observed state of ClusterFluentdConfig
type ClusterFluentdConfigStatus struct {
	// Messages defines the plugin errors which is selected by this fluentdconfig
	Messages string `json:"messages,omitempty"`
	// The state of this fluentd config
	State StatusState `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=cfdc,scope=Cluster
// +genclient
// +genclient:nonNamespaced

// ClusterFluentdConfig is the Schema for the clusterfluentdconfigs API
type ClusterFluentdConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterFluentdConfigSpec   `json:"spec,omitempty"`
	Status ClusterFluentdConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ClusterFluentdConfigList contains a list of ClusterFluentdConfig
type ClusterFluentdConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterFluentdConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ClusterFluentdConfig{}, &ClusterFluentdConfigList{})
}

func (cfg *ClusterFluentdConfig) GetNamespace() string {
	return cfg.Namespace
}

func (cfg *ClusterFluentdConfig) GetName() string {
	return cfg.Name
}

func (cfg *ClusterFluentdConfig) GetCfgId() string {
	return fmt.Sprintf("%s-%s-%s", cfg.Kind, "cluster", cfg.Name)
}

func (cfg *ClusterFluentdConfig) GetWatchedLabels() map[string]string {
	return cfg.Spec.WatchedLabels
}

func (cfg *ClusterFluentdConfig) GetWatchedNamespaces() []string {
	return cfg.Spec.WatchedNamespaces
}

func (cfg *ClusterFluentdConfig) GetWatchedContainers() []string {
	return cfg.Spec.WatchedContainers
}

func (cfg *ClusterFluentdConfig) GetWatchedHosts() []string {
	return cfg.Spec.WatchedHosts
}
