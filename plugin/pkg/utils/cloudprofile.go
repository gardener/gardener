// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"slices"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/util"
	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
	gardenerutils "github.com/gardener/gardener/pkg/utils"
)

// GetCloudProfileSpec determines whether the given shoot references a CloudProfile or a NamespacedCloudProfile and returns the appropriate CloudProfileSpec.
func GetCloudProfileSpec(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, shoot *core.Shoot) (*gardencorev1beta1.CloudProfileSpec, error) {
	cloudProfileReference := BuildCloudProfileReference(shoot)
	if cloudProfileReference == nil {
		return nil, fmt.Errorf("no cloudprofile reference has been provided")
	}
	switch cloudProfileReference.Kind {
	case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
		namespacedCloudProfile, err := namespacedCloudProfileLister.NamespacedCloudProfiles(shoot.Namespace).Get(cloudProfileReference.Name)
		if err != nil {
			return nil, err
		}
		return &namespacedCloudProfile.Status.CloudProfileSpec, nil
	case v1beta1constants.CloudProfileReferenceKindCloudProfile:
		cloudProfile, err := cloudProfileLister.Get(cloudProfileReference.Name)
		if err != nil {
			return nil, err
		}
		return &cloudProfile.Spec, nil
	}
	return nil, fmt.Errorf("could not find referenced cloudprofile")
}

// ValidateCloudProfileChanges validates that the referenced CloudProfile only changes within the current profile hierarchy
// (i.e. between the parent CloudProfile and the descendant NamespacedCloudProfiles) and that upon changing the profile all
// current configurations still stay valid.
func ValidateCloudProfileChanges(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, newShoot, oldShoot *core.Shoot) error {
	oldCloudProfileReference := BuildCloudProfileReference(oldShoot)
	if oldCloudProfileReference == nil {
		return nil
	}
	newCloudProfileReference := BuildCloudProfileReference(newShoot)
	if apiequality.Semantic.DeepEqual(oldCloudProfileReference, newCloudProfileReference) {
		return nil
	}

	newCloudProfileRoot, err := getRootCloudProfile(namespacedCloudProfileLister, newCloudProfileReference, newShoot.Namespace)
	if err != nil {
		return err
	}
	oldCloudProfileRoot, err := getRootCloudProfile(namespacedCloudProfileLister, oldCloudProfileReference, oldShoot.Namespace)
	if err != nil {
		return err
	}

	if !apiequality.Semantic.DeepEqual(oldCloudProfileRoot, newCloudProfileRoot) {
		fromProfile := fmt.Sprintf("%q", oldCloudProfileReference.Name)
		if oldCloudProfileReference.Kind != v1beta1constants.CloudProfileReferenceKindCloudProfile {
			fromProfile += fmt.Sprintf(" (root: %q)", oldCloudProfileRoot.Name)
		}
		toProfile := fmt.Sprintf("%q", newCloudProfileReference.Name)
		if newCloudProfileReference.Kind != v1beta1constants.CloudProfileReferenceKindCloudProfile {
			toProfile += fmt.Sprintf(" (root: %q)", newCloudProfileRoot.Name)
		}
		return fmt.Errorf("cloud profile reference change is invalid: cannot change from %s to %s. The cloud profile reference must remain within the same hierarchy", fromProfile, toProfile)
	}

	if !apiequality.Semantic.DeepEqual(newCloudProfileReference, oldCloudProfileReference) {
		newCloudProfileSpec, err := GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, newShoot)
		if err != nil {
			return fmt.Errorf("could not find cloudProfileSpec from the shoot cloudProfile reference: %s", err.Error())
		}
		newCloudProfileSpecCore := &core.CloudProfileSpec{}
		if err := api.Scheme.Convert(newCloudProfileSpec, newCloudProfileSpecCore, nil); err != nil {
			return err
		}
		oldCloudProfileSpec, err := GetCloudProfileSpec(cloudProfileLister, namespacedCloudProfileLister, oldShoot)
		if err != nil {
			return fmt.Errorf("could not find cloudProfileSpec from the shoot cloudProfile reference: %s", err.Error())
		}
		oldCloudProfileSpecCore := &core.CloudProfileSpec{}
		if err := api.Scheme.Convert(oldCloudProfileSpec, oldCloudProfileSpecCore, nil); err != nil {
			return err
		}

		// Check that the target cloud profile still supports the currently used machine types, machine images and volume types.
		// No need to check for Kubernetes versions, as the NamespacedCloudProfile could have only extended a version so with the next maintenance the Shoot will be updated to a supported version.
		_, removedMachineImageVersions, _, _ := gardencorehelper.GetMachineImageDiff(oldCloudProfileSpecCore.MachineImages, newCloudProfileSpecCore.MachineImages)
		machineTypes := gardenerutils.CreateMapFromSlice(newCloudProfileSpec.MachineTypes, func(mt gardencorev1beta1.MachineType) string { return mt.Name })
		volumeTypes := gardenerutils.CreateMapFromSlice(newCloudProfileSpec.VolumeTypes, func(vt gardencorev1beta1.VolumeType) string { return vt.Name })

		for _, w := range newShoot.Spec.Provider.Workers {
			if len(removedMachineImageVersions) > 0 && w.Machine.Image != nil {
				if removedVersions, exists := removedMachineImageVersions[w.Machine.Image.Name]; exists {
					if removedVersions.Has(w.Machine.Image.Version) {
						return fmt.Errorf("newly referenced cloud profile does not contain the machine image version \"%s@%s\" currently in use by worker \"%s\"", w.Machine.Image.Name, w.Machine.Image.Version, w.Name)
					}
				}
			}

			if _, exists := machineTypes[w.Machine.Type]; !exists {
				return fmt.Errorf("newly referenced cloud profile does not contain the machine type %q currently in use by worker %q", w.Machine.Type, w.Name)
			}

			if w.Volume != nil && w.Volume.Type != nil {
				if _, exists := volumeTypes[*w.Volume.Type]; !exists {
					return fmt.Errorf("newly referenced cloud profile does not contain the volume type %q currently in use by worker %q", *w.Volume.Type, w.Name)
				}
			}
		}
	}
	return nil
}

