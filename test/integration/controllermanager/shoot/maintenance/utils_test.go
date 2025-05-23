// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package maintenance_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
)

// waitForShootToBeMaintained uses gomega.Eventually to wait until the maintenance controller has picked up its work
// and removed the operation annotation.
// This is better than wait.Poll* because it respects gomega's environment variables for globally configuring the
// polling intervals and timeouts. This allows to easily make integration tests more robust in CI environments.
// see https://onsi.github.io/gomega/#modifying-default-intervals
func waitForShootToBeMaintained(shoot *gardencorev1beta1.Shoot) {
	By("Wait for shoot to be maintained")
	EventuallyWithOffset(1, func(g Gomega) bool {
		g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		return shoot.ObjectMeta.Annotations[v1beta1constants.GardenerOperation] == v1beta1constants.ShootOperationMaintain
	}).Should(BeFalse())
}

func waitMachineImageVersionToBeExpiredInCloudProfile(cloudProfileName, imageName, imageVersion string, expirationDate *metav1.Time) {
	EventuallyWithOffset(1, func(g Gomega) {
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		g.Expect(mgrClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile)).To(Succeed())

		machineImageVersion, ok := v1beta1helper.FindMachineImageVersion(cloudProfile.Spec.MachineImages, imageName, imageVersion)
		g.Expect(ok).To(BeTrue())
		g.Expect(machineImageVersion.Classification).To(PointTo(Equal(gardencorev1beta1.ClassificationDeprecated)))
		g.Expect(machineImageVersion.ExpirationDate).NotTo(BeNil())
		g.Expect(machineImageVersion.ExpirationDate.UTC()).To(Equal(expirationDate.UTC()))
	}).Should(Succeed())
}

func waitKubernetesVersionToBeExpiredInCloudProfile(cloudProfileName, k8sVersion string, expirationDate *metav1.Time) {
	EventuallyWithOffset(1, func(g Gomega) {
		cloudProfile := &gardencorev1beta1.CloudProfile{}
		g.Expect(mgrClient.Get(ctx, client.ObjectKey{Name: cloudProfileName}, cloudProfile)).To(Succeed())

		found, k8sVersion, err := v1beta1helper.KubernetesVersionExistsInCloudProfile(cloudProfile, k8sVersion)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(k8sVersion.Classification).To(PointTo(Equal(gardencorev1beta1.ClassificationDeprecated)))
		g.Expect(k8sVersion.ExpirationDate).NotTo(BeNil())
		g.Expect(k8sVersion.ExpirationDate.UTC()).To(Equal(expirationDate.UTC()))
	}).Should(Succeed())
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
		versionExists, index := v1beta1helper.ShootMachineImageVersionExists(image, testMachineImage)
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
