// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package prometheus

import (
	"context"
	"fmt"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// TODO(rfranzke): Remove this file after all Prometheis have been migrated.

const labelMigrationPVCName = "disk-migration.monitoring.gardener.cloud/pvc-name"

// DataMigration is a struct for migrating data from existing disks.
type DataMigration struct {
	// ImageAlpine defines the container image of alpine.
	ImageAlpine string
	// StatefulSetName is the name of the StatefulSet related to the old Prometheus.
	StatefulSetName string
	// PVCName is the name of the PersistentVolumeClaim related to the old Prometheus containing the data to copy over.
	PVCName string
}

func (p *prometheus) existingPVTakeOverPrerequisites(ctx context.Context, log logr.Logger) (bool, *corev1.PersistentVolume, *corev1.PersistentVolumeClaim, error) {
	if p.values.DataMigration.StatefulSetName == "" || p.values.DataMigration.PVCName == "" {
		return false, nil, nil, nil
	}

	oldPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: p.values.DataMigration.PVCName, Namespace: p.namespace}}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(oldPVC), oldPVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, nil, nil, err
		}
		log.Info("Old Prometheus PVC not found when checking whether migrating is needed", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	}

	if oldPVC.Spec.VolumeName == "" {
		// When spec.volumeName is empty then the old PVC wasn't found - let's try finding the existing PV via the label
		// potentially set in previous invocations of this function.
		pv, err := p.findPersistentVolumeByLabel(ctx, log)
		if err != nil {
			return false, nil, nil, err
		}
		return pv != nil, pv, oldPVC, nil
	}

	// At this point the old PVC was found, so let's read the PV based on spec.volumeName.
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: oldPVC.Spec.VolumeName}}
	if err := p.client.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, nil, nil, err
		}
		log.Info("Old Prometheus PV not found, nothing to migrate", "persistentVolume", client.ObjectKeyFromObject(pv))
		return false, nil, nil, nil
	}

	// PV was found, so let's label it so that we can find it in future invocations of this function (even after the old
	// PVC has already been deleted).
	log.Info("Adding PVC name to PV labels", "persistentVolume", client.ObjectKeyFromObject(pv), "label", labelMigrationPVCName, "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := p.patchPV(ctx, pv, func(obj *corev1.PersistentVolume) {
		metav1.SetMetaDataLabel(&pv.ObjectMeta, labelMigrationPVCName, oldPVC.Name)
	}); err != nil {
		return false, nil, nil, err
	}

	return true, pv, oldPVC, nil
}

func (p *prometheus) findPersistentVolumeByLabel(ctx context.Context, log logr.Logger) (*corev1.PersistentVolume, error) {
	pvList := &corev1.PersistentVolumeList{}
	if err := p.client.List(ctx, pvList, client.MatchingLabels{labelMigrationPVCName: p.values.DataMigration.PVCName}); err != nil {
		return nil, err
	}

	switch len(pvList.Items) {
	case 0:
		log.Info("Old Prometheus PV not found via label, nothing to migrate", "label", labelMigrationPVCName)
		return nil, nil
	case 1:
		log.Info("Existing PV found via label", "persistentVolume", client.ObjectKeyFromObject(&pvList.Items[0]), "label", labelMigrationPVCName)
		return &pvList.Items[0], nil
	default:
		return nil, fmt.Errorf("more than one PV found with label %s=%s", labelMigrationPVCName, p.values.DataMigration.PVCName)
	}
}

func (p *prometheus) prepareExistingPVTakeOver(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume, oldPVC *corev1.PersistentVolumeClaim) error {
	log.Info("Must take over old Prometheus disk")

	if err := p.deleteOldStatefulSetAndWaitUntilPodsDeleted(ctx, log); err != nil {
		return err
	}

	log.Info("Setting persistentVolumeReclaimPolicy to 'Retain'", "persistentVolume", client.ObjectKeyFromObject(pv))
	if err := p.patchPV(ctx, pv, func(obj *corev1.PersistentVolume) {
		obj.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
	}); err != nil {
		return err
	}

	if err := p.deleteOldPVCAndWaitUntilDeleted(ctx, log, oldPVC); err != nil {
		return err
	}

	if err := p.removeClaimRefFromPVAndWaitUntilAvailable(ctx, log, pv); err != nil {
		return err
	}

	if err := p.createNewPVCAndWaitUntilPVGotBound(ctx, log, pv); err != nil {
		return err
	}

	return nil
}

