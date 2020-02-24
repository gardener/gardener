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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorelisters "github.com/gardener/gardener/pkg/client/core/listers/core/v1beta1"
	"github.com/gardener/gardener/pkg/logger"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// DetermineShootsAssociatedTo gets a <shootLister> to determine the Shoots resources which are associated
// to given <obj> (either a CloudProfile a or a Seed object).
func DetermineShootsAssociatedTo(obj interface{}, shootLister gardencorelisters.ShootLister) ([]string, error) {
	var associatedShoots []string
	shoots, err := shootLister.List(labels.Everything())
	if err != nil {
		logger.Logger.Info(err.Error())
		return nil, err
	}

	for _, shoot := range shoots {
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
		default:
			return nil, fmt.Errorf("unable to determine Shoot associations, due to unknown type %t", t)
		}
	}
	return associatedShoots, nil
}

// DetermineSecretBindingAssociations gets a <bindingLister> to determine the SecretBinding
// resources which are associated to given Quota <obj>.
func DetermineSecretBindingAssociations(quota *gardencorev1beta1.Quota, bindingLister gardencorelisters.SecretBindingLister) ([]string, error) {
	var associatedBindings []string
	bindings, err := bindingLister.List(labels.Everything())
	if err != nil {
		return nil, err
	}

	for _, binding := range bindings {
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
	return determineAssociations(ctx, c, seedName, &gardencorev1beta1.BackupBucketList{}, func(o runtime.Object) (string, error) {
		backupBucket, ok := o.(*gardencorev1beta1.BackupBucket)
		if !ok {
			return "", fmt.Errorf("got unexpected object when expecting BackupBucket")
		}
		if backupBucket.Spec.SeedName == nil {
			return "", nil
		}
		return *backupBucket.Spec.SeedName, nil
	})
}

// DetermineBackupEntryAssociations determine the BackupEntry resources which are associated
// to seed with name <seedName>
func DetermineBackupEntryAssociations(ctx context.Context, c client.Client, seedName string) ([]string, error) {
	return determineAssociations(ctx, c, seedName, &gardencorev1beta1.BackupEntryList{}, func(o runtime.Object) (string, error) {
		backupEntry, ok := o.(*gardencorev1beta1.BackupEntry)
		if !ok {
			return "", fmt.Errorf("got unexpected object when expecting BackupEntry")
		}
		if backupEntry.Spec.SeedName == nil {
			return "", nil
		}
		return *backupEntry.Spec.SeedName, nil
	})
}

// DetermineControllerInstallationAssociations determine the ControllerInstallation resources which are associated
// to seed with name <seedName>
func DetermineControllerInstallationAssociations(ctx context.Context, c client.Client, seedName string) ([]string, error) {
	return determineAssociations(ctx, c, seedName, &gardencorev1beta1.ControllerInstallationList{}, func(o runtime.Object) (string, error) {
		controllerInstallation, ok := o.(*gardencorev1beta1.ControllerInstallation)
		if !ok {
			return "", fmt.Errorf("got unexpected object when expecting ControllerInstallation")
		}
		return controllerInstallation.Spec.SeedRef.Name, nil
	})
}

// DetermineShootAssociations determine the Shoot resources which are associated
// to seed with name <seedName>
func DetermineShootAssociations(ctx context.Context, c client.Client, seedName string) ([]string, error) {
	return determineAssociations(ctx, c, seedName, &gardencorev1beta1.ShootList{}, func(o runtime.Object) (string, error) {
		shoot, ok := o.(*gardencorev1beta1.Shoot)
		if !ok {
			return "", fmt.Errorf("got unexpected object when expecting Shoot")
		}
		if shoot.Spec.SeedName == nil {
			return "", nil
		}
		return *shoot.Spec.SeedName, nil
	})
}

func determineAssociations(ctx context.Context, c client.Client, seedName string, listObj runtime.Object, seedNameFunc func(runtime.Object) (string, error)) ([]string, error) {
	if err := c.List(ctx, listObj); err != nil {
		return nil, err
	}

	var associations []string
	err := meta.EachListItem(listObj, func(obj runtime.Object) error {
		name, err := seedNameFunc(obj)
		if err != nil {
			return err
		}

		if name == seedName {
			accessor, err := meta.Accessor(obj)
			if err != nil {
				return err
			}
			associations = append(associations, accessor.GetName())
		}
		return nil
	})
	return associations, err
}
