// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package namespacedcloudprofile

import (
	"context"
	"fmt"
	"slices"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/clock"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener/pkg/api"
	gardencorehelper "github.com/gardener/gardener/pkg/api/core/helper"
	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler reconciles NamespacedCloudProfiles.
type Reconciler struct {
	Client   client.Client
	Clock    clock.Clock
	Config   controllermanagerconfigv1alpha1.NamespacedCloudProfileControllerConfiguration
	Recorder events.EventRecorder
}

// Reconcile performs the main reconciliation logic.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	log := logf.FromContext(ctx)

	namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
	if err := r.Client.Get(ctx, request.NamespacedName, namespacedCloudProfile); err != nil {
		if apierrors.IsNotFound(err) {
			log.V(1).Info("Object is gone, stop reconciling")
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("error retrieving object from store: %w", err)
	}

	if namespacedCloudProfile.DeletionTimestamp != nil {
		return r.delete(ctx, log, namespacedCloudProfile)
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

	return reconcile.Result{
		RequeueAfter: v1beta1helper.DurationUntilNextVersionTransition(&namespacedCloudProfile.Status.CloudProfileSpec, r.Clock.Now()),
	}, nil
}

// delete deletes the NamespacedCloudProfile as intended by its deletionTimestamp. Before deletion, it has to be ensured that
// no Shoots are assigned to the CloudProfile anymore.
// If this is the case, the controller will remove the finalizers from the NamespacedCloudProfile so that it can be garbage collected.
func (r *Reconciler) delete(ctx context.Context, log logr.Logger, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) (reconcile.Result, error) {
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
				r.Recorder.Eventf(namespacedCloudProfile, nil, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, gardencorev1beta1.EventActionDelete, "failed to remove finalizer: %v", err)
				return reconcile.Result{}, fmt.Errorf("failed to remove finalizer: %w", err)
			}
		}

		return reconcile.Result{}, nil
	}

	message := fmt.Sprintf("Cannot delete NamespacedCloudProfile, because the following Shoots are still referencing it: %+v", associatedShoots)
	r.Recorder.Eventf(namespacedCloudProfile, nil, corev1.EventTypeNormal, v1beta1constants.EventResourceReferenced, gardencorev1beta1.EventActionReconcile, message)
	return reconcile.Result{}, fmt.Errorf("%s", message)
}

func mergeAndPatchCloudProfile(ctx context.Context, c client.Client, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, parentCloudProfile *gardencorev1beta1.CloudProfile) error {
	old := namespacedCloudProfile.DeepCopy()

	MergeCloudProfiles(namespacedCloudProfile, parentCloudProfile)
	namespacedCloudProfile.Status.ObservedGeneration = namespacedCloudProfile.Generation

	if equality.Semantic.DeepEqual(old.Status, namespacedCloudProfile.Status) {
		return nil
	}
	return c.Status().Patch(ctx, namespacedCloudProfile, client.MergeFrom(old))
}

// MergeCloudProfiles merges the cloud profile spec from a base CloudProfile and a NamespacedCloudProfile
// into the NamespacedCloudProfile.Status.CloudProfileSpec.
func MergeCloudProfiles(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, cloudProfile *gardencorev1beta1.CloudProfile) {
	namespacedCloudProfile.Status.CloudProfileSpec = cloudProfile.Spec

	if namespacedCloudProfile.Spec.Kubernetes != nil {
		namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions, namespacedCloudProfile.Spec.Kubernetes.Versions, expirableVersionKeyFunc, ApplyExpirableVersionOverrides, false)
	}

	// TODO(Roncossek): Remove TransformSpecToParentFormat once all CloudProfiles have been migrated to use CapabilityFlavors and the Architecture fields are effectively forbidden or have been removed.
	uniformNamespacedCloudProfileSpec := gardenerutils.TransformSpecToParentFormat(namespacedCloudProfile.Spec, cloudProfile.Spec.MachineCapabilities)
	namespacedCloudProfile.Status.CloudProfileSpec.MachineImages = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.MachineImages, uniformNamespacedCloudProfileSpec.MachineImages, machineImageKeyFunc, mergeMachineImages, true)
	namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.MachineTypes, uniformNamespacedCloudProfileSpec.MachineTypes, machineTypeKeyFunc, nil, true)
	namespacedCloudProfile.Status.CloudProfileSpec.VolumeTypes = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.VolumeTypes, namespacedCloudProfile.Spec.VolumeTypes, volumeTypeKeyFunc, nil, true)
	if namespacedCloudProfile.Spec.CABundle != nil {
		mergedCABundles := fmt.Sprintf("%s%s", ptr.Deref(namespacedCloudProfile.Status.CloudProfileSpec.CABundle, ""), ptr.Deref(namespacedCloudProfile.Spec.CABundle, ""))
		namespacedCloudProfile.Status.CloudProfileSpec.CABundle = &mergedCABundles
	}
	if namespacedCloudProfile.Spec.Limits != nil {
		if namespacedCloudProfile.Status.CloudProfileSpec.Limits == nil {
			namespacedCloudProfile.Status.CloudProfileSpec.Limits = &gardencorev1beta1.Limits{}
		}
		if ptr.Deref(namespacedCloudProfile.Spec.Limits.MaxNodesTotal, 0) > 0 {
			namespacedCloudProfile.Status.CloudProfileSpec.Limits.MaxNodesTotal = namespacedCloudProfile.Spec.Limits.MaxNodesTotal
		}
	}

	syncArchitectureCapabilities(namespacedCloudProfile)
}

