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

// CollectorSpec defines the desired state of FluentBit
type CollectorSpec struct {
	// Fluent Bit image.
	Image string `json:"image,omitempty"`
	// Fluent Bit Watcher command line arguments.
	Args []string `json:"args,omitempty"`
	// Fluent Bit image pull policy.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Fluent Bit image pull secret
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
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
	// SecurityContext holds pod-level security attributes and common container settings.
	SecurityContext *corev1.PodSecurityContext `json:"securityContext,omitempty"`
	// Host networking is requested for this pod. Use the host's network namespace. If this option is set, the ports that will be used must be specified. Default to false.
	HostNetwork bool `json:"hostNetwork,omitempty"`
	// PVC definition
	PersistentVolumeClaim *corev1.PersistentVolumeClaim `json:"pvc,omitempty"`
	// RBACRules represents additional rbac rules which will be applied to the fluent-bit clusterrole.
	RBACRules []rbacv1.PolicyRule `json:"rbacRules,omitempty"`
	// By default will build the related service according to the globalinputs definition.
	DisableService bool `json:"disableService,omitempty"`
	// The path where buffer chunks are stored.
	BufferPath *string `json:"bufferPath,omitempty"`
	// Ports represents the pod's ports.
	Ports []corev1.ContainerPort `json:"ports,omitempty"`
	// Service represents configurations on the fluent-bit service.
	Service CollectorService `json:"service,omitempty"`
}

// CollectorService defines the service of the FluentBit
type CollectorService struct {
	// Name is the name of the FluentBit service.
	Name string `json:"name,omitempty"`
	// Annotations to add to each Fluentbit service.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to add to each FluentBit service
	Labels map[string]string `json:"labels,omitempty"`
}

// CollectorStatus defines the observed state of FluentBit
type CollectorStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=co
// +genclient

// Collector is the Schema for the fluentbits API
type Collector struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CollectorSpec   `json:"spec,omitempty"`
	Status CollectorStatus `json:"status,omitempty"`
}

// IsBeingDeleted returns true if a deletion timestamp is set
func (co *Collector) IsBeingDeleted() bool {
	return !co.ObjectMeta.DeletionTimestamp.IsZero()
}

// CollectorFinalizerName is the name of the fluentbit finalizer
const CollectorFinalizerName = "collector.fluent.io"

// HasFinalizer returns true if the item has the specified finalizer
func (co *Collector) HasFinalizer(finalizerName string) bool {
	return utils.ContainString(co.ObjectMeta.Finalizers, finalizerName)
}

// AddFinalizer adds the specified finalizer
func (co *Collector) AddFinalizer(finalizerName string) {
	co.ObjectMeta.Finalizers = append(co.ObjectMeta.Finalizers, finalizerName)
}

// RemoveFinalizer removes the specified finalizer
func (co *Collector) RemoveFinalizer(finalizerName string) {
	co.ObjectMeta.Finalizers = utils.RemoveString(co.ObjectMeta.Finalizers, finalizerName)
}

// +kubebuilder:object:root=true

// CollectorList contains a list of Collector
type CollectorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Collector `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Collector{}, &CollectorList{})
}
