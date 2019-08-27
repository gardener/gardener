// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package framework

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"
	"github.com/gardener/gardener/pkg/apis/garden/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NewShootMaintenanceTest creates a new ShootMaintenanceTest
func NewShootMaintenanceTest(ctx context.Context, shootGardenTest *ShootGardenerTest, shootMachineImageName *string) (*ShootMaintenanceTest, error) {
	cloudProfileForShoot := &gardenv1beta1.CloudProfile{}
	shoot := shootGardenTest.Shoot
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Spec.Cloud.Profile}, cloudProfileForShoot); err != nil {
		return nil, err
	}

	cloudProvider, err := helper.DetermineCloudProviderInShoot(shoot.Spec.Cloud)

	if shootMachineImageName == nil || len(*shootMachineImageName) == 0 {
		shootCurrentMachineImage := helper.GetDefaultMachineImageFromShoot(cloudProvider, shoot)
		if shootCurrentMachineImage == nil {
			return nil, fmt.Errorf("could not determine shoots machine image ")
		}
		shootMachineImageName = &shootCurrentMachineImage.Name
	}

	machineImagesFound, machineImageFromCloudProfile, err := helper.DetermineMachineImageForName(*cloudProfileForShoot, *shootMachineImageName)
	if err != nil || !machineImagesFound {
		return nil, fmt.Errorf("failure while determining the machine images in the CloudProfile: %s", err.Error())
	}

	_, latestMachineImage, err := helper.GetShootMachineImageFromLatestMachineImageVersion(machineImageFromCloudProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image in cloud profile: %s", err.Error())
	}

	return &ShootMaintenanceTest{
		ShootGardenerTest: shootGardenTest,
		CloudProfile:      cloudProfileForShoot,
		CloudProvider:     cloudProvider,
		ShootMachineImage: latestMachineImage,
	}, nil
}

// CreateShoot creates a Shoot Resource
func (s *ShootMaintenanceTest) CreateShoot(ctx context.Context) (*gardenv1beta1.Shoot, error) {
	_, err := s.ShootGardenerTest.GetShoot(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	return s.ShootGardenerTest.CreateShootResource(ctx, s.ShootGardenerTest.Shoot)
}

// CleanupCloudProfile tries to update the CloudProfile with retries to make sure the machine image version & kubernetes version introduced during the integration test is being removed
func (s *ShootMaintenanceTest) CleanupCloudProfile(ctx context.Context, testMachineImage gardenv1beta1.ShootMachineImage, testKubernetesVersions []gardenv1beta1.KubernetesVersion) error {
	var (
		attempt int
	)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		existingCloudProfile := &gardenv1beta1.CloudProfile{}
		if err = s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: s.ShootGardenerTest.Shoot.Spec.Cloud.Profile}, existingCloudProfile); err != nil {
			return err
		}

		// clean machine image
		cloudProfileImages, err := helper.GetMachineImagesFromCloudProfile(existingCloudProfile)
		if err != nil {
			return err
		}
		if cloudProfileImages == nil {
			return fmt.Errorf("cloud profile does not contain any machine images")
		}

		removedCloudProfileImages := []gardenv1beta1.MachineImage{}
		for _, image := range cloudProfileImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				image.Versions = append(image.Versions[:index], image.Versions[index+1:]...)
			}
			removedCloudProfileImages = append(removedCloudProfileImages, image)
		}

		if err := helper.SetMachineImages(existingCloudProfile, removedCloudProfileImages); err != nil {
			return fmt.Errorf("failed to set machine images to cloud profile: %s", err.Error())
		}

		// clean kubernetes cloudprofileVersion
		versions, err := helper.GetKubernetesVersionsFromCloudProfile(*existingCloudProfile)
		if err != nil {
			return err
		}
		if versions == nil {
			return fmt.Errorf("cloud profile does not contain any kubernetes versions")
		}

		removedKubernetesVersions := []gardenv1beta1.KubernetesVersion{}
		for _, cloudprofileVersion := range versions {
			versionShouldBeRemoved := false
			for _, versionToBeRemoved := range testKubernetesVersions {
				if cloudprofileVersion.Version == versionToBeRemoved.Version {
					versionShouldBeRemoved = true
					break
				}
			}
			if !versionShouldBeRemoved {
				removedKubernetesVersions = append(removedKubernetesVersions, cloudprofileVersion)
			}
		}

		if err := helper.SetKubernetesVersions(existingCloudProfile, removedKubernetesVersions, []string{}); err != nil {
			return fmt.Errorf("failed to set kubernetes versions to cloud profile: %s", err.Error())
		}

		// update Cloud Profile to remove the test machine image
		if _, err = s.ShootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(existingCloudProfile); err != nil {
			logger.Logger.Errorf("attempt %d failed to update CloudProfile %s due to %v", attempt, existingCloudProfile.Name, err)
			return err
		}

		return nil
	}); err != nil {
		return err
	}
	return nil
}

// WaitForExpectedMachineImageMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's machine image  equals the targetImage and if an image update is required.
func (s *ShootMaintenanceTest) WaitForExpectedMachineImageMaintenance(ctx context.Context, targetMachineImage gardenv1beta1.ShootMachineImage, cloudProvider gardenv1beta1.CloudProvider, imageUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardenv1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}

		shootMachineImage := helper.GetDefaultMachineImageFromShoot(cloudProvider, shoot)
		if apiequality.Semantic.DeepEqual(*shootMachineImage, targetMachineImage) && imageUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - received machine image update")
			return true, nil
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && imageUpdateRequired {
			return false, fmt.Errorf("Shoot did not get the expected machine image maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !imageUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - did not receive an machineImage update")
			return true, nil
		}
		s.ShootGardenerTest.Logger.Infof("shoot %s has machine version %s-%s. Target image: %s-%s. ImageUpdateRequired: %t. Deadline is in %v", s.ShootGardenerTest.Shoot.Name, shootMachineImage.Name, shootMachineImage.Version, targetMachineImage.Name, targetMachineImage.Version, imageUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// WaitForExpectedKubernetesVersionMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's kubernetes version equals the targetVersion and if an kubernetes version update is required.
func (s *ShootMaintenanceTest) WaitForExpectedKubernetesVersionMaintenance(ctx context.Context, targetVersion string, kubernetesVersionUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardenv1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}

		if shoot.Spec.Kubernetes.Version == targetVersion && kubernetesVersionUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - received kubernetes version update")
			return true, nil
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && kubernetesVersionUpdateRequired {
			return false, fmt.Errorf("Shoot did not get the expected kubernetes version maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !kubernetesVersionUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - did not receive an kubernetes version update")
			return true, nil
		}
		s.ShootGardenerTest.Logger.Infof("shoot %s has kubernetes version %s. Target version: %s. Kubernetes Version Update Required: %t. Deadline is in %v", s.ShootGardenerTest.Shoot.Name, shoot.Spec.Kubernetes.Version, targetVersion, kubernetesVersionUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// TryUpdateShootForMachineImageMaintenance tries to update the maintenance section of the shoot spec regarding the machine image
func (s *ShootMaintenanceTest) TryUpdateShootForMachineImageMaintenance(ctx context.Context, shootToUpdate *gardenv1beta1.Shoot, updateMachineImage bool, update func(*gardenv1beta1.Cloud)) error {
	_, err := kutil.TryUpdateShoot(s.ShootGardenerTest.GardenClient.Garden(), retry.DefaultBackoff, shootToUpdate.ObjectMeta, func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.MachineImageVersion
		shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, shootToUpdate.Annotations)
		if updateMachineImage {
			update(&shoot.Spec.Cloud)
		}
		return shoot, nil
	})
	return err
}