// getRootCloudProfile determines the root CloudProfile from a CloudProfileReference containing any (Namespaced)CloudProfile
func getRootCloudProfile(namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, cloudProfile *gardencorev1beta1.CloudProfileReference, namespace string) (*gardencorev1beta1.CloudProfileReference, error) {
	if cloudProfile == nil {
		return nil, errors.New("unexpected nil cloudprofile to get root of")
	}
	switch cloudProfile.Kind {
	case v1beta1constants.CloudProfileReferenceKindCloudProfile:
		return cloudProfile, nil
	case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
		cp, err := namespacedCloudProfileLister.NamespacedCloudProfiles(namespace).Get(cloudProfile.Name)
		if err != nil {
			return nil, err
		}
		return getRootCloudProfile(namespacedCloudProfileLister, &cp.Spec.Parent, namespace)
	}
	return nil, fmt.Errorf("unexpected cloudprofile kind %s", cloudProfile.Kind)
}

// BuildCloudProfileReference determines and returns the CloudProfile reference of the given shoot,
// depending on the availability of cloudProfileName and cloudProfile.
func BuildCloudProfileReference(shoot *core.Shoot) *gardencorev1beta1.CloudProfileReference {
	if shoot == nil {
		return nil
	}
	if shoot.Spec.CloudProfile != nil {
		cloudProfileV1Beta1 := &gardencorev1beta1.CloudProfileReference{}
		if err := api.Scheme.Convert(shoot.Spec.CloudProfile, cloudProfileV1Beta1, nil); err != nil {
			return nil
		}
		if len(cloudProfileV1Beta1.Kind) == 0 {
			cloudProfileV1Beta1.Kind = v1beta1constants.CloudProfileReferenceKindCloudProfile
		}
		return cloudProfileV1Beta1
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: v1beta1constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	return nil
}

// SyncCloudProfileFields handles the coexistence of a Shoot Spec's cloudProfileName and cloudProfile
// by making sure both fields are synced correctly and appropriate fallback cases are handled.
func SyncCloudProfileFields(oldShoot, newShoot *core.Shoot) {
	if newShoot.DeletionTimestamp != nil {
		return
	}

	// clear cloudProfile if namespacedCloudProfile is newly provided but feature toggle is disabled
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile && !utilfeature.DefaultFeatureGate.Enabled(features.UseNamespacedCloudProfile) &&
		(oldShoot == nil || oldShoot.Spec.CloudProfile == nil || oldShoot.Spec.CloudProfile.Kind != v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile) {
		newShoot.Spec.CloudProfile = nil
	}

	// fill empty cloudProfile field from cloudProfileName, if provided
	if newShoot.Spec.CloudProfile == nil && newShoot.Spec.CloudProfileName != nil {
		newShoot.Spec.CloudProfile = &core.CloudProfileReference{
			Kind: v1beta1constants.CloudProfileReferenceKindCloudProfile,
			Name: *newShoot.Spec.CloudProfileName,
		}
	}

	// default empty cloudprofile kind to CloudProfile
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == "" {
		newShoot.Spec.CloudProfile.Kind = v1beta1constants.CloudProfileReferenceKindCloudProfile
	}

	// fill cloudProfileName from cloudProfile if provided and kind is CloudProfile
	// for backwards compatibility (esp. Dashboard), the CloudProfileName field is synced here with the referenced CloudProfile
	// TODO(LucaBernstein): Remove this block as soon as the CloudProfileName field is deprecated.
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == v1beta1constants.CloudProfileReferenceKindCloudProfile {
		newShoot.Spec.CloudProfileName = &newShoot.Spec.CloudProfile.Name
	}

	// if other than CloudProfile is provided, unset cloudProfileName
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind != v1beta1constants.CloudProfileReferenceKindCloudProfile {
		newShoot.Spec.CloudProfileName = nil
	}
}

// SyncArchitectureCapabilityFields syncs the architecture capabilities and the architecture fields.
func SyncArchitectureCapabilityFields(newCloudProfileSpec core.CloudProfileSpec, oldCloudProfileSpec core.CloudProfileSpec) {
	hasCapabilities := len(newCloudProfileSpec.Capabilities) > 0
	if !hasCapabilities || !gardencorehelper.HasCapability(newCloudProfileSpec.Capabilities, v1beta1constants.ArchitectureName) {
		return
	}

	isInitialMigration := hasCapabilities && len(oldCloudProfileSpec.Capabilities) == 0

	// During the initial migration to capabilities, synchronize the architecture fields with the capability definitions.
	// After the migration, only sync architectures from the capability definitions back to the architecture fields.
	// This approach ensures that capabilities are consistently used once defined.
	// Any mismatch between capabilities and architecture fields will result in a validation error.
	syncMachineImageArchitectureCapabilities(newCloudProfileSpec.MachineImages, oldCloudProfileSpec.MachineImages, isInitialMigration)
	syncMachineTypeArchitectureCapabilities(newCloudProfileSpec.MachineTypes, oldCloudProfileSpec.MachineTypes, isInitialMigration)
}

func syncMachineImageArchitectureCapabilities(newMachineImages, oldMachineImages []core.MachineImage, isInitialMigration bool) {
	oldMachineImagesMap := util.NewCoreImagesContext(oldMachineImages)

	for imageIdx, image := range newMachineImages {
		for versionIdx, version := range newMachineImages[imageIdx].Versions {
			oldMachineImageVersion, oldVersionExists := oldMachineImagesMap.GetImageVersion(image.Name, version.Version)
			capabilityArchitectures := gardencorehelper.ExtractArchitecturesFromCapabilitySets(version.CapabilitySets)

			// Skip any architecture field syncing if
			// - architecture field has been modified and changed to any value other than empty.
			architecturesFieldHasBeenChanged := oldVersionExists && len(version.Architectures) > 0 &&
				!apiequality.Semantic.DeepEqual(oldMachineImageVersion.Architectures, version.Architectures)

			// - both the architecture field and the architecture capability are empty or filled equally.
			if architecturesFieldHasBeenChanged || slices.Equal(capabilityArchitectures, version.Architectures) {
				continue
			}

			// Sync architecture field to capabilities if filled on initial migration.
			if isInitialMigration && len(version.Architectures) > 0 && len(version.CapabilitySets) == 0 {
				for _, architecture := range version.Architectures {
					newMachineImages[imageIdx].Versions[versionIdx].CapabilitySets = append(newMachineImages[imageIdx].Versions[versionIdx].CapabilitySets,
						core.CapabilitySet{
							Capabilities: core.Capabilities{
								v1beta1constants.ArchitectureName: []string{architecture},
							},
						})
				}
				continue
			}

			// Sync capability architectures to architectures field.
			if len(capabilityArchitectures) > 0 {
				newMachineImages[imageIdx].Versions[versionIdx].Architectures = capabilityArchitectures
			}
		}
	}
}

func syncMachineTypeArchitectureCapabilities(newMachineTypes, oldMachineTypes []core.MachineType, isInitialMigration bool) {
	oldMachineTypesMap := gardenerutils.CreateMapFromSlice(oldMachineTypes, func(machineType core.MachineType) string { return machineType.Name })

	for i, machineType := range newMachineTypes {
		oldMachineType, oldMachineTypeExists := oldMachineTypesMap[machineType.Name]
		architectureValue := ptr.Deref(machineType.Architecture, "")
		oldArchitectureValue := ptr.Deref(oldMachineType.Architecture, "")
		capabilityArchitectures := machineType.Capabilities[v1beta1constants.ArchitectureName]

		// Skip any architecture field syncing if
		// - architecture field has been modified and changed to any value other than empty.
		architectureFieldHasBeenChanged := oldMachineTypeExists && architectureValue != "" &&
			(oldArchitectureValue == "" || oldArchitectureValue != architectureValue)
		// - both the architecture field and the architecture capability are empty or filled equally.
		architecturesInSync := len(capabilityArchitectures) == 0 && architectureValue == "" ||
			len(capabilityArchitectures) == 1 && capabilityArchitectures[0] == architectureValue
		if architectureFieldHasBeenChanged || architecturesInSync {
			continue
		}

		// Sync architecture field to capabilities if filled on initial migration.
		if isInitialMigration && architectureValue != "" && len(capabilityArchitectures) == 0 {
			if newMachineTypes[i].Capabilities == nil {
				newMachineTypes[i].Capabilities = make(core.Capabilities)
			}
			newMachineTypes[i].Capabilities[v1beta1constants.ArchitectureName] = []string{architectureValue}
			continue
		}

		// Sync capability architecture to architecture field.
		if len(capabilityArchitectures) == 1 {
			newMachineTypes[i].Architecture = ptr.To(capabilityArchitectures[0])
		}
	}
}
