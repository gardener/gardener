// Copyright 2023 SAP SE or an SAP affiliate company
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// EtcdCopyBackupsTask is a task for copying etcd backups from a source to a target store.
type EtcdCopyBackupsTask struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EtcdCopyBackupsTaskSpec   `json:"spec,omitempty"`
	Status EtcdCopyBackupsTaskStatus `json:"status,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true

// EtcdCopyBackupsTaskList contains a list of EtcdCopyBackupsTask objects.
type EtcdCopyBackupsTaskList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EtcdCopyBackupsTask `json:"items"`
}

// EtcdCopyBackupsTaskSpec defines the parameters for the copy backups task.
type EtcdCopyBackupsTaskSpec struct {
	// SourceStore defines the specification of the source object store provider for storing backups.
	SourceStore StoreSpec `json:"sourceStore"`
	// TargetStore defines the specification of the target object store provider for storing backups.
	TargetStore StoreSpec `json:"targetStore"`
	// MaxBackupAge is the maximum age in days that a backup must have in order to be copied.
	// By default all backups will be copied.
	// +optional
	MaxBackupAge *uint32 `json:"maxBackupAge,omitempty"`
	// MaxBackups is the maximum number of backups that will be copied starting with the most recent ones.
	// +optional
	MaxBackups *uint32 `json:"maxBackups,omitempty"`
	// WaitForFinalSnapshot defines the parameters for waiting for a final full snapshot before copying backups.
	// +optional
	WaitForFinalSnapshot *WaitForFinalSnapshotSpec `json:"waitForFinalSnapshot,omitempty"`
}

// WaitForFinalSnapshotSpec defines the parameters for waiting for a final full snapshot before copying backups.
type WaitForFinalSnapshotSpec struct {
	// Enabled specifies whether to wait for a final full snapshot before copying backups.
	Enabled bool `json:"enabled"`
	// Timeout is the timeout for waiting for a final full snapshot. When this timeout expires, the copying of backups
	// will be performed anyway. No timeout or 0 means wait forever.
	// +optional
	Timeout *metav1.Duration `json:"timeout,omitempty"`
}

const (
	// EtcdCopyBackupsTaskSucceeded is a condition type indicating that a EtcdCopyBackupsTask has succeeded.
	EtcdCopyBackupsTaskSucceeded ConditionType = "Succeeded"
	// EtcdCopyBackupsTaskFailed is a condition type indicating that a EtcdCopyBackupsTask has failed.
	EtcdCopyBackupsTaskFailed ConditionType = "Failed"
)

// EtcdCopyBackupsTaskStatus defines the observed state of the copy backups task.
type EtcdCopyBackupsTaskStatus struct {
	// Conditions represents the latest available observations of an object's current state.
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	Conditions []Condition `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`
	// LastError represents the last occurred error.
	// +optional
	LastError *string `json:"lastError,omitempty"`
}

// GetJobName returns the name of the CopyBackups Job.
func (e *EtcdCopyBackupsTask) GetJobName() string {
	return fmt.Sprintf("%s-worker", e.Name)
}
