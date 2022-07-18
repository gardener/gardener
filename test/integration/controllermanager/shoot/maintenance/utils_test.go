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

package maintenance_test

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