func syncArchitectureCapabilities(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) {
	var coreCloudProfileSpec gardencore.CloudProfileSpec
	_ = api.Scheme.Convert(&namespacedCloudProfile.Status.CloudProfileSpec, &coreCloudProfileSpec, nil)
	defaultMachineTypeArchitectures(coreCloudProfileSpec, coreCloudProfileSpec.MachineCapabilities)
	defaultMachineImageArchitectures(coreCloudProfileSpec, coreCloudProfileSpec.MachineCapabilities)
	_ = api.Scheme.Convert(&coreCloudProfileSpec, &namespacedCloudProfile.Status.CloudProfileSpec, nil)
}

// defaultMachineTypeArchitectures defaults the architectures of the machine types for NamespacedCloudProfiles.
// The sync can only happen after having had a look at the parent CloudProfile and whether it uses capabilities.
func defaultMachineTypeArchitectures(cloudProfile gardencore.CloudProfileSpec, capabilitiesDefinitions []gardencore.CapabilityDefinition) {
	for i, machineType := range cloudProfile.MachineTypes {
		effectiveArchitecture := machineType.GetArchitecture(capabilitiesDefinitions)
		if effectiveArchitecture == "" {
			cloudProfile.MachineTypes[i].Architecture = new(v1beta1constants.ArchitectureAMD64)
		} else if cloudProfile.MachineTypes[i].Architecture == nil {
			cloudProfile.MachineTypes[i].Architecture = new(effectiveArchitecture)
		}
	}
}

// defaultMachineImageArchitectures defaults the architectures of the machine images for NamespacedCloudProfiles.
// The sync can only happen after having had a look at the parent CloudProfile and whether it uses capabilities.
func defaultMachineImageArchitectures(cloudProfile gardencore.CloudProfileSpec, capabilitiesDefinitions []gardencore.CapabilityDefinition) {
	for i, machineImage := range cloudProfile.MachineImages {
		for j, version := range machineImage.Versions {
			if len(version.Architectures) > 0 {
				continue
			}
			capabilityFlavors := gardencorehelper.GetImageFlavorsWithAppliedDefaults(version.CapabilityFlavors, capabilitiesDefinitions)
			capabilityArchitectures := gardencorehelper.ExtractArchitecturesFromImageFlavors(capabilityFlavors)
			if len(capabilityArchitectures) == 0 {
				cloudProfile.MachineImages[i].Versions[j].Architectures = []string{v1beta1constants.ArchitectureAMD64}
			} else if len(capabilityArchitectures) > 0 {
				cloudProfile.MachineImages[i].Versions[j].Architectures = capabilityArchitectures
			}
		}
	}
}

var (
	expirableVersionKeyFunc    = func(v gardencorev1beta1.ExpirableVersion) string { return v.Version }
	machineImageKeyFunc        = func(i gardencorev1beta1.MachineImage) string { return i.Name }
	machineImageVersionKeyFunc = func(v gardencorev1beta1.MachineImageVersion) string { return v.Version }
	machineTypeKeyFunc         = func(t gardencorev1beta1.MachineType) string { return t.Name }
	volumeTypeKeyFunc          = func(t gardencorev1beta1.VolumeType) string { return t.Name }
)

