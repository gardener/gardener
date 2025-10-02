// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/Masterminds/semver/v3"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorehelper "github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/version"
)

// ImagesContext is a helper struct to consume cloud profile images and their versions.
type ImagesContext[A any, B any] struct {
	Images map[string]A

	createVersionsMap func(A) map[string]B
	// imageVersions will be calculated lazily on first access of each key.
	imageVersions map[string]map[string]B
}

// GetCloudProfile determines whether the given shoot references a CloudProfile or a NamespacedCloudProfile and returns the appropriate object.
func GetCloudProfile(ctx context.Context, reader client.Reader, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfileReference := BuildV1beta1CloudProfileReference(shoot)
	if cloudProfileReference == nil {
		return nil, fmt.Errorf("could not determine cloudprofile from shoot")
	}
	var cloudProfile *gardencorev1beta1.CloudProfile
	switch cloudProfileReference.Kind {
	case v1beta1constants.CloudProfileReferenceKindCloudProfile:
		cloudProfile = &gardencorev1beta1.CloudProfile{}
		if err := reader.Get(ctx, client.ObjectKey{Name: cloudProfileReference.Name}, cloudProfile); err != nil {
			return nil, err
		}
	case v1beta1constants.CloudProfileReferenceKindNamespacedCloudProfile:
		namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
		if err := reader.Get(ctx, client.ObjectKey{Name: cloudProfileReference.Name, Namespace: shoot.Namespace}, namespacedCloudProfile); err != nil {
			return nil, err
		}
		cloudProfile = &gardencorev1beta1.CloudProfile{
			ObjectMeta: metav1.ObjectMeta{
				Name:      cloudProfileReference.Name,
				Namespace: shoot.Namespace,
			},
			Spec: namespacedCloudProfile.Status.CloudProfileSpec,
		}
	}
	return cloudProfile, nil
}

