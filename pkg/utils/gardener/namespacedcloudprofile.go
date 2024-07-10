// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	pkgclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/api"
	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

// GetCloudProfile gets determine whether a given CloudProfile name is a NamespacedCloudProfile or a CloudProfile and returns the appropriate object
func GetCloudProfile(ctx context.Context, client pkgclient.Client, cloudProfileReference *gardencorev1beta1.CloudProfileReference, namespace string) (*gardencorev1beta1.CloudProfile, error) {
	switch cloudProfileReference.Kind {
	case constants.CloudProfileReferenceKindCloudProfile:
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		err := client.Get(ctx, pkgclient.ObjectKey{Name: cloudProfileReference.Name}, cloudProfile)
		if err == nil {
			return cloudProfile, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	case constants.CloudProfileReferenceKindNamespacedCloudProfile:
		namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
		err := client.Get(ctx, pkgclient.ObjectKey{Name: cloudProfileReference.Name, Namespace: namespace}, namespacedCloudProfile)
		if err == nil {
			return &gardencorev1beta1.CloudProfile{Spec: namespacedCloudProfile.Status.CloudProfileSpec}, nil
		}
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
	}
	return nil, fmt.Errorf("could not get cloud profile for reference: %+v", cloudProfileReference)
}

// BuildCloudProfileReference is a type overload to BuildCloudProfileReferenceV1Beta1 and determines the CloudProfile to use depending on the availability of cloudProfileName and cloudProfile
func BuildCloudProfileReference(cloudProfileName *string, cloudProfile *core.CloudProfileReference) *gardencorev1beta1.CloudProfileReference {
	var cloudProfileReferenceV1Beta1 *gardencorev1beta1.CloudProfileReference
	if cloudProfile != nil {
		cloudProfileReferenceV1Beta1 = &gardencorev1beta1.CloudProfileReference{}
		if err := api.Scheme.Convert(cloudProfile, cloudProfileReferenceV1Beta1, nil); err != nil {
			return nil
		}
	}
	return BuildCloudProfileReferenceV1Beta1(cloudProfileName, cloudProfileReferenceV1Beta1)
}

// BuildCloudProfileReferenceV1Beta1 determines the CloudProfile to use depending on the availability of cloudProfileName and cloudProfile
func BuildCloudProfileReferenceV1Beta1(cloudProfileName *string, cloudProfile *gardencorev1beta1.CloudProfileReference) *gardencorev1beta1.CloudProfileReference {
	if cloudProfileName != nil && len(*cloudProfileName) > 0 {
		return &gardencorev1beta1.CloudProfileReference{
			Name: *cloudProfileName,
			Kind: constants.CloudProfileReferenceKindCloudProfile,
		}
	}
	return cloudProfile
}