// getExpirationStage return a pointer to the expired stage of an ExpirableVersion or nil if not found.
func getExpirationStage(v *gardencorev1beta1.ExpirableVersion) *gardencorev1beta1.LifecycleStage {
	for i := range v.Lifecycle {
		if v.Lifecycle[i].Classification == gardencorev1beta1.ClassificationExpired {
			return &v.Lifecycle[i]
		}
	}
	return nil
}

// ApplyExpirableVersionOverrides applies a NamespacedCloudProfile override to a CloudProfile ExpirableVersion.
// The behavior depends on whether the base and override use the legacy- or lifecycle classification:
//   - legacy / legacy: preserve existing behavior and only add or replace the expiration date.
//   - lifecycle / legacy: add or replace only the expired lifecycle stage and preserve all other stages.
//   - legacy / lifecycle: the override lifecycle is authoritative and replaces the legacy classification fields
//   - lifecycle / lifecycle: the override lifecycle is authoritative and replaces the base lifecycle
func ApplyExpirableVersionOverrides(base, override gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	baseUsesLifecycle := len(base.Lifecycle) > 0
	overrideUsesLifecycle := len(override.Lifecycle) > 0

	if !overrideUsesLifecycle && override.ExpirationDate == nil {
		// Removal of expiration in legacy classification is not allowed.
		return base
	}

	if overrideUsesLifecycle {
		return gardencorev1beta1.ExpirableVersion{
			Version:   base.Version,
			Lifecycle: slices.Clone(override.Lifecycle),
		}
	}

	if !baseUsesLifecycle {
		base.ExpirationDate = override.ExpirationDate.DeepCopy()
		return base
	}

	base.Lifecycle = slices.Clone(base.Lifecycle)
	if baseExpiryStage := getExpirationStage(&base); baseExpiryStage != nil {
		baseExpiryStage.StartTime = override.ExpirationDate.DeepCopy()
	} else {
		base.Lifecycle = append(base.Lifecycle, gardencorev1beta1.LifecycleStage{
			Classification: gardencorev1beta1.ClassificationExpired,
			StartTime:      override.ExpirationDate.DeepCopy(),
		})
	}

	return base
}

func mergeMachineImages(base, override gardencorev1beta1.MachineImage) gardencorev1beta1.MachineImage {
	if ptr.Deref(override.UpdateStrategy, "") != "" {
		base.UpdateStrategy = override.UpdateStrategy
	}
	base.Versions = mergeDeep(base.Versions, override.Versions, machineImageVersionKeyFunc, mergeMachineImageVersions, true)
	return base
}

func mergeMachineImageVersions(base, override gardencorev1beta1.MachineImageVersion) gardencorev1beta1.MachineImageVersion {
	if len(override.Architectures) > 0 ||
		len(override.CapabilityFlavors) > 0 ||
		len(override.CRI) > 0 ||
		len(ptr.Deref(override.KubeletVersionConstraint, "")) > 0 ||
		len(ptr.Deref(override.Classification, "")) > 0 {
		// If the NamespacedCloudProfile machine image version has been there before, do not merge it with the parent CloudProfile machine image version.
		return override
	}
	base.ExpirableVersion = ApplyExpirableVersionOverrides(base.ExpirableVersion, override.ExpirableVersion)
	return base
}

// mergeDeep merges override slice into baseArr slice by key.
// Existing items are replaced, or merged with mergeFunc when provided.
// New override items are added only if allowAdditional is true.
// The original order from baseArr is preserved.
func mergeDeep[T any](baseArr, override []T, keyFunc func(T) string, mergeFunc func(T, T) T, allowAdditional bool) []T {
	existing := utils.CreateOrderedMapFromSlice(baseArr, keyFunc)
	for _, value := range override {
		key := keyFunc(value)
		existingValue, exists := existing.Get(key)
		if !exists {
			if allowAdditional {
				existing.Set(key, value)
			}
			continue
		}
		if mergeFunc != nil {
			existing.Set(key, mergeFunc(existingValue, value))
		} else {
			existing.Set(key, value)
		}
	}
	if res := slices.Collect(existing.Values()); res != nil {
		return res
	}
	// If the merged result is empty, slices.Collect returns nil. Instead, return the baseArr as-is (which might be an empty slice but not nil).
	return baseArr
}