func (p *prometheus) finalizeExistingPVTakeOver(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) error {
	if err := p.waitForNewStatefulSetToBeRolledOut(ctx, log); err != nil {
		return err
	}

	log.Info("Setting persistentVolumeReclaimPolicy to 'Delete' and removing migration label", "persistentVolume", client.ObjectKeyFromObject(pv))
	if err := p.patchPV(ctx, pv, func(obj *corev1.PersistentVolume) {
		obj.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
		delete(pv.Labels, labelMigrationPVCName)
	}); err != nil {
		return err
	}

	log.Info("Migration complete")
	return nil
}

func (p *prometheus) deleteOldStatefulSetAndWaitUntilPodsDeleted(ctx context.Context, log logr.Logger) error {
	oldStatefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: p.values.DataMigration.StatefulSetName, Namespace: p.namespace}}
	log.Info("Delete old Prometheus StatefulSet to unmount disk", "statefulSet", client.ObjectKeyFromObject(oldStatefulSet))
	if err := p.client.Delete(ctx, oldStatefulSet); client.IgnoreNotFound(err) != nil {
		return err
	}

	log.Info("Wait until all pods belonging to the old StatefulSet are deleted")

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}
		if err := p.client.List(ctx, podList, client.InNamespace(p.namespace), client.MatchingLabels{"statefulset.kubernetes.io/pod-name": p.values.DataMigration.StatefulSetName + "-0"}); err != nil {
			return retry.SevereError(err)
		}

		if length := len(podList.Items); length > 0 {
			log.Info("There are still existing pods belonging to the old StatefulSet", "count", length)
			return retry.MinorError(fmt.Errorf("pods still exist for StatefulSet %s", p.values.DataMigration.StatefulSetName))
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	log.Info("All pods belonging to the old StatefulSet are deleted")
	return nil
}

func (p *prometheus) patchPV(ctx context.Context, pv *corev1.PersistentVolume, mutate func(*corev1.PersistentVolume)) error {
	patch := client.MergeFrom(pv.DeepCopy())
	mutate(pv)
	if err := p.client.Patch(ctx, pv, patch); err != nil {
		return fmt.Errorf("failed patching PV %s: %w", client.ObjectKeyFromObject(pv), err)
	}

	return nil
}

