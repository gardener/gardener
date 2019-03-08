package core

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Plant represents an external kubernetes cluster.
type Plant struct {
	metav1.TypeMeta
	// Standard object metadata.
	metav1.ObjectMeta
	// Spec contains the specification of this Plant.
	Spec PlantSpec
	// Status contains the status of this Plant.
	Status PlantStatus
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PlantList is a collection of Plants.
type PlantList struct {
	metav1.TypeMeta
	// Standard list object metadata.
	// +optional
	metav1.ListMeta
	// Items is the list of Plants.
	Items []Plant
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
	SecretRef corev1.SecretReference
	// Monitoring is the configuration for the plant monitoring
	// +optional
	Monitoring Monitoring
	// Logging is the configuration for the plant logging
	// +optional
	Logging Logging
}

// Monitoring is the configuration for the plant monitoring
type Monitoring struct {
	Endpoints []Endpoint
}

// Logging is the configuration for the plant logging
type Logging struct {
	Endpoints []Endpoint
}

// Endpoint is an endpoint for monitoring and logging
type Endpoint struct {
	// Name is the name of the endpoint
	Name string
	// Url is the url of the endpoint
	Url string
}

// PlantStatus is th	e status of a Plant.
type PlantStatus struct {
	// Conditions represents the latest available observations of a Plant's current state.
	Conditions []Condition
	// ObservedGeneration is the most recent generation observed for this Plant. It corresponds to the
	// Plant's generation, which is updated on mutation by the API Server.
	ObservedGeneration *int64
	// ClusterInfo is additional computed information about the newly added cluster (Plant)
	ClusterInfo ClusterInfo
}

// ClusterInfo information about the Plant cluster
type ClusterInfo struct {
	Cloud      Cloud
	Kubernetes garden.Kubernetes
}

// Cloud information about the cloud
type Cloud struct {
	Type   string
	Region string
}
