// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package project

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

var _ = Describe("Project Tests", Label("Project", "default"), func() {
	Describe("NamespacedCloudProfile", Ordered, func() {
		tc := NewTestContext()

		originalNamespacedCloudProfile := DefaultNamespacedCloudProfile()
		namespacedCloudProfile := addCustomMachineImage(originalNamespacedCloudProfile.DeepCopy())

		BeforeAll(func() {
			DeferCleanup(func(ctx SpecContext) {
				Eventually(ctx, func() error {
					return tc.GardenClient.Delete(ctx, namespacedCloudProfile)
				}).Should(Or(Succeed(), BeNotFoundError()))
			}, NodeTimeout(15*time.Minute))
		})

		It("Create NamespacedCloudProfile", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Create(ctx, namespacedCloudProfile)).To(Succeed())
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Wait for new NamespacedCloudProfile to be reconciled", func(ctx SpecContext) {
			Eventually(ctx, tc.GardenKomega.Object(namespacedCloudProfile)).WithPolling(5*time.Second).Should(HaveField(
				"Status.ObservedGeneration", Equal(namespacedCloudProfile.Generation),
			), "NamespacedCloudProfile status has been reconciled")
		}, SpecTimeout(time.Minute))

		It("Check for correct mutation of the status provider config", func() {
			Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig).NotTo(BeNil())

			scheme := runtime.NewScheme()
			Expect(localv1alpha1.AddToScheme(scheme)).To(Succeed())
			decoder := serializer.NewCodecFactory(scheme, serializer.EnableStrict).UniversalDecoder()

			cloudProfileConfig := &localv1alpha1.CloudProfileConfig{}
			Expect(runtime.DecodeInto(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw, cloudProfileConfig)).To(Succeed())

			Expect(cloudProfileConfig.MachineImages).To(ContainElement(MatchFields(IgnoreExtras, Fields{
				"Name": Equal("nscpfl-machine-image-1"),
				"Versions": ContainElements(
					localv1alpha1.MachineImageVersion{Version: "1.1", Image: "local/image:1.1"},
				),
			})))
		})

		It("Remove custom machine image again", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Update(ctx, originalNamespacedCloudProfile)).To(Succeed())
			}).Should(Succeed())

			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
				g.Expect(namespacedCloudProfile.Generation).To(Equal(namespacedCloudProfile.Status.ObservedGeneration))
				g.Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal(originalNamespacedCloudProfile.Spec.MachineImages))
				g.Expect(namespacedCloudProfile.Spec.ProviderConfig).To(Equal(originalNamespacedCloudProfile.Spec.ProviderConfig))
			}).WithPolling(5 * time.Second).Should(Succeed())
		}, SpecTimeout(time.Minute))
	})
})

func addCustomMachineImage(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) *gardencorev1beta1.NamespacedCloudProfile {
	namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
		{
			Name:           "nscpfl-machine-image-1",
			UpdateStrategy: new(gardencorev1beta1.UpdateStrategyMinor),
			Versions: []gardencorev1beta1.MachineImageVersion{
				{ExpirableVersion: gardencorev1beta1.ExpirableVersion{Version: "1.1"}, Architectures: []string{"amd64"}, CRI: []gardencorev1beta1.CRI{{Name: "containerd"}}},
			},
		},
	}
	namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{
		Raw: []byte(`{
			"apiVersion":"local.provider.extensions.gardener.cloud/v1alpha1",
			"kind":"CloudProfileConfig",
			"machineImages":[
			 {"name":"nscpfl-machine-image-1","versions":[{"version":"1.1","image":"local/image:1.1"}]}
			]}`),
	}
	return namespacedCloudProfile
}
