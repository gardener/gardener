/*
Copyright 2021.

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
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/fluent/fluent-operator/v2/pkg/utils"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FluentBitSpec defines the desired state of FluentBit
type FluentBitSpec struct {
	// DisableService tells if the fluentbit service should be deployed.
	DisableService bool `json:"disableService,omitempty"`
	// Fluent Bit image.
	Image string `json:"image,omitempty"`
	// Fluent Bit Watcher command line arguments.
	Args []string `json:"args,omitempty"`
	// Fluent Bit Watcher command.
	Command []string `json:"command,omitempty"`
	// Fluent Bit image pull policy.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Fluent Bit image pull secret
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// Storage for position db. You will use it if tail input is enabled.
	PositionDB corev1.VolumeSource `json:"positionDB,omitempty"`
	// Container log path
	ContainerLogRealPath string `json:"containerLogRealPath,omitempty"`
	// Compute Resources required by container.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Pod's scheduling constraints.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// Fluentbitconfig object associated with this Fluentbit
	FluentBitConfigName string `json:"fluentBitConfigName,omitempty"`
	// NamespacedFluentBitCfgSelector selects the namespace FluentBitConfig associated with this FluentBit
	NamespacedFluentBitCfgSelector metav1.LabelSelector `json:"namespaceFluentBitCfgSelector,omitempty"`
	// The Secrets are mounted into /fluent-bit/secrets/<secret-name>.
	Secrets []string `json:"secrets,omitempty"`
	// RuntimeClassName represents the container runtime configuration.
	RuntimeClassName string `json:"runtimeClassName,omitempty"`
	// PriorityClassName represents the pod's priority class.
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// List of volumes that can be mounted by containers belonging to the pod.
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// Pod volumes to mount into the container's filesystem.
	VolumesMounts []corev1.VolumeMount `json:"volumesMounts,omitempty"`
	// Annotations to add to each Fluentbit pod.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Annotations to add to the Fluentbit service account
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
	// Labels to add to each FluentBit pod
	Labels map[string]string `json:"labels,omitempty"`
	// SecurityContext holds pod-level security attributes and common container settings.
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// Host networking is requested for this pod. Use the host's network namespace. If this option is set, the ports that will be used must be specified. Default to false.
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// EnvVars represent environment variables that can be passed to fluentbit pods.
	EnvVars []corev1.EnvVar `json:"envVars,omitempty"`
	// LivenessProbe represents the pod's liveness probe.
	LivenessProbe *corev1.Probe `json:"livenessProbe,omitempty"`
	// ReadinessProbe represents the pod's readiness probe.
	ReadinessProbe *corev1.Probe `json:"readinessProbe,omitempty"`
	// InitContainers represents the pod's init containers.
	InitContainers []corev1.Container `json:"initContainers,omitempty"`
	// Ports represents the pod's ports.
	Ports []corev1.ContainerPort `json:"ports,omitempty"`
	// RBACRules represents additional rbac rules which will be applied to the fluent-bit clusterrole.
	RBACRules []rbacv1.PolicyRule `json:"rbacRules,omitempty"`
	// Set DNS policy for the pod. Defaults to "ClusterFirst". Valid values are
	// 'ClusterFirstWithHostNet', 'ClusterFirst', 'Default' or 'None'.
	DNSPolicy corev1.DNSPolicy `json:"dnsPolicy,omitempty"`
	// MetricsPort is the port used by the metrics server. If this option is set, HttpPort from ClusterFluentBitConfig needs to match this value. Default is 2020.
	MetricsPort int32 `json:"metricsPort,omitempty"`
	// Service represents configurations on the fluent-bit service.
	Service FluentBitService `json:"service,omitempty"`
}

// FluentBitService defines the service of the FluentBit
type FluentBitService struct {
	// Name is the name of the FluentBit service.
	Name string `json:"name,omitempty"`
	// Annotations to add to each Fluentbit service.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to add to each FluentBit service
	Labels map[string]string `json:"labels,omitempty"`
}

// FluentBitStatus defines the observed state of FluentBit
type FluentBitStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=fb
// +genclient

// FluentBit is the Schema for the fluentbits API
type FluentBit struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluentBitSpec   `json:"spec,omitempty"`
	Status FluentBitStatus `json:"status,omitempty"`
}

// IsBeingDeleted returns true if a deletion timestamp is set
func (fb *FluentBit) IsBeingDeleted() bool {
	return !fb.ObjectMeta.DeletionTimestamp.IsZero()
}

// FluentBitFinalizerName is the name of the fluentbit finalizer
const FluentBitFinalizerName = "fluentbit.fluent.io"

// HasFinalizer returns true if the item has the specified finalizer
func (fb *FluentBit) HasFinalizer(finalizerName string) bool {
	return utils.ContainString(fb.ObjectMeta.Finalizers, finalizerName)
}

// AddFinalizer adds the specified finalizer
func (fb *FluentBit) AddFinalizer(finalizerName string) {
	fb.ObjectMeta.Finalizers = append(fb.ObjectMeta.Finalizers, finalizerName)
}

// RemoveFinalizer removes the specified finalizer
func (fb *FluentBit) RemoveFinalizer(finalizerName string) {
	fb.ObjectMeta.Finalizers = utils.RemoveString(fb.ObjectMeta.Finalizers, finalizerName)
}

// +kubebuilder:object:root=true

// FluentBitList contains a list of FluentBit
type FluentBitList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []FluentBit `json:"items"`
}

func init() {
	SchemeBuilder.Register(&FluentBit{}, &FluentBitList{})
}
