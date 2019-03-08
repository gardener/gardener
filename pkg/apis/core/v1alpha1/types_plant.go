package v1alpha1

import (
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +k8s:openapi-gen=x-kubernetes-print-columns:custom-columns=NAMESPACE:.metadata.namespace,NAME:.metadata.name,TYPE:.status.cloudInfo.cloud.type,REGION:.status.cloudInfo.cloud.region,VERSION:.status.cloudInfo.kubernetes.version,APISERVER:.status.conditions[?(@.type == 'APIServerAvailable')].status,NODES:.status.conditions[?(@.type == 'EveryNodeReady')].status
type Plant struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object metadata.
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Spec contains the specification of this Plant.
	Spec PlantSpec `json:"spec,omitempty"`
	// Status contains the status of this Plant.
	Status PlantStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlantList is a collection of Plants.
type PlantList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list object metadata.
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	// Items is the list of Plants.
	Items []Plant `json:"items"`
}

const (
	// EveryPlantNodeReady is a constant for a condition type indicating the node health.
	PlantEveryNodeReady ConditionType = "EveryNodeReady"
	// PlantAPIServerAvailable is a constant for a condition type indicating that the Plant cluster API server is available.
	PlantAPIServerAvailable ConditionType = "APIServerAvailable"
)

// PlantSpec is the specification of a Plant.
type PlantSpec struct {
	// SecretRef is a reference to a Secret object containing the Kubeconfig of the external kubernetes
	// clusters to be added to Gardener.
	SecretRef corev1.SecretReference `json:"secretRef"`
	// Monitoring is the configuration for the plant monitoring
	// +optional
	Monitoring Monitoring `json:"monitoring,omitempty"`
	// Logging is the configuration for the plant logging
	// +optional
	Logging Logging `json:"logging,omitempty"`
}

// PlantStatus is the status of a Plant.
type PlantStatus struct {
	// Conditions represents the latest available observations of a Plant's current state.
	// +optional
	Conditions []Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent generation observed for this Plant. It corresponds to the
	// Plant's generation, which is updated on mutation by the API Server.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// ClusterInfo is additional computed information about the newly added cluster (Plant)
	ClusterInfo ClusterInfo `json:"clusterInfo"`
}

// Monitoring is the configuration for the plant monitoring
type Monitoring struct {
	Endpoints []Endpoint `json:"endpoints"`
}

type Logging struct {
	Endpoints []Endpoint `json:"endpoints"`
}

// Endpoint is an endpoint for monitoring and logging
type Endpoint struct {
	// Name is the name of the endpoint
	Name string `json:"name"`
	// Url is the url of the endpoint
	Url string `json:"url"`
}

// ClusterInfo information about the Plant cluster
type ClusterInfo struct {
	Cloud      Cloud              `json:"cloud"`
	Kubernetes v1beta1.Kubernetes `json:"kubernetes"`
}

// Cloud information about the cloud
type Cloud struct {
	Type   string `json:"type"`
	Region string `json:"region"`
}
