// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
)

// GetCloudProfile determines whether a given CloudProfile (plus namespace) is a valid NamespacedCloudProfile or a CloudProfile and returns the appropriate object
func GetCloudProfile(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, NamespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, shoot *core.Shoot) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfileReference := BuildCloudProfileReference(shoot)
	if cloudProfileReference.Kind == constants.CloudProfileReferenceKindNamespacedCloudProfile {
		namespacedCloudProfile, err := NamespacedCloudProfileLister.NamespacedCloudProfiles(shoot.Namespace).Get(cloudProfileReference.Name)
		if err != nil {
			return nil, err
		}
		return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
	}
	return cloudProfileLister.Get(cloudProfileReference.Name)
}

// ValidateCloudProfileChanges validates that the referenced CloudProfile does only change towards a more specific reference
// (i.e. currently only from a CloudProfile to a descendant NamespacedCloudProfile).
// For now, other changes are not supported (e.g. from one CloudProfile to another or from one NamespacedCloudProfile to another).
func ValidateCloudProfileChanges(_ gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, newShoot, oldShoot *core.Shoot) error {
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

// BuildCloudProfileReference determines the CloudProfile of a Shoot to use
// depending on the availability of cloudProfileName and cloudProfile.
func BuildCloudProfileReference(shoot *core.Shoot) *gardencorev1beta1.CloudProfileReference {
	if shoot == nil {
		return nil
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	if shoot.Spec.CloudProfile == nil {
		return nil
	}
	cloudProfileV1Beta1 := &gardencorev1beta1.CloudProfileReference{}
	if err := api.Scheme.Convert(shoot.Spec.CloudProfile, cloudProfileV1Beta1, nil); err != nil {
		return nil
	}
	return cloudProfileV1Beta1
}
