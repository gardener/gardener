// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"fmt"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardencorev1beta1listers "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/gardener"
)

// GetCloudProfile determines whether a given CloudProfile (plus namespace) is a valid NamespacedCloudProfile or a CloudProfile and returns the appropriate object
func GetCloudProfile(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, NamespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, cp *core.CloudProfileReference, name *string, namespace string) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfileReference := gardener.BuildCloudProfileReference(name, cp)

	if cloudProfileReference.Kind == constants.CloudProfileReferenceKindNamespacedCloudProfile {
		var ncpErr error
		namespacedCloudProfile, ncpErr := NamespacedCloudProfileLister.NamespacedCloudProfiles(namespace).Get(cloudProfileReference.Name)
		if ncpErr == nil {
			return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
		}
		if !apierrors.IsNotFound(ncpErr) {
			return nil, ncpErr
		}
	}

	cloudProfile, err := cloudProfileLister.Get(cloudProfileReference.Name)
	if err == nil {
		return cloudProfile, nil
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	return nil, fmt.Errorf("could not get cloud profile: %+v", err.Error())
}

// ValidateCloudProfileChanges validates that CloudProfiles referenced are all the same CloudProfile root node or children thereof (NamespacedCloudProfiles with the root node as parent)
func ValidateCloudProfileChanges(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, namespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, newShootSpec, oldShootSpec core.ShootSpec, namespace string) error {
	oldCloudProfileReference := gardener.BuildCloudProfileReference(oldShootSpec.CloudProfileName, oldShootSpec.CloudProfile)
	if oldCloudProfileReference == nil {
		return nil
	}
	newCloudProfileReference := gardener.BuildCloudProfileReference(newShootSpec.CloudProfileName, newShootSpec.CloudProfile)
	if equality.Semantic.DeepEqual(oldCloudProfileReference, newCloudProfileReference) {
		return nil
	}

	oldCloudProfileRoot, err := GetRootCloudProfile(cloudProfileLister, namespacedCloudProfileLister, oldCloudProfileReference, namespace)
	if err != nil {
		return err
	}
	newCloudProfileRoot, err := GetRootCloudProfile(cloudProfileLister, namespacedCloudProfileLister, newCloudProfileReference, namespace)
	if err != nil {
		return err
	}
	if equality.Semantic.DeepEqual(oldCloudProfileRoot, newCloudProfileRoot) {
		return nil
	}
	return fmt.Errorf("(Namespaced)CloudProfile root nodes do not match for %s (%s) and %s (%s)", oldCloudProfileReference.Name, oldCloudProfileRoot.Name, newCloudProfileReference.Name, newCloudProfileRoot.Name)
}

func GetRootCloudProfile(cloudProfileLister gardencorev1beta1listers.CloudProfileLister, NamespacedCloudProfileLister gardencorev1beta1listers.NamespacedCloudProfileLister, cloudProfile *gardencorev1beta1.CloudProfileReference, namespace string) (*gardencorev1beta1.CloudProfileReference, error) {
	if cloudProfile == nil {
		return nil, errors.New("unexpected nil cloudprofile to get root of")
	}
	switch cloudProfile.Kind {
	case constants.CloudProfileReferenceKindCloudProfile:
		return cloudProfile, nil
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
		cp, err := NamespacedCloudProfileLister.NamespacedCloudProfiles(namespace).Get(cloudProfile.Name)
		if err != nil {
			return nil, err
		}
		return GetRootCloudProfile(cloudProfileLister, NamespacedCloudProfileLister, &cp.Spec.Parent, namespace)
	}
	return nil, fmt.Errorf("unexpected cloudprofile kind %s", cloudProfile.Kind)
}
