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

// FluentdConfigSpec defines the desired state of FluentdConfig
type FluentdConfigSpec struct {
	// Emit mode. If batch, the plugin will emit events per labels matched. Enum: record, batch.
	// will make no effect if EnableFilterKubernetes is set false.
	// +kubebuilder:validation:Enum:=record;batch
	EmitMode string `json:"emit_mode,omitempty"`
	// Sticky tags will match only one record from an event stream. The same tag will be treated the same way.
	// will make no effect if EnableFilterKubernetes is set false.
	StickyTags string `json:"stickyTags,omitempty"`
	// A set of hosts. Ignored if left empty.
	WatchedHosts []string `json:"watchedHosts,omitempty"`
	// A set of container names. Ignored if left empty.
	WatchedContainers []string `json:"watchedConstainers,omitempty"`
	// Use this field to filter the logs, will make no effect if EnableFilterKubernetes is set false.
	WatchedLabels map[string]string `json:"watchedLabels,omitempty"`
	// Select namespaced filter plugins
	FilterSelector *metav1.LabelSelector `json:"filterSelector,omitempty"`
	// Select namespaced output plugins
	OutputSelector *metav1.LabelSelector `json:"outputSelector,omitempty"`
	// Select cluster filter plugins
	ClusterFilterSelector *metav1.LabelSelector `json:"clusterFilterSelector,omitempty"`
	// Select cluster output plugins
	ClusterOutputSelector *metav1.LabelSelector `json:"clusterOutputSelector,omitempty"`
}

// FluentdConfigStatus defines the observed state of FluentdConfig
type FluentdConfigStatus struct {
	// Messages defines the plugin errors which is selected by this fluentdconfig
	Messages string `json:"messages,omitempty"`
	// The state of this fluentd config
	State StatusState `json:"state,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=fdc
//+genclient

// FluentdConfig is the Schema for the fluentdconfigs API
type FluentdConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluentdConfigSpec   `json:"spec,omitempty"`
	Status FluentdConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// FluentdConfigList contains a list of FluentdConfig
type FluentdConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluentdConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluentdConfig{}, &FluentdConfigList{})
}

func (cfg *FluentdConfig) GetNamespace() string {
	return cfg.Namespace
}

func (cfg *FluentdConfig) GetName() string {
	return cfg.Name
}

func (cfg *FluentdConfig) GetCfgId() string {
	return fmt.Sprintf("%s-%s-%s", cfg.Kind, cfg.Namespace, cfg.Name)
}

func (cfg *FluentdConfig) GetWatchedLabels() map[string]string {
	return cfg.Spec.WatchedLabels
}

func (cfg *FluentdConfig) GetWatchedNamespaces() []string {
	return []string{cfg.Namespace}
}

func (cfg *FluentdConfig) GetWatchedContainers() []string {
	return cfg.Spec.WatchedContainers
}

func (cfg *FluentdConfig) GetWatchedHosts() []string {
	return cfg.Spec.WatchedHosts
}
