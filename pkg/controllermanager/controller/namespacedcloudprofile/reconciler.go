// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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

	"github.com/gardener/gardener/pkg/api"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
)

// Reconciler reconciles NamespacedCloudProfiles.
type Reconciler struct {
	Client   client.Client
	Config   controllermanagerconfigv1alpha1.NamespacedCloudProfileControllerConfiguration
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
	patch := client.MergeFrom(namespacedCloudProfile.DeepCopy())
	MergeCloudProfiles(namespacedCloudProfile, parentCloudProfile)
	namespacedCloudProfile.Status.ObservedGeneration = namespacedCloudProfile.Generation
	return c.Status().Patch(ctx, namespacedCloudProfile, patch)
}

// MergeCloudProfiles merges the cloud profile spec from a base CloudProfile and a NamespacedCloudProfile
// into the NamespacedCloudProfile.Status.CloudProfileSpec.
func MergeCloudProfiles(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile, cloudProfile *gardencorev1beta1.CloudProfile) {
	namespacedCloudProfile.Status.CloudProfileSpec = cloudProfile.Spec

	if namespacedCloudProfile.Spec.Kubernetes != nil {
		namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions = mergeDeep(namespacedCloudProfile.Status.CloudProfileSpec.Kubernetes.Versions, namespacedCloudProfile.Spec.Kubernetes.Versions, expirableVersionKeyFunc, mergeExpirationDates, false)
	}

	// @Roncossek: Remove ensureUniformFormat once all CloudProfiles have been migrated to use CapabilityFlavors and the Architecture fields are deprecated.
	uniformNamespacedCloudProfileSpec := ensureUniformFormat(namespacedCloudProfile.Spec, cloudProfile.Spec.MachineCapabilities)
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

// ensureUniformFormat ensures that the given NamespacedCloudProfileSpec is in a uniform format with its parent CloudProfileSpec.
// If the parent CloudProfileSpec uses capability definitions, then the NamespacedCloudProfileSpec is transformed to also use capabilities
// and vice versa.
func ensureUniformFormat(
	spec gardencorev1beta1.NamespacedCloudProfileSpec,
	capabilityDefinitions []gardencorev1beta1.CapabilityDefinition,
) gardencorev1beta1.NamespacedCloudProfileSpec {
	isParentInCapabilityFormat := len(capabilityDefinitions) > 0
	transformedSpec := spec.DeepCopy()

	// Normalize MachineImages
	for idx, machineImage := range transformedSpec.MachineImages {
		for idy, version := range machineImage.Versions {
			legacyArchitectures := version.Architectures

			if isParentInCapabilityFormat && len(version.CapabilityFlavors) == 0 {
				// Convert legacy architectures to capability flavors
				version.CapabilityFlavors = make([]gardencorev1beta1.MachineImageFlavor, len(legacyArchitectures))
				for i, arch := range legacyArchitectures {
					version.CapabilityFlavors[i] = gardencorev1beta1.MachineImageFlavor{
						Capabilities: gardencorev1beta1.Capabilities{
							v1beta1constants.ArchitectureName: []string{arch},
						},
					}
				}
				version.Architectures = legacyArchitectures
			} else if !isParentInCapabilityFormat {
				// Convert capability flavors to legacy architectures
				if len(legacyArchitectures) == 0 && len(version.CapabilityFlavors) > 0 {
					architectureSet := sets.New[string]()
					for _, flavor := range version.CapabilityFlavors {
						architectureSet.Insert(flavor.Capabilities[v1beta1constants.ArchitectureName]...)
					}
					version.Architectures = architectureSet.UnsortedList()
				}
				version.CapabilityFlavors = nil
			}

			transformedSpec.MachineImages[idx].Versions[idy] = version
		}
	}

	// Normalize MachineTypes
	for idx, machineType := range transformedSpec.MachineTypes {
		if isParentInCapabilityFormat {
			if len(machineType.Capabilities) > 0 {
				continue
			}
			architecture := machineType.GetArchitecture(capabilityDefinitions)
			if architecture == "" {
				architecture = ptr.Deref(machineType.Architecture, "")
			}
			if architecture == "" {
				architecture = v1beta1constants.ArchitectureAMD64
			}
			machineType.Capabilities = gardencorev1beta1.Capabilities{
				v1beta1constants.ArchitectureName: []string{architecture},
			}
		} else {
			machineType.Capabilities = nil
		}
		transformedSpec.MachineTypes[idx] = machineType
	}

	return *transformedSpec
}

func syncArchitectureCapabilities(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) {
	var coreCloudProfileSpec gardencore.CloudProfileSpec
	_ = api.Scheme.Convert(&namespacedCloudProfile.Status.CloudProfileSpec, &coreCloudProfileSpec, nil)
	gardenerutils.SyncArchitectureCapabilityFields(coreCloudProfileSpec, gardencore.CloudProfileSpec{})
	defaultMachineTypeArchitectures(coreCloudProfileSpec)
	_ = api.Scheme.Convert(&coreCloudProfileSpec, &namespacedCloudProfile.Status.CloudProfileSpec, nil)
}

// defaultMachineTypeArchitectures defaults the architectures of the machine types for NamespacedCloudProfiles.
// The sync can only happen after having had a look at the parent CloudProfile and whether it uses capabilities.
func defaultMachineTypeArchitectures(cloudProfile gardencore.CloudProfileSpec) {
	for i, machineType := range cloudProfile.MachineTypes {
		if machineType.GetArchitecture() == "" {
			cloudProfile.MachineTypes[i].Architecture = ptr.To(v1beta1constants.ArchitectureAMD64)
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

func mergeExpirationDates(base, override gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	base.ExpirationDate = override.ExpirationDate
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
