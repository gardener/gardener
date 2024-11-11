// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
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
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
		namespacedCloudProfile, err := namespacedCloudProfileLister.NamespacedCloudProfiles(shoot.Namespace).Get(cloudProfileReference.Name)
		if err != nil {
			return nil, err
		}
		return &namespacedCloudProfile.Status.CloudProfileSpec, nil
	case constants.CloudProfileReferenceKindCloudProfile:
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
	if equality.Semantic.DeepEqual(oldCloudProfileReference, newCloudProfileReference) {
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

	if !equality.Semantic.DeepEqual(oldCloudProfileRoot, newCloudProfileRoot) {
		fromProfile := fmt.Sprintf("%q", oldCloudProfileReference.Name)
		if oldCloudProfileReference.Kind != constants.CloudProfileReferenceKindCloudProfile {
			fromProfile += fmt.Sprintf(" (root: %q)", oldCloudProfileRoot.Name)
		}
		toProfile := fmt.Sprintf("%q", newCloudProfileReference.Name)
		if newCloudProfileReference.Kind != constants.CloudProfileReferenceKindCloudProfile {
			toProfile += fmt.Sprintf(" (root: %q)", newCloudProfileRoot.Name)
		}
		return fmt.Errorf("cloud profile reference change is invalid: cannot change from %s to %s. The cloud profile reference must remain within the same hierarchy", fromProfile, toProfile)
	}

	if !equality.Semantic.DeepEqual(newCloudProfileReference, oldCloudProfileReference) {
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
		_, removedMachineImageVersions, _, _ := helper.GetMachineImageDiff(oldCloudProfileSpecCore.MachineImages, newCloudProfileSpecCore.MachineImages)
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
	case constants.CloudProfileReferenceKindCloudProfile:
		return cloudProfile, nil
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
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
			cloudProfileV1Beta1.Kind = constants.CloudProfileReferenceKindCloudProfile
		}
		return cloudProfileV1Beta1
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: constants.CloudProfileReferenceKindCloudProfile,
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
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == constants.CloudProfileReferenceKindNamespacedCloudProfile && !utilfeature.DefaultFeatureGate.Enabled(features.UseNamespacedCloudProfile) &&
		(oldShoot == nil || oldShoot.Spec.CloudProfile == nil || oldShoot.Spec.CloudProfile.Kind != constants.CloudProfileReferenceKindNamespacedCloudProfile) {
		newShoot.Spec.CloudProfile = nil
	}

	// fill empty cloudProfile field from cloudProfileName, if provided
	if newShoot.Spec.CloudProfile == nil && newShoot.Spec.CloudProfileName != nil {
		newShoot.Spec.CloudProfile = &core.CloudProfileReference{
			Kind: constants.CloudProfileReferenceKindCloudProfile,
			Name: *newShoot.Spec.CloudProfileName,
		}
	}

	// default empty cloudprofile kind to CloudProfile
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == "" {
		newShoot.Spec.CloudProfile.Kind = constants.CloudProfileReferenceKindCloudProfile
	}

	// fill cloudProfileName from cloudProfile if provided and kind is CloudProfile
	// for backwards compatibility (esp. Dashboard), the CloudProfileName field is synced here with the referenced CloudProfile
	// TODO(LucaBernstein): Remove this block as soon as the CloudProfileName field is deprecated.
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind == constants.CloudProfileReferenceKindCloudProfile {
		newShoot.Spec.CloudProfileName = &newShoot.Spec.CloudProfile.Name
	}

	// if other than CloudProfile is provided, unset cloudProfileName
	if newShoot.Spec.CloudProfile != nil && newShoot.Spec.CloudProfile.Kind != constants.CloudProfileReferenceKindCloudProfile {
		newShoot.Spec.CloudProfileName = nil
	}
}
