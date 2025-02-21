// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/extensions/pkg/util"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	api "github.com/gardener/gardener/pkg/provider-local/apis/local"
	"github.com/gardener/gardener/pkg/provider-local/apis/local/install"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Hibernate, Wake up and Delete Shoot", func() {
		test := func(s *ShootContext) {
			if s.Shoot.Spec.CloudProfileName == nil && s.Shoot.Spec.CloudProfile != nil && s.Shoot.Spec.CloudProfile.Kind == "NamespacedCloudProfile" {
				originalNamespacedCloudProfile := DefaultNamespacedCloudProfile()
				namespacedCloudProfile := addCustomMachineImage(originalNamespacedCloudProfile.DeepCopy())

				BeforeAll(func() {
					DeferCleanup(func(ctx SpecContext) {
						Eventually(func(g Gomega) {
							g.Expect(s.GardenClient.Delete(ctx, namespacedCloudProfile)).To(Or(Succeed(), BeNotFoundError()))
						}).Should(Succeed())
					}, NodeTimeout(15*time.Minute))
				})

				It("Create NamespacedCloudProfile", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						g.Expect(s.GardenClient.Create(ctx, namespacedCloudProfile)).To(Succeed())
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				It("Wait for new NamespacedCloudProfile to be reconciled", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
						g.Expect(namespacedCloudProfile.Generation).To(Equal(namespacedCloudProfile.Status.ObservedGeneration),
							"NamespacedCloudProfile status has been reconciled")
					}).WithPolling(5 * time.Second).Should(Succeed())
				}, SpecTimeout(time.Minute))

				It("Check for correct mutation of the status provider config", func() {
					utilruntime.Must(install.AddToScheme(s.GardenClient.Scheme()))
					decoder := serializer.NewCodecFactory(s.GardenClient.Scheme(), serializer.EnableStrict).UniversalDecoder()
					cloudProfileConfig := &api.CloudProfileConfig{}
					Expect(namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig).To(Not(BeNil()))
					Expect(util.Decode(decoder, namespacedCloudProfile.Status.CloudProfileSpec.ProviderConfig.Raw, cloudProfileConfig)).To(Succeed())
					Expect(cloudProfileConfig.MachineImages).To(ContainElement(MatchFields(IgnoreExtras, Fields{
						"Name": Equal("nscpfl-machine-image-1"),
						"Versions": ContainElements(
							api.MachineImageVersion{Version: "1.1", Image: "local/image:1.1"},
						),
					})))
				})

				It("Remove custom machine image again", func(ctx SpecContext) {
					Eventually(func(g Gomega) {
						patch := client.MergeFrom(namespacedCloudProfile)
						g.Expect(s.GardenClient.Patch(ctx, originalNamespacedCloudProfile, patch)).To(Succeed())
					}).Should(Succeed())

					Eventually(func(g Gomega) {
						g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
						g.Expect(namespacedCloudProfile.Generation).To(Equal(namespacedCloudProfile.Status.ObservedGeneration))
						g.Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal(originalNamespacedCloudProfile.Spec.MachineImages))
						g.Expect(namespacedCloudProfile.Spec.ProviderConfig).To(Equal(originalNamespacedCloudProfile.Spec.ProviderConfig))
					}).WithPolling(5 * time.Second).Should(Succeed())
				}, SpecTimeout(time.Minute))
			}

			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			//TODO: add inclusterclient.VerifyInClusterAccessToAPIServer once it got refactored

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				node.VerifyNodeCriticalComponentsBootstrapping(s)
			}

			It("Hibernate Shoot", func(ctx SpecContext) {
				Eventually(func() error {
					patch := client.MergeFrom(s.Shoot.DeepCopy())
					s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
						Enabled: ptr.To(true),
					}

					return s.GardenClient.Patch(ctx, s.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			It("Wake up Shoot", func(ctx SpecContext) {
				Eventually(func() error {
					patch := client.MergeFrom(s.Shoot.DeepCopy())
					s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
						Enabled: ptr.To(false),
					}

					return s.GardenClient.Patch(ctx, s.Shoot, patch)
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			//TODO: add inclusterclient.VerifyInClusterAccessToAPIServer once it got refactored

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Label("basic"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-wake-up")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-wake-up")))
		})

		Context("Shoot with workers with NamespacedCloudProfile", Label("basic"), Ordered, func() {
			shoot := DefaultShoot("e2e-wake-up-ncp")
			shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
				Kind: "NamespacedCloudProfile",
				Name: "my-profile",
			}
			test(NewTestContext().ForShoot(shoot))
		})
	})
})

func addCustomMachineImage(namespacedCloudProfile *gardencorev1beta1.NamespacedCloudProfile) *gardencorev1beta1.NamespacedCloudProfile {
	namespacedCloudProfile.Spec.MachineImages = []gardencorev1beta1.MachineImage{
		{
			Name:           "nscpfl-machine-image-1",
			UpdateStrategy: ptr.To(gardencorev1beta1.UpdateStrategyMinor),
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
