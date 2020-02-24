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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/logger"
	"github.com/gardener/gardener/pkg/utils"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// NewShootMaintenanceTest creates a new ShootMaintenanceTest
func NewShootMaintenanceTest(ctx context.Context, shootGardenTest *ShootGardenerTest, shootMachineImageName *string) (*ShootMaintenanceTest, error) {
	cloudProfileForShoot := &gardencorev1beta1.CloudProfile{}
	shoot := shootGardenTest.Shoot
	if err := shootGardenTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: shoot.Spec.CloudProfileName}, cloudProfileForShoot); err != nil {
		return nil, err
	}

	// Get a ShootMachineImage with the latest version of the given machine image name from the CloudProfile
	latestMachineImage, err := getLatestShootMachineImagePossible(shootMachineImageName, *cloudProfileForShoot)
	if err != nil {
		return nil, err
	}

	return &ShootMaintenanceTest{
		ShootGardenerTest: shootGardenTest,
		CloudProfile:      cloudProfileForShoot,
		ShootMachineImage: *latestMachineImage,
	}, nil
}

func getLatestShootMachineImagePossible(shootMachineImageName *string, profile gardencorev1beta1.CloudProfile) (*gardencorev1beta1.ShootMachineImage, error) {
	if shootMachineImageName == nil || len(*shootMachineImageName) == 0 {
		shootCurrentMachineImage := helper.GetDefaultMachineImageFromCloudProfile(profile)
		if shootCurrentMachineImage == nil {
			return nil, fmt.Errorf("could not get a default machine image from the CloudProfile")
		}
		shootMachineImageName = &shootCurrentMachineImage.Name
	}

	// Get the machine image from the CloudProfile
	machineImagesFound, machineImageFromCloudProfile, err := helper.DetermineMachineImageForName(&profile, *shootMachineImageName)
	if err != nil || !machineImagesFound {
		return nil, fmt.Errorf("failure while determining the machine images in the CloudProfile: %s", err.Error())
	}

	// Determine the latest version of the shoots image.
	_, latestMachineImage, err := helper.GetShootMachineImageFromLatestMachineImageVersion(machineImageFromCloudProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to determine latest machine image in cloud profile: %s", err.Error())
	}
	return &latestMachineImage, nil
}

// CreateShoot creates a Shoot Resource
func (s *ShootMaintenanceTest) CreateShoot(ctx context.Context) (*gardencorev1beta1.Shoot, error) {
	_, err := s.ShootGardenerTest.GetShoot(ctx)
	if !apierrors.IsNotFound(err) {
		return nil, err
	}
	return s.ShootGardenerTest.CreateShootResource(ctx, s.ShootGardenerTest.Shoot)
}

