// Copyright 2018 The Gardener Authors.
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

package botanist

import (
	"errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	crdExceptions = map[string]bool{
		"global-config.projectcalico.org": true,
		"ip-pool.projectcalico.org":       true,
	}
	workloadExceptions = map[string]bool{
		metav1.NamespaceSystem + "/calico-node":         true,
		metav1.NamespaceSystem + "/kube-proxy":          true,
		metav1.NamespaceSystem + "/kube-dns":            true,
		metav1.NamespaceSystem + "/kube-dns-autoscaler": true,
	}
	namespaceExceptions = map[string]bool{
		metav1.NamespacePublic:  true,
		metav1.NamespaceSystem:  true,
		metav1.NamespaceDefault: true,
	}
	serviceExceptions = map[string]bool{
		metav1.NamespaceDefault + "/kubernetes": true,
	}
)

// CleanupNamespaces deletes all the Namespaces in the Shoot cluster other than those stored in the
// exceptions map <namespaceExceptions>.
func (b *Botanist) CleanupNamespaces() error {
	return b.K8sShootClient.CleanupNamespaces(namespaceExceptions)
}

// CheckNamespaceCleanup will check whether all the Namespaces in the Shoot cluster other than those
// stored in the exceptions map <namespaceExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckNamespaceCleanup() error {
	finished, err := b.K8sShootClient.CheckNamespaceCleanup(namespaceExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all Namespaces to be deleted")
		}
		return err
	}
	return nil
}

// CleanupServices deletes all the Services in the Shoot cluster other than those stored in the
// exceptions map <serviceExceptions>.
func (b *Botanist) CleanupServices() error {
	return b.K8sShootClient.CleanupServices(serviceExceptions)
}

// CheckServiceCleanup will check whether all the Services in the Shoot cluster other than those
// stored in the exceptions map <serviceExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckServiceCleanup() error {
	finished, err := b.K8sShootClient.CheckServiceCleanup(serviceExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all Services to be deleted")
		}
		return err
	}
	return nil
}

// CleanupStatefulSets deletes all the StatefulSets in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupStatefulSets() error {
	return b.K8sShootClient.CleanupStatefulSets(workloadExceptions)
}

// CheckStatefulSetCleanup will check whether all the StatefulSets in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckStatefulSetCleanup() error {
	finished, err := b.K8sShootClient.CheckStatefulSetCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all StatefulSets to be deleted")
		}
		return err
	}
	return nil
}

// CleanupDeployments deletes all the Deployments in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupDeployments() error {
	return b.K8sShootClient.CleanupDeployments(workloadExceptions)
}

// CheckDeploymentCleanup will check whether all the Deployments in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckDeploymentCleanup() error {
	finished, err := b.K8sShootClient.CheckDeploymentCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all Deployments to be deleted")
		}
		return err
	}
	return nil
}

// CleanupReplicationControllers deletes all the ReplicationControllers in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupReplicationControllers() error {
	return b.K8sShootClient.CleanupReplicationControllers(workloadExceptions)
}

// CheckReplicationControllerCleanup will check whether all the ReplicationControllers in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckReplicationControllerCleanup() error {
	finished, err := b.K8sShootClient.CheckReplicationControllerCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all ReplicationControllers to be deleted")
		}
		return err
	}
	return nil
}

// CleanupReplicaSets deletes all the ReplicaSets in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupReplicaSets() error {
	return b.K8sShootClient.CleanupReplicaSets(workloadExceptions)
}

// CheckReplicaSetCleanup will check whether all the ReplicaSets in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckReplicaSetCleanup() error {
	finished, err := b.K8sShootClient.CheckReplicaSetCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all ReplicaSets to be deleted")
		}
		return err
	}
	return nil
}

// CleanupJobs deletes all the Jobs in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupJobs() error {
	return b.K8sShootClient.CleanupJobs(workloadExceptions)
}

// CheckJobCleanup will check whether all the Jobs in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckJobCleanup() error {
	finished, err := b.K8sShootClient.CheckJobCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all Jobs to be deleted")
		}
		return err
	}
	return nil
}

// CleanupPods deletes all the Pods in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupPods() error {
	return b.K8sShootClient.CleanupPods(workloadExceptions)
}

// CheckPodCleanup will check whether all the Pods in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckPodCleanup() error {
	finished, err := b.K8sShootClient.CheckPodCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all Pods to be deleted")
		}
		return err
	}
	return nil
}

// CleanupDaemonSets deletes all the DaemonSets in the Shoot cluster other than those stored in the
// exceptions map <workloadExceptions>.
func (b *Botanist) CleanupDaemonSets() error {
	return b.K8sShootClient.CleanupDaemonSets(workloadExceptions)
}

// CheckDaemonSetCleanup will check whether all the DaemonSets in the Shoot cluster other than those
// stored in the exceptions map <workloadExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckDaemonSetCleanup() error {
	finished, err := b.K8sShootClient.CheckDaemonSetCleanup(workloadExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all DaemonSets to be deleted")
		}
		return err
	}
	return nil
}

// CleanupCRDs deletes all the TPRs/CRDs in the Shoot cluster other than those stored in the
// exceptions map <crdExceptions>.
func (b *Botanist) CleanupCRDs() error {
	return b.K8sShootClient.CleanupCRDs(crdExceptions)
}

// CheckCRDCleanup will check whether all the CRDs in the Shoot cluster other than those
// stored in the exceptions map <crdExceptions> have been deleted. It will return an error
// in case it has not finished yet, and nil if all resources are gone.
func (b *Botanist) CheckCRDCleanup() error {
	finished, err := b.K8sShootClient.CheckCRDCleanup(crdExceptions)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all TPRs/CRDs to be deleted")
		}
		return err
	}
	return nil
}

// CleanupPersistentVolumeClaims deletes all the PersistentVolumeClaims in the Shoot cluster.
func (b *Botanist) CleanupPersistentVolumeClaims() error {
	return b.K8sShootClient.CleanupPersistentVolumeClaims(nil)
}

// CheckPersistentVolumeClaimCleanup will check whether all the PersistentVolumeClaims in the Shoot
// cluster have been deleted. It will return an error in case it has not finished yet, and nil if all
// resources are gone.
func (b *Botanist) CheckPersistentVolumeClaimCleanup() error {
	finished, err := b.K8sShootClient.CheckPersistentVolumeClaimCleanup(nil)
	if err != nil {
		if !finished {
			return errors.New("Waiting for all PersistentVolumeClaims to be deleted")
		}
		return err
	}
	return nil
}
