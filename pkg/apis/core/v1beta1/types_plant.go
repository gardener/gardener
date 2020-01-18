// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type Plant struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Spec contains the specification of this Plant.
	Spec PlantSpec `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	// Status contains the status of this Plant.
	Status PlantStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlantList is a collection of Plants.
type PlantList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is the list of Plants.
	Items []Plant `json:"items" protobuf:"bytes,2,rep,name=items"`
}

const (
	// PlantEveryNodeReady is a constant for a condition type indicating the node health.
	PlantEveryNodeReady ConditionType = "EveryNodeReady"
	// PlantAPIServerAvailable is a constant for a condition type indicating that the Plant cluster API server is available.
	PlantAPIServerAvailable ConditionType = "APIServerAvailable"
)

// PlantSpec is the specification of a Plant.
type PlantSpec struct {
	// SecretRef is a reference to a Secret object containing the Kubeconfig of the external kubernetes
	// clusters to be added to Gardener.
	SecretRef corev1.LocalObjectReference `json:"secretRef" protobuf:"bytes,1,opt,name=secretRef"`
	// Endpoints is the configuration plant endpoints
	// +patchMergeKey=name
	// +patchStrategy=merge
	// +optional
	Endpoints []Endpoint `json:"endpoints,omitempty" patchStrategy:"merge" patchMergeKey:"name" protobuf:"bytes,2,rep,name=endpoints"`
}

// PlantStatus is the status of a Plant.
type PlantStatus struct {
	// Conditions represents the latest available observations of a Plant's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type" protobuf:"bytes,1,rep,name=conditions"`
	// ObservedGeneration is the most recent generation observed for this Plant. It corresponds to the
	// Plant's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty" protobuf:"varint,2,opt,name=observedGeneration"`
	// ClusterInfo is additional computed information about the newly added cluster (Plant)
	ClusterInfo *ClusterInfo `json:"clusterInfo,omitempty" protobuf:"bytes,3,opt,name=clusterInfo"`
}

// Endpoint is an endpoint for monitoring, logging and other services around the plant.
type Endpoint struct {
	// Name is the name of the endpoint
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// URL is the url of the endpoint
	URL string `json:"url" protobuf:"bytes,2,opt,name=url"`
	// Purpose is the purpose of the endpoint
	Purpose string `json:"purpose" protobuf:"bytes,3,opt,name=purpose"`
}

// ClusterInfo contains information about the Plant cluster
type ClusterInfo struct {
	// Cloud describes the cloud information
	Cloud CloudInfo `json:"cloud" protobuf:"bytes,1,opt,name=cloud"`
	// Kubernetes describes kubernetes meta information (e.g., version)
	Kubernetes KubernetesInfo `json:"kubernetes" protobuf:"bytes,2,opt,name=kubernetes"`
}

// CloudInfo contains information about the cloud
type CloudInfo struct {
	// Type is the cloud type
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Region is the cloud region
	Region string `json:"region" protobuf:"bytes,2,opt,name=region"`
}

// KubernetesInfo contains the version and configuration variables for the Plant cluster.
type KubernetesInfo struct {
	// Version is the semantic Kubernetes version to use for the Plant cluster.
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`
}
