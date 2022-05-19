// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package shoot_maintenance_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
)

// waitForShootToBeMaintained uses gomega.Eventually to wait until the maintenance controller has picked up its work
// and removed the operation annotation.
// This is better than wait.Poll* because it respects gomega's environment variables for globally configuring the
// polling intervals and timeouts. This allows to easily make integration tests more robust in CI environments.
// see https://onsi.github.io/gomega/#modifying-default-intervals
// TODO: use this helper in all test cases instead of polling like in the other helper functions.
func waitForShootToBeMaintained(shoot *gardencorev1beta1.Shoot) {
	By("waiting for shoot to be maintained")
	Eventually(func(g Gomega) bool {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		return shoot.ObjectMeta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.ShootOperationMaintain
	}).Should(BeFalse())
}

// WaitForExpectedMachineImageMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's machine image  equals the targetImage and if an image update is required.
func waitForExpectedMachineImageMaintenance(ctx context.Context, logger *logrus.Logger, gardenClient client.Client, s *gardencorev1beta1.Shoot, targetMachineImage gardencorev1beta1.ShootMachineImage, imageUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := gardenClient.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: s.Name}, shoot)
		if err != nil {
			return false, err
		}

		// If one worker of the shoot got an machine image update, we assume the maintenance to having been successful
		// in the integration test we only use one worker pool
		nameVersions := make(map[string]string)
		for _, worker := range shoot.Spec.Provider.Workers {
			if worker.Machine.Image.Version != nil {
				nameVersions[worker.Machine.Image.Name] = *worker.Machine.Image.Version
			}
			if worker.Machine.Image != nil && apiequality.Semantic.DeepEqual(*worker.Machine.Image, targetMachineImage) && imageUpdateRequired {
				logger.Infof("shoot maintained properly - received machine image update")
				return true, nil
			}
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && imageUpdateRequired {
			return false, fmt.Errorf("shoot did not get the expected machine image maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !imageUpdateRequired {
			logger.Infof("shoot maintained properly - did not receive an machineImage update")
			return true, nil
		}
		logger.Infof("shoot %s has workers with machine images (name:version) '%v'. Target image: %s-%s. ImageUpdateRequired: %t. Deadline is in %v", shoot.Name, nameVersions, targetMachineImage.Name, *targetMachineImage.Version, imageUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// WaitForExpectedKubernetesVersionMaintenance polls a shoot until the given deadline is exceeded. Checks if the shoot's kubernetes version equals the targetVersion and if an kubernetes version update is required.
func waitForExpectedKubernetesVersionMaintenance(ctx context.Context, logger *logrus.Logger, gardenClient client.Client, s *gardencorev1beta1.Shoot, targetVersion string, kubernetesVersionUpdateRequired bool, deadline time.Time) error {
	return wait.PollImmediateUntil(2*time.Second, func() (bool, error) {
		shoot := &gardencorev1beta1.Shoot{}
		err := gardenClient.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: s.Name}, shoot)
		if err != nil {
			return false, err
		}

		if shoot.Spec.Kubernetes.Version == targetVersion && kubernetesVersionUpdateRequired {
			logger.Infof("shoot maintained properly - received kubernetes version update")
			return true, nil
		}

		now := time.Now()
		nowIsAfterDeadline := now.After(deadline)
		if nowIsAfterDeadline && kubernetesVersionUpdateRequired {
			return false, fmt.Errorf("shoot did not get the expected kubernetes version maintenance. Deadline exceeded. ")
		} else if nowIsAfterDeadline && !kubernetesVersionUpdateRequired {
			logger.Infof("shoot maintained properly - did not receive an kubernetes version update")
			return true, nil
		}
		logger.Infof("shoot %s has kubernetes version %s. Target version: %s. Kubernetes Version Update Required: %t. Deadline is in %v", shoot.Name, shoot.Spec.Kubernetes.Version, targetVersion, kubernetesVersionUpdateRequired, deadline.Sub(now))
		return false, nil
	}, ctx.Done())
}

// PatchCloudProfileForMachineImageMaintenance patches the images of the Cloud Profile
func patchCloudProfileForMachineImageMaintenance(ctx context.Context, gardenClient client.Client, cloudProfileName string, testMachineImage gardencorev1beta1.ShootMachineImage, expirationDate *metav1.Time, classification *gardencorev1beta1.VersionClassification) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile); err != nil {
		return err
	}
	patch := client.StrategicMergeFrom(cloudProfile.DeepCopy())

	// update Cloud Profile with expirationDate for integration test machine image
	for i, image := range cloudProfile.Spec.MachineImages {
		versionExists, index := helper.ShootMachineImageVersionExists(image, testMachineImage)
		if versionExists {
			cloudProfile.Spec.MachineImages[i].Versions[index].ExpirationDate = expirationDate
			cloudProfile.Spec.MachineImages[i].Versions[index].Classification = classification
		}
	}

	return gardenClient.Patch(ctx, cloudProfile, patch)
}

// PatchCloudProfileForKubernetesVersionMaintenance patches a specific kubernetes version of the Cloud Profile
func patchCloudProfileForKubernetesVersionMaintenance(ctx context.Context, gardenClient client.Client, cloudProfileName string, targetVersion string, expirationDate *metav1.Time, classification *gardencorev1beta1.VersionClassification) error {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := gardenClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile); err != nil {
		return err
	}
	patch := client.StrategicMergeFrom(cloudProfile.DeepCopy())

	// update kubernetes version in cloud profile with an expiration date
	for i, version := range cloudProfile.Spec.Kubernetes.Versions {
		if version.Version == targetVersion {
			cloudProfile.Spec.Kubernetes.Versions[i].Classification = classification
			cloudProfile.Spec.Kubernetes.Versions[i].ExpirationDate = expirationDate
		}
	}

	return gardenClient.Patch(ctx, cloudProfile, patch)
}

// DeleteShoot deletes the given shoot
func deleteShoot(ctx context.Context, gardenClient client.Client, shoot *gardencorev1beta1.Shoot) error {
	err := gutil.ConfirmDeletion(ctx, gardenClient, shoot)
	if err != nil {
		return err
	}
	return client.IgnoreNotFound(gardenClient.Delete(ctx, shoot))
}

func deleteProject(ctx context.Context, gardenClient client.Client, project *gardencorev1beta1.Project) error {
	err := gutil.ConfirmDeletion(ctx, gardenClient, project)
	if err != nil {
		return err
	}
	return client.IgnoreNotFound(gardenClient.Delete(ctx, project))
}