// BuildV1beta1CloudProfileReference determines and returns the CloudProfile reference of the given shoot,
// depending on the availability of cloudProfileName and cloudProfile.
func BuildV1beta1CloudProfileReference(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.CloudProfileReference {
	if shoot == nil {
		return nil
	}
	if shoot.Spec.CloudProfile != nil {
		cloudProfileReference := shoot.Spec.CloudProfile.DeepCopy()
		if len(cloudProfileReference.Kind) == 0 {
			cloudProfileReference.Kind = v1beta1constants.CloudProfileReferenceKindCloudProfile
		}
		return cloudProfileReference
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: v1beta1constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	return nil
}

// GetCloudProfileSpec determines whether the given shoot references a CloudProfile or a NamespacedCloudProfile and returns the appropriate CloudProfileSpec.
func GetCloudProfileSpec(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, shoot *core.Shoot) (*gardencorev1beta1.CloudProfileSpec, error) {
	cloudProfileReference := BuildCoreCloudProfileReference(shoot)
	if cloudProfileReference == nil {
		return nil, nil
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
	return nil, fmt.Errorf("could not find referenced cloud profile with kind %q", cloudProfileReference.Kind)
}

// ValidateCloudProfileChanges validates that the referenced CloudProfile only changes within the current profile hierarchy
// (i.e. between the parent CloudProfile and the descendant NamespacedCloudProfiles) and that upon changing the profile all
// current configurations still stay valid.
func ValidateCloudProfileChanges(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, newShoot, oldShoot *core.Shoot) error {
	oldCloudProfileReference := BuildCoreCloudProfileReference(oldShoot)
	if oldCloudProfileReference == nil {
		return nil
	}
	newCloudProfileReference := BuildCoreCloudProfileReference(newShoot)
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
		machineTypes := utils.CreateMapFromSlice(newCloudProfileSpec.MachineTypes, func(mt gardencorev1beta1.MachineType) string { return mt.Name })
		volumeTypes := utils.CreateMapFromSlice(newCloudProfileSpec.VolumeTypes, func(vt gardencorev1beta1.VolumeType) string { return vt.Name })

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

// BuildCoreCloudProfileReference determines and returns the CloudProfile reference of the given shoot,
// depending on the availability of cloudProfileName and cloudProfile.
func BuildCoreCloudProfileReference(shoot *core.Shoot) *gardencorev1beta1.CloudProfileReference {
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

	shootK8sVersion, _ := semver.NewVersion(newShoot.Spec.Kubernetes.Version)

	// Starting with k8s v1.34, no more syncing is taking place. Only the cloudprofile kind will be defaulted if unset.
	if shootK8sVersion != nil && version.ConstraintK8sGreaterEqual134.Check(shootK8sVersion) {
		// default empty cloudprofile kind to CloudProfile
		if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == "" {
			newShoot.Spec.CloudProfile.Kind = v1beta1constants.CloudProfileReferenceKindCloudProfile
		}
		// For update operations, an unchanged cloudProfileName will be set to nil if it equals the value in cloudProfile.name.
		if oldShoot != nil && newShoot.Spec.CloudProfileName != nil && newShoot.Spec.CloudProfile != nil &&
			newShoot.Spec.CloudProfile.Kind == v1beta1constants.CloudProfileReferenceKindCloudProfile &&
			ptr.Deref(newShoot.Spec.CloudProfileName, "") == ptr.Deref(oldShoot.Spec.CloudProfileName, "") &&
			ptr.Deref(newShoot.Spec.CloudProfileName, "") == newShoot.Spec.CloudProfile.Name {
			newShoot.Spec.CloudProfileName = nil
		}
		return
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

	// As long as shoot k8s version < v1.33: fill cloudProfileName from cloudProfile if provided and kind is CloudProfile.
	// For backwards compatibility (esp. Dashboard), the cloudProfileName field is synced here with the referenced CloudProfile.
	cloudProfileName := ptr.Deref(newShoot.Spec.CloudProfileName, "")
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == v1beta1constants.CloudProfileReferenceKindCloudProfile &&
		(shootK8sVersion == nil || version.ConstraintK8sLess133.Check(shootK8sVersion) || cloudProfileName != "" && cloudProfileName != newShoot.Spec.CloudProfile.Name) {
		newShoot.Spec.CloudProfileName = &newShoot.Spec.CloudProfile.Name
	}

	// Unset cloudProfileName if kind other than CloudProfile is provided.
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind != v1beta1constants.CloudProfileReferenceKindCloudProfile {
		newShoot.Spec.CloudProfileName = nil
	}
}

// SyncArchitectureCapabilityFields syncs the architecture capabilities and the architecture fields.
func SyncArchitectureCapabilityFields(newCloudProfileSpec core.CloudProfileSpec, oldCloudProfileSpec core.CloudProfileSpec) {
	hasCapabilities := len(newCloudProfileSpec.MachineCapabilities) > 0
	if !hasCapabilities || !gardencorehelper.HasCapability(newCloudProfileSpec.MachineCapabilities, v1beta1constants.ArchitectureName) {
		return
	}

	isInitialMigration := hasCapabilities && len(oldCloudProfileSpec.MachineCapabilities) == 0

	// During the initial migration to capabilities, synchronize the architecture fields with the capability definitions.
	// After the migration, only sync architectures from the capability definitions back to the architecture fields.
	// This approach ensures that capabilities are consistently used once defined.
	// Any mismatch between capabilities and architecture fields will result in a validation error.
	syncMachineImageArchitectureCapabilities(newCloudProfileSpec.MachineImages, oldCloudProfileSpec.MachineImages, newCloudProfileSpec.MachineCapabilities, isInitialMigration)
	syncMachineTypeArchitectureCapabilities(newCloudProfileSpec.MachineTypes, oldCloudProfileSpec.MachineTypes, newCloudProfileSpec.MachineCapabilities, isInitialMigration)
}

func syncMachineImageArchitectureCapabilities(newMachineImages, oldMachineImages []core.MachineImage, capabilityDefinitions []core.CapabilityDefinition, isInitialMigration bool) {
	oldMachineImagesMap := NewCoreImagesContext(oldMachineImages)

	for imageIdx, image := range newMachineImages {
		for versionIdx, version := range newMachineImages[imageIdx].Versions {
			oldMachineImageVersion, oldVersionExists := oldMachineImagesMap.GetImageVersion(image.Name, version.Version)
			defaultedImageFlavors := gardencorehelper.GetImageFlavorsWithAppliedDefaults(version.CapabilityFlavors, capabilityDefinitions)
			capabilityArchitectures := gardencorehelper.ExtractArchitecturesFromImageFlavors(defaultedImageFlavors)

			// Skip any architecture field syncing if
			// - architecture field has been modified and changed to any value other than empty.
			architecturesFieldHasBeenChanged := oldVersionExists && len(version.Architectures) > 0 &&
				!apiequality.Semantic.DeepEqual(oldMachineImageVersion.Architectures, version.Architectures)

			// - both the architecture field and the architecture capability are empty or filled equally.
			if architecturesFieldHasBeenChanged || slices.Equal(capabilityArchitectures, version.Architectures) {
				continue
			}

			// Sync architecture field to capabilities if filled on initial migration.
			capabilities := gardencorehelper.CapabilityDefinitionsToCapabilities(capabilityDefinitions)
			if isInitialMigration && len(version.Architectures) > 0 && len(version.CapabilityFlavors) == 0 && len(capabilities[v1beta1constants.ArchitectureName]) != 1 {
				for _, architecture := range version.Architectures {
					newMachineImages[imageIdx].Versions[versionIdx].CapabilityFlavors = append(newMachineImages[imageIdx].Versions[versionIdx].CapabilityFlavors,
						core.MachineImageFlavor{
							Capabilities: core.Capabilities{
								v1beta1constants.ArchitectureName: []string{architecture},
							},
						})
				}
				continue
			}

			// Sync capability architectures to architectures field.
			finalDefaultedImageFlavors := gardencorehelper.GetImageFlavorsWithAppliedDefaults(newMachineImages[imageIdx].Versions[versionIdx].CapabilityFlavors, capabilityDefinitions)
			defaultedCapabilityArchitectures := gardencorehelper.ExtractArchitecturesFromImageFlavors(finalDefaultedImageFlavors)
			if len(defaultedCapabilityArchitectures) > 0 {
				newMachineImages[imageIdx].Versions[versionIdx].Architectures = defaultedCapabilityArchitectures
			}
		}
	}
}

func syncMachineTypeArchitectureCapabilities(newMachineTypes, oldMachineTypes []core.MachineType, capabilityDefinitions []core.CapabilityDefinition, isInitialMigration bool) {
	oldMachineTypesMap := utils.CreateMapFromSlice(oldMachineTypes, func(machineType core.MachineType) string { return machineType.Name })

	for i, machineType := range newMachineTypes {
		oldMachineType, oldMachineTypeExists := oldMachineTypesMap[machineType.Name]
		architectureValue := ptr.Deref(machineType.Architecture, "")
		oldArchitectureValue := ptr.Deref(oldMachineType.Architecture, "")
		defaultedCapabilities := gardencorehelper.GetCapabilitiesWithAppliedDefaults(machineType.Capabilities, capabilityDefinitions)
		capabilityArchitectures := defaultedCapabilities[v1beta1constants.ArchitectureName]

		// Skip any architecture field syncing if
		// - architecture field has been modified and changed to any value other than empty.
		architectureFieldHasBeenChanged := oldMachineTypeExists && architectureValue != "" &&
			(oldArchitectureValue == "" || oldArchitectureValue != architectureValue)

		if architectureFieldHasBeenChanged {
			continue
		}

		// Sync architecture field to capabilities if filled on initial migration.
		// Only create a capability entry if the defaulted capabilities do not already contain exactly one architecture.
		capabilities := gardencorehelper.CapabilityDefinitionsToCapabilities(capabilityDefinitions)
		if isInitialMigration && architectureValue != "" && len(capabilityArchitectures) != 1 && len(capabilities[v1beta1constants.ArchitectureName]) != 1 {
			if newMachineTypes[i].Capabilities == nil {
				newMachineTypes[i].Capabilities = make(core.Capabilities)
			}
			newMachineTypes[i].Capabilities[v1beta1constants.ArchitectureName] = []string{architectureValue}
			continue
		}

		// Sync capability architecture to architecture field.
		finalDefaultedCapabilities := gardencorehelper.GetCapabilitiesWithAppliedDefaults(newMachineTypes[i].Capabilities, capabilityDefinitions)
		if len(finalDefaultedCapabilities[v1beta1constants.ArchitectureName]) == 1 {
			newMachineTypes[i].Architecture = ptr.To(finalDefaultedCapabilities[v1beta1constants.ArchitectureName][0])
		}
	}
}

// GetImage returns the image with the given name.
func (vc *ImagesContext[A, B]) GetImage(imageName string) (A, bool) {
	o, exists := vc.Images[imageName]
	return o, exists
}

// GetImageVersion returns the image version with the given name and version.
func (vc *ImagesContext[A, B]) GetImageVersion(imageName string, version string) (B, bool) {
	o, exists := vc.getImageVersions(imageName)[version]
	return o, exists
}

func (vc *ImagesContext[A, B]) getImageVersions(imageName string) map[string]B {
	if versions, exists := vc.imageVersions[imageName]; exists {
		return versions
	}
	vc.imageVersions[imageName] = vc.createVersionsMap(vc.Images[imageName])
	return vc.imageVersions[imageName]
}

// NewImagesContext creates a new generic ImagesContext.
func NewImagesContext[A any, B any](images map[string]A, createVersionsMap func(A) map[string]B) *ImagesContext[A, B] {
	return &ImagesContext[A, B]{
		Images:            images,
		createVersionsMap: createVersionsMap,
		imageVersions:     make(map[string]map[string]B),
	}
}

// NewCoreImagesContext creates a new ImagesContext for core.MachineImage.
func NewCoreImagesContext(profileImages []core.MachineImage) *ImagesContext[core.MachineImage, core.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(profileImages, func(mi core.MachineImage) string { return mi.Name }),
		func(mi core.MachineImage) map[string]core.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v core.MachineImageVersion) string { return v.Version })
		},
	)
}

// NewV1beta1ImagesContext creates a new ImagesContext for gardencorev1beta1.MachineImage.
func NewV1beta1ImagesContext(parentImages []gardencorev1beta1.MachineImage) *ImagesContext[gardencorev1beta1.MachineImage, gardencorev1beta1.MachineImageVersion] {
	return NewImagesContext(
		utils.CreateMapFromSlice(parentImages, func(mi gardencorev1beta1.MachineImage) string { return mi.Name }),
		func(mi gardencorev1beta1.MachineImage) map[string]gardencorev1beta1.MachineImageVersion {
			return utils.CreateMapFromSlice(mi.Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version })
		},
	)
}