// TryUpdateShootForKubernetesMaintenance tries to update the maintenance section of the shoot spec regarding the Kubernetes version
func (s *ShootMaintenanceTest) TryUpdateShootForKubernetesMaintenance(ctx context.Context, shootToUpdate *gardenv1beta1.Shoot) error {
	_, err := kutil.TryUpdateShoot(s.ShootGardenerTest.GardenClient.Garden(), retry.DefaultBackoff, shootToUpdate.ObjectMeta, func(shoot *gardenv1beta1.Shoot) (*gardenv1beta1.Shoot, error) {
		shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.KubernetesVersion
		shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, shootToUpdate.Annotations)
		return shoot, nil
	})
	return err
}

// TryUpdateCloudProfileForMaintenance tries to update the images of the Cloud Profile
func (s *ShootMaintenanceTest) TryUpdateCloudProfileForMaintenance(ctx context.Context, shoot *gardenv1beta1.Shoot, testMachineImage gardenv1beta1.ShootMachineImage, expirationDate *metav1.Time) error {
	var attempt int
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		attempt++
		cloudProfileForShoot := &gardenv1beta1.CloudProfile{}
		err = s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Spec.Cloud.Profile}, cloudProfileForShoot)
		if err != nil {
			return err
		}

		cloudProfileImages, err := helper.GetMachineImagesFromCloudProfile(cloudProfileForShoot)
		if err != nil {
			return err
		}
		if cloudProfileImages == nil {
			return fmt.Errorf("cloud Profile does not contain any machine images")
		}

		// update Cloud Profile with expirationDate for integration test machine image
		cloudProfileImagesToUpdate := []gardenv1beta1.MachineImage{}
		for _, image := range cloudProfileImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				image.Versions[index].ExpirationDate = expirationDate
			}

			cloudProfileImagesToUpdate = append(cloudProfileImagesToUpdate, image)
		}

		if err := helper.SetMachineImages(cloudProfileForShoot, cloudProfileImagesToUpdate); err != nil {
			return fmt.Errorf("failed to set machine images for cloud provider: %s", err.Error())
		}

		_, err = s.ShootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(cloudProfileForShoot)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update CloudProfile %s due to %v", attempt, cloudProfileForShoot.Name, err)
			return err
		}
		return nil
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated CloudProfile after %d attempts due to %v", attempt, err)
		return err
	}
	return nil
}

// TryUpdateCloudProfileForKubernetesVersionMaintenance tries to update the images of the Cloud Profile
func (s *ShootMaintenanceTest) TryUpdateCloudProfileForKubernetesVersionMaintenance(ctx context.Context, shoot *gardenv1beta1.Shoot, targetVersion string, expirationDateInNearFuture *metav1.Time) error {
	var attempt int

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		attempt++
		cloudProfileForShoot := &gardenv1beta1.CloudProfile{}
		err = s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Spec.Cloud.Profile}, cloudProfileForShoot)
		if err != nil {
			return err
		}

		versions, err := helper.GetKubernetesVersionsFromCloudProfile(*cloudProfileForShoot)
		if err != nil {
			return err
		}

		if versions == nil {
			return fmt.Errorf("cloud profile does not contain any kubernetes versions")
		}

		// update kubernetes version in cloud profile with an expiration date
		cloudProfileKubernetesVersionsToUpdate := []gardenv1beta1.KubernetesVersion{}
		for _, version := range versions {
			if version.Version == targetVersion {
				version.ExpirationDate = expirationDateInNearFuture
			}
			cloudProfileKubernetesVersionsToUpdate = append(cloudProfileKubernetesVersionsToUpdate, version)
		}

		if err := helper.SetKubernetesVersions(cloudProfileForShoot, cloudProfileKubernetesVersionsToUpdate, []string{}); err != nil {
			return fmt.Errorf("failed to set kubernetes versions to cloud profile: %s", err.Error())
		}

		_, err = s.ShootGardenerTest.GardenClient.Garden().GardenV1beta1().CloudProfiles().Update(cloudProfileForShoot)
		if err != nil {
			logger.Logger.Errorf("Attempt %d failed to update CloudProfile %s due to %v", attempt, cloudProfileForShoot.Name, err)
			return err
		}
		return nil
	})
	if err != nil {
		logger.Logger.Errorf("Failed to updated CloudProfile after %d attempts due to %v", attempt, err)
		return err
	}
	return nil
}