// CleanupCloudProfile tries to update the CloudProfile with retries to make sure the machine image version & kubernetes version introduced during the integration test is being removed
func (s *ShootMaintenanceTest) CleanupCloudProfile(ctx context.Context, testMachineImage gardencorev1beta1.ShootMachineImage, testKubernetesVersions []gardencorev1beta1.ExpirableVersion) error {
	var (
		attempt int
	)
	if err := retry.RetryOnConflict(retry.DefaultBackoff, func() (err error) {
		existingCloudProfile := &gardencorev1beta1.CloudProfile{}
		if err = s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Name: s.ShootGardenerTest.Shoot.Spec.CloudProfileName}, existingCloudProfile); err != nil {
			return err
		}

		// clean machine image
		removedCloudProfileImages := []gardencorev1beta1.MachineImage{}
		for _, image := range existingCloudProfile.Spec.MachineImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				image.Versions = append(image.Versions[:index], image.Versions[index+1:]...)
			}
			removedCloudProfileImages = append(removedCloudProfileImages, image)
		}
		existingCloudProfile.Spec.MachineImages = removedCloudProfileImages

		// clean kubernetes CloudProfile Version
		removedKubernetesVersions := []gardencorev1beta1.ExpirableVersion{}
		for _, cloudprofileVersion := range existingCloudProfile.Spec.Kubernetes.Versions {
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
		existingCloudProfile.Spec.Kubernetes.Versions = removedKubernetesVersions

		// update Cloud Profile to remove the test machine image
		if _, err = s.ShootGardenerTest.GardenClient.GardenCore().CoreV1beta1().CloudProfiles().Update(existingCloudProfile); err != nil {
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
func (s *ShootMaintenanceTest) WaitForExpectedMachineImageMaintenance(ctx context.Context, targetMachineImage gardencorev1beta1.ShootMachineImage, imageUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := s.ShootGardenerTest.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: s.ShootGardenerTest.Shoot.Namespace, Name: s.ShootGardenerTest.Shoot.Name}, shoot)
		if err != nil {
			return false, err
		}

		// If one worker of the shoot got an machine image update, we assume the maintenance to having been successful
		// in the integration test we only use one worker pool
		nameVersions := make(map[string]string)
		for _, worker := range shoot.Spec.Provider.Workers {
			nameVersions[worker.Machine.Image.Name] = worker.Machine.Image.Version
			if worker.Machine.Image != nil && apiequality.Semantic.DeepEqual(*worker.Machine.Image, targetMachineImage) && imageUpdateRequired {
				s.ShootGardenerTest.Logger.Infof("shoot maintained properly - received machine image update")
				return true, nil
			}
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && imageUpdateRequired {
			return false, fmt.Errorf("shoot did not get the expected machine image maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !imageUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - did not receive an machineImage update")
			return true, nil
		}
		s.ShootGardenerTest.Logger.Infof("shoot %s has workers with machine images (name:version) '%v'. Target image: %s-%s. ImageUpdateRequired: %t. Deadline is in %v", s.ShootGardenerTest.Shoot.Name, nameVersions, targetMachineImage.Name, targetMachineImage.Version, imageUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// WaitForExpectedKubernetesVersionMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's kubernetes version equals the targetVersion and if an kubernetes version update is required.
func (s *ShootMaintenanceTest) WaitForExpectedKubernetesVersionMaintenance(ctx context.Context, targetVersion string, kubernetesVersionUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
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
			return false, fmt.Errorf("shoot did not get the expected kubernetes version maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !kubernetesVersionUpdateRequired {
			s.ShootGardenerTest.Logger.Infof("shoot maintained properly - did not receive an kubernetes version update")
			return true, nil
		}
		s.ShootGardenerTest.Logger.Infof("shoot %s has kubernetes version %s. Target version: %s. Kubernetes Version Update Required: %t. Deadline is in %v", s.ShootGardenerTest.Shoot.Name, shoot.Spec.Kubernetes.Version, targetVersion, kubernetesVersionUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// TryUpdateShootForMachineImageMaintenance tries to update the maintenance section of the shoot spec regarding the machine image
func (s *ShootMaintenanceTest) TryUpdateShootForMachineImageMaintenance(ctx context.Context, shootToUpdate *gardencorev1beta1.Shoot, updateMachineImage bool, workers []gardencorev1beta1.Worker) error {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: shootToUpdate.ObjectMeta}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, s.ShootGardenerTest.GardenClient.Client(), shoot, func() error {
		if shootToUpdate.Spec.Maintenance.AutoUpdate != nil {
			shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.MachineImageVersion
		}
		shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, shootToUpdate.Annotations)
		if updateMachineImage {
			shoot.Spec.Provider.Workers = workers
		}
		return nil
	})
}

// TryUpdateShootForKubernetesMaintenance tries to update the maintenance section of the shoot spec regarding the Kubernetes version
func (s *ShootMaintenanceTest) TryUpdateShootForKubernetesMaintenance(ctx context.Context, shootToUpdate *gardencorev1beta1.Shoot) error {
	shoot := &gardencorev1beta1.Shoot{ObjectMeta: shootToUpdate.ObjectMeta}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, s.ShootGardenerTest.GardenClient.Client(), shoot, func() error {
		shoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = shootToUpdate.Spec.Maintenance.AutoUpdate.KubernetesVersion
		shoot.Annotations = utils.MergeStringMaps(shoot.Annotations, shootToUpdate.Annotations)
		return nil
	})
}

// TryUpdateCloudProfileForMaintenance tries to update the images of the Cloud Profile
func (s *ShootMaintenanceTest) TryUpdateCloudProfileForMaintenance(ctx context.Context, shoot *gardencorev1beta1.Shoot, testMachineImage gardencorev1beta1.ShootMachineImage, expirationDate *metav1.Time) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: shoot.Spec.CloudProfileName,
		},
	}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, s.ShootGardenerTest.GardenClient.Client(), cloudProfile, func() error {
		// update Cloud Profile with expirationDate for integration test machine image
		cloudProfileImagesToUpdate := []gardencorev1beta1.MachineImage{}
		for _, image := range cloudProfile.Spec.MachineImages {
			versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
			if versionExists {
				image.Versions[index].ExpirationDate = expirationDate
			}
			cloudProfileImagesToUpdate = append(cloudProfileImagesToUpdate, image)
		}
		cloudProfile.Spec.MachineImages = cloudProfileImagesToUpdate
		return nil
	})
}

// TryUpdateCloudProfileForKubernetesVersionMaintenance tries to update the images of the Cloud Profile
func (s *ShootMaintenanceTest) TryUpdateCloudProfileForKubernetesVersionMaintenance(ctx context.Context, shoot *gardencorev1beta1.Shoot, targetVersion string, expirationDateInNearFuture *metav1.Time) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: shoot.Spec.CloudProfileName,
		},
	}

	return kutil.TryUpdate(ctx, retry.DefaultBackoff, s.ShootGardenerTest.GardenClient.Client(), cloudProfile, func() error {
		// update kubernetes version in cloud profile with an expiration date
		cloudProfileKubernetesVersionsToUpdate := []gardencorev1beta1.ExpirableVersion{}
		for _, version := range cloudProfile.Spec.Kubernetes.Versions {
			if version.Version == targetVersion {
				version.ExpirationDate = expirationDateInNearFuture
			}
			cloudProfileKubernetesVersionsToUpdate = append(cloudProfileKubernetesVersionsToUpdate, version)
		}
		cloudProfile.Spec.Kubernetes.Versions = cloudProfileKubernetesVersionsToUpdate
		return nil
	})
}
