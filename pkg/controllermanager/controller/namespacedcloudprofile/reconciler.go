// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"
	"fmt"

	"golang.org/x/exp/maps"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
)

// Reconciler reconciles NamespacedCloudProfiles.
type Reconciler struct {
	Client   client.Client
	Config   config.NamespacedCloudProfileControllerConfiguration
	Recorder record.EventRecorder
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	ctx, cancel := controllerutils.GetMainReconciliationContext(ctx, controllerutils.DefaultReconciliationTimeout)
	defer cancel()

	namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
	if err := r.Client.Get(ctx, request.NamespacedName, namespacedCloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	// The deletionTimestamp labels the NamespacedCloudProfile as intended to get deleted. Before deletion, it has to be ensured that
	// no Shoots and Seed are assigned to the NamespacedCloudProfile anymore. If this is the case then the controller will remove
	// the finalizers from the NamespacedCloudProfile so that it can be garbage collected.
	if namespacedCloudProfile.DeletionTimestamp != nil {
		if !sets.New(namespacedCloudProfile.Finalizers...).Has(gardencorev1beta1.GardenerName) {
			return reconcile.Result{}, nil
		}

		associatedShoots, err := controllerutils.DetermineShootsAssociatedTo(ctx, r.Client, namespacedCloudProfile)
		if err != nil {
			return reconcile.Result{}, err
		}

		if len(associatedShoots) == 0 {
			log.Info("No Shoots are referencing the NamespacedCloudProfile, deletion accepted")

			if controllerutil.ContainsFinalizer(namespacedCloudProfile, gardencorev1beta1.GardenerName) {
				log.Info("Removing finalizer")
				if err := controllerutils.RemoveFinalizers(ctx, r.Client, namespacedCloudProfile, gardencorev1beta1.GardenerName); err != nil {
					return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
				}
			}

			return reconcile.Result{}, nil
		}

		message := fmt.Sprintf("Cannot delete NamespacedCloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
		r.Recorder.Event(namespacedCloudProfile, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, message)
		return reconcile.Result{}, fmt.Errorf("%s", message)
	}

	if !controllerutil.ContainsFinalizer(namespacedCloudProfile, gardencorev1beta1.GardenerName) {
		log.Info("Adding finalizer")
		if err := controllerutils.AddFinalizers(ctx, r.Client, namespacedCloudProfile, gardencorev1beta1.GardenerName); err != nil {
			return reconcile.Result{}, fmt.Errorf("failed to add finalizer: %w", err)
		}
	}

	parentCloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := r.Client.Get(ctx, client.ObjectKey{Name: namespacedCloudProfile.Spec.Parent.Name}, parentCloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Parent object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if err := mergeAndPatchCloudProfile(ctx, r.Client, namespacedCloudProfile, parentCloudProfile); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func mergeAndPatchCloudProfile(ctx context.Context, c client.Client, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, parentCloudProfile *gardencorev1beta1.CloudProfile) error {
	patch := client.StrategicMergeFrom(namespacedCloudProfile.DeepCopy())
	MergeCloudProfiles(namespacedCloudProfile, parentCloudProfile)
	namespacedCloudProfile.Status.ObservedGeneration = namespacedCloudProfile.Generation
	return c.Patch(ctx, namespacedCloudProfile, patch)
}

// MergeCloudProfiles merges the cloud profile spec from a base CloudProfile and a NamespacedCloudProfile
// into the NamespacedCloudProfile.Status.CloudProfileSpec.
func MergeCloudProfiles(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, cloudProfile *gardencorev1beta1.CloudProfile) {
	namespacedCloudProfile.Status.CloudProfileSpec = cloudProfile.Spec

	if namespacedCloudProfile.Spec.Kubernetes != nil {
		namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions, namespacedCloudProfile.Spec.Kubernetes.Versions, expirableVersionKeyFunc, mergeExpirationDates, false)
	}
	namespacedCloudProfile.Status.CloudProfileSpec.MachineImages = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages, namespacedCloudProfile.Spec.MachineImages, machineImageKeyFunc, mergeMachineImages, false)
	namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes, namespacedCloudProfile.Spec.MachineTypes, machineTypeKeyFunc, nil, true)
	namespacedCloudProfile.Status.CloudProfileSpec.VolumeTypes = append(namespacedCloudProfile.Status.CloudProfileSpec.VolumeTypes, namespacedCloudProfile.Spec.VolumeTypes...)
	if namespacedCloudProfile.Spec.CABundle != nil {
		mergedCABundles := fmt.Sprintf("%s%s", ptr.Deref(namespacedCloudProfile.Status.CloudProfileSpec.CABundle, ""), ptr.Deref(namespacedCloudProfile.Spec.CABundle, ""))
		namespacedCloudProfile.Status.CloudProfileSpec.CABundle = &mergedCABundles
	}
}

var (
	expirableVersionKeyFunc    = func(v gardencorev1beta1.ExpirableVersion) string { return v.Version }
	machineImageKeyFunc        = func(i gardencorev1beta1.MachineImage) string { return i.Name }
	machineImageVersionKeyFunc = func(v gardencorev1beta1.MachineImageVersion) string { return v.Version }
	machineTypeKeyFunc         = func(t gardencorev1beta1.MachineType) string { return t.Name }
)

func mergeExpirationDates(base, override gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	base.ExpirationDate = override.ExpirationDate
	return base
}

func mergeMachineImages(base, override gardencorev1beta1.MachineImage) gardencorev1beta1.MachineImage {
	base.Versions = mergeDeep(base.Versions, override.Versions, machineImageVersionKeyFunc, mergeMachineImageVersions, false)
	return base
}

func mergeMachineImageVersions(base, override gardencorev1beta1.MachineImageVersion) gardencorev1beta1.MachineImageVersion {
	base.ExpirableVersion = mergeExpirationDates(base.ExpirableVersion, override.ExpirableVersion)
	return base
}

func mergeDeep[T any](baseArr, override []T, keyFunc func(T) string, mergeFunc func(T, T) T, allowAdditional bool) []T {
	existing := utils.CreateMapFromSlice(baseArr, keyFunc)
	for _, value := range override {
		key := keyFunc(value)
		if _, exists := existing[key]; !exists {
			if allowAdditional {
				existing[key] = value
			}
			continue
		}
		if mergeFunc != nil {
			existing[key] = mergeFunc(existing[key], value)
		} else {
			existing[key] = value
		}
	}
	return maps.Values(existing)
}