func (p *prometheus) deleteOldPVCAndWaitUntilDeleted(ctx context.Context, log logr.Logger, oldPVC *corev1.PersistentVolumeClaim) error {
	log.Info("Delete old PVC", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := p.client.Delete(ctx, oldPVC); client.IgnoreNotFound(err) != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	log.Info("Wait until old PVC is deleted", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := kubernetesutils.WaitUntilResourceDeleted(timeoutCtx, p.client, oldPVC, time.Second); err != nil {
		return err
	}

	log.Info("Old PVC is deleted", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	return nil
}

func (p *prometheus) removeClaimRefFromPVAndWaitUntilAvailable(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) error {
	log.Info("Removing claimRef from PV if necessary", "persistentVolume", client.ObjectKeyFromObject(pv))

	if pv.Spec.ClaimRef == nil {
		log.Info("PV is already unclaimed, nothing to be done", "persistentVolume", client.ObjectKeyFromObject(pv))
	} else if pv.Spec.ClaimRef.Name == "prometheus-db-"+p.name()+"-0" {
		log.Info("PV is already claimed by new PVC, nothing to be done", "persistentVolume", client.ObjectKeyFromObject(pv))
	} else {
		patch := client.MergeFrom(pv.DeepCopy())
		pv.Spec.ClaimRef = nil
		if err := p.client.Patch(ctx, pv, patch); err != nil {
			return fmt.Errorf("failed removing claimRef of PV %s: %w", client.ObjectKeyFromObject(pv), err)
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		log.Info("Wait until PV is available", "persistentVolume", client.ObjectKeyFromObject(pv))
		if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
			if err := p.client.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
				return retry.SevereError(err)
			}

			if pv.Status.Phase != corev1.VolumeAvailable {
				log.Info("PV is not yet in 'Available' phase", "phase", pv.Status.Phase, "persistentVolume", client.ObjectKeyFromObject(pv))
				return retry.MinorError(fmt.Errorf("phase is %s instead of %s", pv.Status.Phase, corev1.VolumeAvailable))
			}

			return retry.Ok()
		}); err != nil {
			return err
		}

		log.Info("PV is available to get bound by new PVC", "persistentVolume", client.ObjectKeyFromObject(pv))
	}

	return nil
}

func (p *prometheus) createNewPVCAndWaitUntilPVGotBound(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) error {
	newPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "prometheus-db-" + p.name() + "-0", Namespace: p.namespace}}
	log.Info("Creating new PVC to bind the PV", "persistentVolumeClaim", client.ObjectKeyFromObject(newPVC))

	if _, err := controllerutil.CreateOrUpdate(ctx, p.client, newPVC, func() error {
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/instance", "cache")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/managed-by", "prometheus-operator")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/name", "prometheus")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "operator.prometheus.io/name", "cache")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "operator.prometheus.io/shard", "0")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "prometheus", "cache")

		newPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		newPVC.Spec.Resources = corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: p.values.StorageCapacity}}

		volumeMode := corev1.PersistentVolumeFilesystem
		newPVC.Spec.VolumeMode = &volumeMode

		newPVC.Spec.VolumeName = pv.Name
		return nil
	}); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	log.Info("Wait until PV is bound by new PVC", "persistentVolumeClaim", client.ObjectKeyFromObject(newPVC))
	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		if err := p.client.Get(ctx, client.ObjectKeyFromObject(newPVC), newPVC); err != nil {
			return retry.SevereError(err)
		}

		if newPVC.Status.Phase != corev1.ClaimBound {
			log.Info("New PVC is not yet in 'Bound' phase", "phase", newPVC.Status.Phase, "persistentVolumeClaim", client.ObjectKeyFromObject(newPVC))
			return retry.MinorError(fmt.Errorf("phase is %s instead of %s", newPVC.Status.Phase, corev1.ClaimBound))
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	log.Info("PV is bound by new PVC", "persistentVolumeClaim", client.ObjectKeyFromObject(newPVC))
	return nil
}

func (p *prometheus) waitForNewStatefulSetToBeRolledOut(ctx context.Context, log logr.Logger) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: p.name(), Namespace: p.namespace}}
	log.Info("Wait for the new Prometheus StatefulSet to be healthy and no longer progressing", "statefulSet", client.ObjectKeyFromObject(statefulSet))

	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		if err := p.client.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
			return retry.SevereError(err)
		}

		healthyErr := health.CheckStatefulSet(statefulSet)
		if healthyErr != nil {
			log.Info("New Prometheus StatefulSet is still unhealthy", "reason", healthyErr.Error(), "statefulSet", client.ObjectKeyFromObject(statefulSet))
			return retry.MinorError(healthyErr)
		}

		progressing, reason := health.IsStatefulSetProgressing(statefulSet)
		if progressing {
			log.Info("New Prometheus StatefulSet is still progressing", "reason", reason, "statefulSet", client.ObjectKeyFromObject(statefulSet))
			return retry.MinorError(fmt.Errorf("StatefulSet %q is still progressing: %s", client.ObjectKeyFromObject(statefulSet), reason))
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	log.Info("New Prometheus StatefulSet is healthy now and no longer progressing", "statefulSet", client.ObjectKeyFromObject(statefulSet))
	return nil
}
