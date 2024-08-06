// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/features"
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

// ValidateCloudProfileChanges validates that the referenced CloudProfile does only change towards a more specific reference
// (i.e. currently only from a CloudProfile to a descendant NamespacedCloudProfile).
// For now, other changes are not supported (e.g. from one CloudProfile to another or from one NamespacedCloudProfile to another).
func ValidateCloudProfileChanges(namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, newShoot, oldShoot *core.Shoot) error {
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
	if oldCloudProfileReference.Kind == constants.CloudProfileReferenceKindCloudProfile &&
		newCloudProfileReference.Kind == constants.CloudProfileReferenceKindNamespacedCloudProfile &&
		equality.Semantic.DeepEqual(oldCloudProfileReference, newCloudProfileRoot) {
		return nil
	}

	return fmt.Errorf("a Seed's CloudProfile may only be changed to a descendant NamespacedCloudProfile, other modifications are currently not supported")
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

// MergeCloudProfiles builds the cloud profile spec from a base CloudProfile and a NamespacedCloudProfile by updating the CloudProfile Spec inplace.
// The CloudProfile Spec can then be used as NamespacedCloudProfile.Status.CloudProfileSpec value.
func MergeCloudProfiles(resultingCloudProfile *gardencorev1beta1.CloudProfile, namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) {
	resultingCloudProfile.ObjectMeta = metav1.ObjectMeta{}
	if namespacedCloudProfile.Spec.Kubernetes != nil {
		resultingCloudProfile.Spec.Kubernetes.Versions = mergeDeep(resultingCloudProfile.Spec.Kubernetes.Versions, namespacedCloudProfile.Spec.Kubernetes.Versions, func(v gardencorev1beta1.ExpirableVersion) string { return v.Version }, mergeExpirationDates, false)
	}
	resultingCloudProfile.Spec.MachineImages = mergeDeep(resultingCloudProfile.Spec.MachineImages, namespacedCloudProfile.Spec.MachineImages, func(image gardencorev1beta1.MachineImage) string { return image.Name }, mergeMachineImages, false)
	resultingCloudProfile.Spec.MachineTypes = mergeDeep(resultingCloudProfile.Spec.MachineTypes, namespacedCloudProfile.Spec.MachineTypes, func(machineType gardencorev1beta1.MachineType) string { return machineType.Name }, nil, true)
	resultingCloudProfile.Spec.Regions = append(resultingCloudProfile.Spec.Regions, namespacedCloudProfile.Spec.Regions...)
	resultingCloudProfile.Spec.VolumeTypes = append(resultingCloudProfile.Spec.VolumeTypes, namespacedCloudProfile.Spec.VolumeTypes...)
	if namespacedCloudProfile.Spec.CABundle != nil {
		mergedCABundles := fmt.Sprintf("%s%s", ptr.Deref(resultingCloudProfile.Spec.CABundle, ""), ptr.Deref(namespacedCloudProfile.Spec.CABundle, ""))
		resultingCloudProfile.Spec.CABundle = &mergedCABundles
	}
}

func mergeExpirationDates(base, override gardencorev1beta1.ExpirableVersion) gardencorev1beta1.ExpirableVersion {
	base.ExpirationDate = override.ExpirationDate
	return base
}

func mergeMachineImages(base, override gardencorev1beta1.MachineImage) gardencorev1beta1.MachineImage {
	base.Versions = mergeDeep(base.Versions, override.Versions, func(v gardencorev1beta1.MachineImageVersion) string { return v.Version }, mergeMachineImageVersions, false)
	return base
}

func mergeMachineImageVersions(base, override gardencorev1beta1.MachineImageVersion) gardencorev1beta1.MachineImageVersion {
	base.ExpirableVersion = mergeExpirationDates(base.ExpirableVersion, override.ExpirableVersion)
	return base
}

// Values converts the values of a map to an array.
func Values[T any](m map[string]T) []T {
	var values []T
	for _, version := range m {
		values = append(values, version)
	}
	return values
}

// MapOf converts the values of an array to a map using a key function.
func MapOf[T any](arr []T, keyFunc func(T) string) map[string]T {
	mapped := make(map[string]T, len(arr))
	for _, value := range arr {
		mapped[keyFunc(value)] = value
	}
	return mapped
}

func mergeDeep[T any](baseArr, override []T, keyFunc func(T) string, mergeFunc func(T, T) T, allowAdditional bool) []T {
	existing := MapOf(baseArr, keyFunc)
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
	return Values(existing)
}
