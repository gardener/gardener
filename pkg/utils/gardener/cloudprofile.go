// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetCloudProfile determines whether the given shoot references a CloudProfile or a NamespacedCloudProfile and returns the appropriate object.
func GetCloudProfile(ctx context.Context, reader client.Reader, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfileReference := BuildCloudProfileReference(shoot)
	if cloudProfileReference == nil {
		return nil, fmt.Errorf("could not determine cloudprofile from shoot")
	}
	var cloudProfile *gardencorev1beta1.CloudProfile
	switch cloudProfileReference.Kind {
	case constants.CloudProfileReferenceKindCloudProfile:
		cloudProfile = &gardencorev1beta1.CloudProfile{}
		if err := reader.Get(ctx, client.ObjectKey{Name: cloudProfileReference.Name}, cloudProfile); err != nil {
			return nil, err
		}
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
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

// BuildCloudProfileReference determines and returns the CloudProfile reference of the given shoot,
// depending on the availability of cloudProfileName and cloudProfile.
func BuildCloudProfileReference(shoot *gardencorev1beta1.Shoot) *gardencorev1beta1.CloudProfileReference {
	if shoot == nil {
		return nil
	}
	if shoot.Spec.CloudProfile != nil {
		cloudProfileReference := shoot.Spec.CloudProfile.DeepCopy()
		if len(cloudProfileReference.Kind) == 0 {
			cloudProfileReference.Kind = constants.CloudProfileReferenceKindCloudProfile
		}
		return cloudProfileReference
	}
	if len(ptr.Deref(shoot.Spec.CloudProfileName, "")) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *shoot.Spec.CloudProfileName,
			Kind: constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	return nil
}
