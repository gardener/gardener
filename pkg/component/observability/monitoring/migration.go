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

package monitoring

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-logr/logr"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// TODO(rfranzke): Remove this file after all Prometheis and AlertManagers have been migrated.

const (
	labelMigrationNamespace = "disk-migration.monitoring.gardener.cloud/namespace"
	labelMigrationPVCName   = "disk-migration.monitoring.gardener.cloud/pvc-name"
)

// DataMigration is a struct for migrating data from existing disks.
type DataMigration struct {
	// Client is the client.
	Client client.Client
	// Namespace is the namespace.
	Namespace string
	// StorageCapacity is the storage capacity of the disk.
	StorageCapacity resource.Quantity
	// FullName is the full name of the component (e.g., prometheus-<name> or alertmanager-<name>).
	FullName string

	// ImageAlpine defines the container image of alpine.
	ImageAlpine string
	// StatefulSetName is the name of the old StatefulSet.
	StatefulSetName string
	// PVCNames is the list of names of the old PersistentVolumeClaims.
	PVCNames []string
}

func (d *DataMigration) kind() string {
	return strings.Split(d.FullName, "-")[0]
}

func (d *DataMigration) name() string {
	return strings.Split(d.FullName, "-")[1]
}

// ExistingPVTakeOverPrerequisites performs the PV take over prerequisites.
func (d *DataMigration) ExistingPVTakeOverPrerequisites(ctx context.Context, log logr.Logger) (bool, []*corev1.PersistentVolume, []*corev1.PersistentVolumeClaim, error) {
	log = log.WithValues("kind", d.kind())

	var (
		pvs      []*corev1.PersistentVolume
		oldPVCs  []*corev1.PersistentVolumeClaim
		takeOver bool
	)

	for _, pvcName := range d.PVCNames {
		mustTakeOver, pv, oldPVC, err := d.checkIfPVMustBeTookOver(ctx, log, pvcName)
		if err != nil {
			return false, nil, nil, err
		}

		pvs = append(pvs, pv)
		oldPVCs = append(oldPVCs, oldPVC)
		if mustTakeOver {
			takeOver = true
		}
	}

	return takeOver, pvs, oldPVCs, nil
}

func (d *DataMigration) checkIfPVMustBeTookOver(ctx context.Context, log logr.Logger, pvcName string) (bool, *corev1.PersistentVolume, *corev1.PersistentVolumeClaim, error) {
	oldPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: pvcName, Namespace: d.Namespace}}
	if err := d.Client.Get(ctx, client.ObjectKeyFromObject(oldPVC), oldPVC); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, nil, nil, err
		}
		log.Info("Old PVC not found when checking whether migrating is needed", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	}

	if oldPVC.Spec.VolumeName == "" {
		// When spec.volumeName is empty then the old PVC wasn't found - let's try finding the existing PV via the label
		// potentially set in previous invocations of this function.
		pv, err := d.findPersistentVolumeByLabel(ctx, log, pvcName)
		if err != nil {
			return false, nil, nil, err
		}
		return pv != nil, pv, oldPVC, nil
	}

	// At this point the old PVC was found, so let's read the PV based on spec.volumeName.
	pv := &corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: oldPVC.Spec.VolumeName}}
	if err := d.Client.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, nil, nil, err
		}
		log.Info("Old PV not found, nothing to migrate", "persistentVolume", client.ObjectKeyFromObject(pv))
		return false, nil, nil, nil
	}

	// PV was found, so let's label it so that we can find it in future invocations of this function (even after the old
	// PVC has already been deleted).
	log.Info("Adding PVC namespace and name to PV labels", "persistentVolume", client.ObjectKeyFromObject(pv), "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := d.patchPV(ctx, pv, func(_ *corev1.PersistentVolume) {
		metav1.SetMetaDataLabel(&pv.ObjectMeta, labelMigrationNamespace, d.Namespace)
		metav1.SetMetaDataLabel(&pv.ObjectMeta, labelMigrationPVCName, pvcName)
	}); err != nil {
		return false, nil, nil, err
	}

	return true, pv, oldPVC, nil
}

func (d *DataMigration) findPersistentVolumeByLabel(ctx context.Context, log logr.Logger, pvcName string) (*corev1.PersistentVolume, error) {
	pvList := &corev1.PersistentVolumeList{}
	if err := d.Client.List(ctx, pvList, client.MatchingLabels{labelMigrationNamespace: d.Namespace, labelMigrationPVCName: pvcName}); err != nil {
		return nil, err
	}

	switch len(pvList.Items) {
	case 0:
		log.Info("Old PV not found via label, nothing to migrate", "namespaceName", d.Namespace, "pvcName", pvcName)
		return nil, nil
	case 1:
		log.Info("Existing PV found via label", "persistentVolume", client.ObjectKeyFromObject(&pvList.Items[0]), "namespaceName", d.Namespace, "pvcName", pvcName)
		return &pvList.Items[0], nil
	default:
		return nil, fmt.Errorf("more than one PV found with labels %s=%s and %s=%s", labelMigrationNamespace, d.Namespace, labelMigrationPVCName, pvcName)
	}
}