// ValidateCapabilities validates the capabilities of a machine type or machine image against the CapabilityDefinition located in a cloud profile at spec.machineCapabilities.
// It checks if the capabilities are supported by the cloud profile and if the architecture is defined correctly.
// It returns a list of field errors if any validation fails.
func ValidateCapabilities(capabilities gardencorev1beta1.Capabilities, capabilityDefinitions []gardencorev1beta1.CapabilityDefinition, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// create map from capabilityDefinitions
	capabilityDefinitionsMap := make(map[string][]string)
	for _, capabilityDefinition := range capabilityDefinitions {
		capabilityDefinitionsMap[capabilityDefinition.Name] = capabilityDefinition.Values
	}
	supportedCapabilityKeys := slices.Collect(maps.Keys(capabilityDefinitionsMap))

	// Check if all capabilities are supported by the cloud profile
	for capabilityKey, capability := range capabilities {
		supportedValues, keyExists := capabilityDefinitionsMap[capabilityKey]
		if !keyExists {
			allErrs = append(allErrs, field.NotSupported(fldPath, capabilityKey, supportedCapabilityKeys))
			continue
		}
		for i, value := range capability {
			if !slices.Contains(supportedValues, value) {
				allErrs = append(allErrs, field.NotSupported(fldPath.Child(capabilityKey).Index(i), value, supportedValues))
			}
		}
	}

	// Check additional requirements for architecture
	// - must be defined when multiple architectures are supported by the cloud profile
	supportedArchitectures := capabilityDefinitionsMap[v1beta1constants.ArchitectureName]
	architectures := capabilities[v1beta1constants.ArchitectureName]
	if len(supportedArchitectures) > 1 && len(architectures) != 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child(v1beta1constants.ArchitectureName), architectures, "must define exactly one architecture when multiple architectures are supported by the cloud profile"))
	}

	return allErrs
}
