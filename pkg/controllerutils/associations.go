// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controllerutils

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetermineShootsAssociatedTo gets a <shootLister> to determine the Shoots resources which are associated
// to given <obj> (either a CloudProfile, Seed, Secretbinding a or a ExposureClass object).
func DetermineShootsAssociatedTo(ctx context.Context, gardenClient client.Reader, obj interface{}) ([]string, error) {
	shootList := &gardencorev1beta1.ShootList{}
	if err := gardenClient.List(ctx, shootList); err != nil {
		return nil, err
	}

	var associatedShoots []string

	for _, shoot := range shootList.Items {
		switch t := obj.(type) {
		case *gardencorev1beta1.CloudProfile:
			cloudProfile := obj.(*gardencorev1beta1.CloudProfile)
			if shoot.Spec.CloudProfileName == cloudProfile.Name {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		case *gardencorev1beta1.Seed:
			seed := obj.(*gardencorev1beta1.Seed)
			if shoot.Spec.SeedName != nil && *shoot.Spec.SeedName == seed.Name {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		case *gardencorev1beta1.SecretBinding:
			binding := obj.(*gardencorev1beta1.SecretBinding)
			if shoot.Spec.SecretBindingName == binding.Name && shoot.Namespace == binding.Namespace {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		case *gardencorev1alpha1.ExposureClass:
			exposureClass := obj.(*gardencorev1alpha1.ExposureClass)
			if shoot.Spec.ExposureClassName != nil && *shoot.Spec.ExposureClassName == exposureClass.Name {
				associatedShoots = append(associatedShoots, fmt.Sprintf("%s/%s", shoot.Namespace, shoot.Name))
			}
		default:
			return nil, fmt.Errorf("unable to determine Shoot associations, due to unknown type %t", t)
		}
	}

	return associatedShoots, nil
}

// DetermineSecretBindingAssociations gets a <bindingLister> to determine the SecretBinding
// resources which are associated to given Quota <obj>.
func DetermineSecretBindingAssociations(ctx context.Context, c client.Client, quota *gardencorev1beta1.Quota) ([]string, error) {
	bindings := &gardencorev1beta1.SecretBindingList{}
	if err := c.List(ctx, bindings); err != nil {
		return nil, err
	}

	var associatedBindings []string
	for _, binding := range bindings.Items {
		for _, quotaRef := range binding.Quotas {
			if quotaRef.Name == quota.Name && quotaRef.Namespace == quota.Namespace {
				associatedBindings = append(associatedBindings, fmt.Sprintf("%s/%s", binding.Namespace, binding.Name))
			}
		}
	}
	return associatedBindings, nil
}

// DetermineBackupBucketAssociations determine the BackupBucket resources which are associated
// to seed with name <seedName>
func DetermineBackupBucketAssociations(ctx context.Context, c client.Client, seedName string) ([]string, error) {
	return determineAssociations(ctx, c, &gardencorev1beta1.BackupBucketList{}, client.MatchingFields{core.BackupBucketSeedName: seedName})
}

// DetermineControllerInstallationAssociations determine the ControllerInstallation resources which are associated
// to seed with name <seedName>
func DetermineControllerInstallationAssociations(ctx context.Context, c client.Client, seedName string) ([]string, error) {
	return determineAssociations(ctx, c, &gardencorev1beta1.ControllerInstallationList{}, client.MatchingFields{core.SeedRefName: seedName})
}

func determineAssociations(ctx context.Context, c client.Client, listObj client.ObjectList, fieldSelector client.MatchingFields) ([]string, error) {
	if err := c.List(ctx, listObj, fieldSelector); err != nil {
		return nil, err
	}

	var associations []string
	err := meta.EachListItem(listObj, func(obj runtime.Object) error {
		accessor, err := meta.Accessor(obj)
		if err != nil {
			return err
		}

		associations = append(associations, accessor.GetName())
		return nil
	})
	return associations, err
}