// PrepareExistingPVTakeOver prepares the PV take over.
func (d *DataMigration) PrepareExistingPVTakeOver(ctx context.Context, log logr.Logger, pvs []*corev1.PersistentVolume, oldPVCs []*corev1.PersistentVolumeClaim) error {
	log = log.WithValues("kind", d.kind())

	log.Info("Must take over old disk")

	if err := d.deleteOldStatefulSetAndWaitUntilPodsDeleted(ctx, log); err != nil {
		return err
	}

	for _, pv := range pvs {
		log.Info("Setting persistentVolumeReclaimPolicy to 'Retain'", "persistentVolume", client.ObjectKeyFromObject(pv))
		if err := d.patchPV(ctx, pv, func(obj *corev1.PersistentVolume) {
			obj.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimRetain
		}); err != nil {
			return err
		}
	}

	for _, oldPVC := range oldPVCs {
		if err := d.deleteOldPVCAndWaitUntilDeleted(ctx, log, oldPVC); err != nil {
			return err
		}
	}

	for i, pv := range pvs {
		if err := d.removeClaimRefFromPVAndWaitUntilAvailable(ctx, log, pv); err != nil {
			return err
		}

		if err := d.createNewPVCAndWaitUntilPVGotBound(ctx, log, i, pv); err != nil {
			return err
		}
	}

	return nil
}

// FinalizeExistingPVTakeOver finalizes the PV take over.
func (d *DataMigration) FinalizeExistingPVTakeOver(ctx context.Context, log logr.Logger, pvs []*corev1.PersistentVolume) error {
	log = log.WithValues("kind", d.kind())

	if err := d.waitForNewStatefulSetToBeRolledOut(ctx, log); err != nil {
		return err
	}

	for _, pv := range pvs {
		log.Info("Setting persistentVolumeReclaimPolicy to 'Delete' and removing migration label", "persistentVolume", client.ObjectKeyFromObject(pv))
		if err := d.patchPV(ctx, pv, func(obj *corev1.PersistentVolume) {
			obj.Spec.PersistentVolumeReclaimPolicy = corev1.PersistentVolumeReclaimDelete
			delete(pv.Labels, labelMigrationNamespace)
			delete(pv.Labels, labelMigrationPVCName)
		}); err != nil {
			return err
		}
	}

	log.Info("Migration complete")
	return nil
}

func (d *DataMigration) deleteOldStatefulSetAndWaitUntilPodsDeleted(ctx context.Context, log logr.Logger) error {
	oldStatefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: d.StatefulSetName, Namespace: d.Namespace}}
	log.Info("Delete old StatefulSet to unmount disk", "statefulSet", client.ObjectKeyFromObject(oldStatefulSet))
	if err := d.Client.Delete(ctx, oldStatefulSet); client.IgnoreNotFound(err) != nil {
		return err
	}

	log.Info("Wait until all pods belonging to the old StatefulSet are deleted")

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		podList := &corev1.PodList{}

		for i := range d.PVCNames {
			podListTmp := &corev1.PodList{}
			if err := d.Client.List(ctx, podListTmp, client.InNamespace(d.Namespace), client.MatchingLabels{"statefulset.kubernetes.io/pod-name": d.StatefulSetName + "-" + strconv.Itoa(i)}); err != nil {
				return retry.SevereError(err)
			}
			podList.Items = append(podList.Items, podListTmp.Items...)
		}

		if length := len(podList.Items); length > 0 {
			log.Info("There are still existing pods belonging to the old StatefulSet", "count", length)
			return retry.MinorError(fmt.Errorf("pods still exist for StatefulSet %s", d.StatefulSetName))
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	log.Info("All pods belonging to the old StatefulSet are deleted")
	return nil
}

func (d *DataMigration) patchPV(ctx context.Context, pv *corev1.PersistentVolume, mutate func(*corev1.PersistentVolume)) error {
	patch := client.MergeFrom(pv.DeepCopy())
	mutate(pv)
	if err := d.Client.Patch(ctx, pv, patch); err != nil {
		return fmt.Errorf("failed patching PV %s: %w", client.ObjectKeyFromObject(pv), err)
	}

	return nil
}

