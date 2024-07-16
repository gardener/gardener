// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"

	"k8s.io/utils/ptr"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetCloudProfile gets determine whether a given CloudProfile name is a NamespacedCloudProfile or a CloudProfile and returns the appropriate object
func GetCloudProfile(ctx context.Context, client pkgclient.Reader, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfileReference := BuildCloudProfileReference(shoot)
	if cloudProfileReference == nil {
		return nil, fmt.Errorf("could not determine cloudprofile from shoot")
	}
	switch cloudProfileReference.Kind {
	case constants.CloudProfileReferenceKindCloudProfile:
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		err := client.Get(ctx, pkgclient.ObjectKey{Name: cloudProfileReference.Name}, cloudProfile)
		return cloudProfile, err
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
		namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
		if err := client.Get(ctx, pkgclient.ObjectKey{Name: cloudProfileReference.Name, Namespace: shoot.Namespace}, namespacedCloudProfile); err != nil {
			return nil, err
		}
		return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
	}
	return nil, fmt.Errorf("could not get cloud profile for reference: %+v", cloudProfileReference)
}

// BuildCloudProfileReference determines the CloudProfile of a Shoot to use
// depending on the availability of cloudProfileName and cloudProfile.
func BuildCloudProfileReference(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.CloudProfileReference {
	if shoot == nil {
		return nil
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	return shoot.Spec.CloudProfile
}
