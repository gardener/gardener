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
	"github.com/fluent/fluent-operator/v2/apis/fluentd/v1alpha1/plugins/input"
	"github.com/fluent/fluent-operator/v2/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	ActiveState   StatusState = "active"
	InactiveState StatusState = "inactive"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// FluentdSpec defines the desired state of Fluentd
type FluentdSpec struct {
	// Fluentd global inputs.
	GlobalInputs []input.Input `json:"globalInputs,omitempty"`
	// By default will build the related service according to the globalinputs definition.
	DisableService bool `json:"disableService,omitempty"`
	// Numbers of the Fluentd instance
	Replicas *int32 `json:"replicas,omitempty"`
	// Numbers of the workers in Fluentd instance
	Workers *int32 `json:"workers,omitempty"`
	// Fluentd image.
	Image string `json:"image,omitempty"`
	// Fluentd Watcher command line arguments.
	Args []string `json:"args,omitempty"`
	// FluentdCfgSelector defines the selectors to select the fluentd config CRs.
	FluentdCfgSelector metav1.LabelSelector `json:"fluentdCfgSelector,omitempty"`
	// Buffer definition
	BufferVolume *BufferVolume `json:"buffer,omitempty"`
	// Fluentd image pull policy.
	ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`
	// Fluentd image pull secret
	ImagePullSecrets []corev1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
	// Compute Resources required by container.
	Resources corev1.ResourceRequirements `json:"resources,omitempty"`
	// NodeSelector
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// Annotations to add to the Fluentd service account
	ServiceAccountAnnotations map[string]string `json:"serviceAccountAnnotations,omitempty"`
	// Pod's scheduling constraints.
	Affinity *corev1.Affinity `json:"affinity,omitempty"`
	// Tolerations
	Tolerations []corev1.Toleration `json:"tolerations,omitempty"`
	// RuntimeClassName represents the container runtime configuration.
	RuntimeClassName string `json:"runtimeClassName,omitempty"`
	// PriorityClassName represents the pod's priority class.
	PriorityClassName string `json:"priorityClassName,omitempty"`
	// RBACRules represents additional rbac rules which will be applied to the fluentd clusterrole.
	RBACRules []rbacv1.PolicyRule `json:"rbacRules,omitempty"`
	// List of volumes that can be mounted by containers belonging to the pod.
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// Pod volumes to mount into the container's filesystem. Cannot be updated.
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
	// volumeClaimTemplates is a list of claims that pods are allowed to reference.
	// The StatefulSet controller is responsible for mapping network identities to
	// claims in a way that maintains the identity of a pod. Every claim in
	// this list must have at least one matching (by name) volumeMount in one
	// container in the template.
	VolumeClaimTemplates []corev1.PersistentVolumeClaim `json:"volumeClaimTemplates,omitempty"`
	// Service represents configurations on the fluentd service.
	Service FluentDService `json:"service,omitempty"`
}

// FluentDService the service of the FluentD
type FluentDService struct {
	// Name is the name of the FluentD service.
	Name string `json:"name,omitempty"`
	// Annotations to add to each FluentD service.
	Annotations map[string]string `json:"annotations,omitempty"`
	// Labels to add to each FluentD service
	Labels map[string]string `json:"labels,omitempty"`
}

type BufferVolume struct {
	// Enabled buffer pvc by default.
	DisableBufferVolume bool `json:"disableBufferVolume,omitempty"`

	// Volume definition.
	HostPath *corev1.HostPathVolumeSource `json:"hostPath,omitempty"`
	EmptyDir *corev1.EmptyDirVolumeSource `json:"emptyDir,omitempty"`

	// PVC definition
	PersistentVolumeClaim *corev1.PersistentVolumeClaim `json:"pvc,omitempty"`
}

// FluentdStatus defines the observed state of Fluentd
type FluentdStatus struct {
	// Messages defines the plugin errors which is selected by this fluentdconfig
	Messages string `json:"messages,omitempty"`
	// The state of this fluentd
	State StatusState `json:"state,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=fd
// +genclient

// Fluentd is the Schema for the fluentds API
type Fluentd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FluentdSpec   `json:"spec,omitempty"`
	Status FluentdStatus `json:"status,omitempty"`
}

// IsBeingDeleted returns true if a deletion timestamp is set
func (fd *Fluentd) IsBeingDeleted() bool {
	return !fd.ObjectMeta.DeletionTimestamp.IsZero()
}

// FluentBitFinalizerName is the name of the fluentbit finalizer
const FluentdFinalizerName = "fluentd.fluent.io"

// HasFinalizer returns true if the item has the specified finalizer
func (fd *Fluentd) HasFinalizer(finalizerName string) bool {
	return utils.ContainString(fd.ObjectMeta.Finalizers, finalizerName)
}

// AddFinalizer adds the specified finalizer
func (fd *Fluentd) AddFinalizer(finalizerName string) {
	fd.ObjectMeta.Finalizers = append(fd.ObjectMeta.Finalizers, finalizerName)
}

// RemoveFinalizer removes the specified finalizer
func (fd *Fluentd) RemoveFinalizer(finalizerName string) {
	fd.ObjectMeta.Finalizers = utils.RemoveString(fd.ObjectMeta.Finalizers, finalizerName)
}

// +kubebuilder:object:root=true

// FluentdList contains a list of Fluentd
type FluentdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Fluentd `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Fluentd{}, &FluentdList{})
}