func (d *DataMigration) deleteOldPVCAndWaitUntilDeleted(ctx context.Context, log logr.Logger, oldPVC *corev1.PersistentVolumeClaim) error {
	log.Info("Delete old PVC", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := d.Client.Delete(ctx, oldPVC); client.IgnoreNotFound(err) != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	log.Info("Wait until old PVC is deleted", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	if err := kubernetesutils.WaitUntilResourceDeleted(timeoutCtx, d.Client, oldPVC, time.Second); err != nil {
		return err
	}

	log.Info("Old PVC is deleted", "persistentVolumeClaim", client.ObjectKeyFromObject(oldPVC))
	return nil
}

func (d *DataMigration) removeClaimRefFromPVAndWaitUntilAvailable(ctx context.Context, log logr.Logger, pv *corev1.PersistentVolume) error {
	log.Info("Removing claimRef from PV if necessary", "persistentVolume", client.ObjectKeyFromObject(pv))

	if pv.Spec.ClaimRef == nil {
		log.Info("PV is already unclaimed, nothing to be done", "persistentVolume", client.ObjectKeyFromObject(pv))
	} else if strings.HasPrefix(pv.Spec.ClaimRef.Name, d.kind()+"-db-"+d.FullName) {
		log.Info("PV is already claimed by new PVC, nothing to be done", "persistentVolume", client.ObjectKeyFromObject(pv))
	} else {
		patch := client.MergeFrom(pv.DeepCopy())
		pv.Spec.ClaimRef = nil
		if err := d.Client.Patch(ctx, pv, patch); err != nil {
			return fmt.Errorf("failed removing claimRef of PV %s: %w", client.ObjectKeyFromObject(pv), err)
		}

		timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		log.Info("Wait until PV is available", "persistentVolume", client.ObjectKeyFromObject(pv))
		if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
			if err := d.Client.Get(ctx, client.ObjectKeyFromObject(pv), pv); err != nil {
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

func (d *DataMigration) createNewPVCAndWaitUntilPVGotBound(ctx context.Context, log logr.Logger, i int, pv *corev1.PersistentVolume) error {
	newPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: d.kind() + "-db-" + d.FullName + "-" + strconv.Itoa(i), Namespace: d.Namespace}}
	log.Info("Creating new PVC to bind the PV", "persistentVolumeClaim", client.ObjectKeyFromObject(newPVC))

	if _, err := controllerutil.CreateOrUpdate(ctx, d.Client, newPVC, func() error {
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/instance", d.name())
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/managed-by", "prometheus-operator")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "app.kubernetes.io/name", d.kind())
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "operator.prometheus.io/name", d.name())
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, "operator.prometheus.io/shard", "0")
		metav1.SetMetaDataLabel(&newPVC.ObjectMeta, d.kind(), d.name())

		newPVC.Spec.AccessModes = []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce}
		newPVC.Spec.Resources = corev1.VolumeResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceStorage: d.StorageCapacity}}

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
		if err := d.Client.Get(ctx, client.ObjectKeyFromObject(newPVC), newPVC); err != nil {
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

func (d *DataMigration) waitForNewStatefulSetToBeRolledOut(ctx context.Context, log logr.Logger) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	statefulSet := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: d.FullName, Namespace: d.Namespace}}
	log.Info("Wait for the new StatefulSet to be healthy and no longer progressing", "statefulSet", client.ObjectKeyFromObject(statefulSet))

	if err := retry.Until(timeoutCtx, time.Second, func(ctx context.Context) (bool, error) {
		if err := d.Client.Get(ctx, client.ObjectKeyFromObject(statefulSet), statefulSet); err != nil {
			return retry.SevereError(err)
		}

		healthyErr := health.CheckStatefulSet(statefulSet)
		if healthyErr != nil {
			log.Info("New StatefulSet is still unhealthy", "reason", healthyErr.Error(), "statefulSet", client.ObjectKeyFromObject(statefulSet))
			return retry.MinorError(healthyErr)
		}

		progressing, reason := health.IsStatefulSetProgressing(statefulSet)
		if progressing {
			log.Info("New StatefulSet is still progressing", "reason", reason, "statefulSet", client.ObjectKeyFromObject(statefulSet))
			return retry.MinorError(fmt.Errorf("StatefulSet %q is still progressing: %s", client.ObjectKeyFromObject(statefulSet), reason))
		}

		return retry.Ok()
	}); err != nil {
		return err
	}

	log.Info("New StatefulSet is healthy now and no longer progressing", "statefulSet", client.ObjectKeyFromObject(statefulSet))
	return nil
}
