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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	localv1alpha1 "github.com/gardener/gardener/pkg/provider-local/apis/local/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create, Hibernate, Wake up and Delete Shoot", func() {
		test := func(s *ShootContext) {
			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			// TODO(timebertt): add inclusterclient.VerifyInClusterAccessToAPIServer once it got refactored

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				// We verify the node readiness feature in this specific e2e test because it uses a single-node shoot cluster.
				// The default shoot e2e test deals with multiple nodes, deleting all of them and waiting for them to be recreated
				// might increase the test duration undesirably.
				node.VerifyNodeCriticalComponentsBootstrapping(s)
			}

			It("Hibernate Shoot", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
						Enabled: ptr.To(true),
					}
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			It("Wake up Shoot", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
					s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
						Enabled: ptr.To(false),
					}
				})).Should(Succeed())
			}, SpecTimeout(time.Minute))

			ItShouldWaitForShootToBeReconciledAndHealthy(s)

			// TODO(timebertt): add inclusterclient.VerifyInClusterAccessToAPIServer once it got refactored

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
			var s *ShootContext

			BeforeTestSetup(func() {
				shoot := DefaultShoot("e2e-wake-up-ncp")
				shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "my-profile",
				}

				s = NewTestContext().ForShoot(shoot)
			})

			originalNamespacedCloudProfile := DefaultNamespacedCloudProfile()
			namespacedCloudProfile := addCustomMachineImage(originalNamespacedCloudProfile.DeepCopy())

			BeforeAll(func() {
				DeferCleanup(func(ctx SpecContext) {
					Eventually(ctx, func() error {
						return s.GardenClient.Delete(ctx, namespacedCloudProfile)
					}).Should(Or(Succeed(), BeNotFoundError()))
				}, NodeTimeout(15*time.Minute))
			})

			It("Create NamespacedCloudProfile", func(ctx SpecContext) {
				Eventually(ctx, func(g Gomega) {
					g.Expect(s.GardenClient.Create(ctx, namespacedCloudProfile)).To(Succeed())
				}).Should(Succeed())
			}, SpecTimeout(time.Minute))

			It("Wait for new NamespacedCloudProfile to be reconciled", func(ctx SpecContext) {
				Eventually(ctx, s.GardenKomega.Object(namespacedCloudProfile)).WithPolling(5*time.Second).Should(HaveField(
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
					g.Expect(s.GardenClient.Update(ctx, originalNamespacedCloudProfile)).To(Succeed())
				}).Should(Succeed())

				Eventually(ctx, func(g Gomega) {
					g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(namespacedCloudProfile), namespacedCloudProfile)).To(Succeed())
					g.Expect(namespacedCloudProfile.Generation).To(Equal(namespacedCloudProfile.Status.ObservedGeneration))
					g.Expect(namespacedCloudProfile.Spec.MachineImages).To(Equal(originalNamespacedCloudProfile.Spec.MachineImages))
					g.Expect(namespacedCloudProfile.Spec.ProviderConfig).To(Equal(originalNamespacedCloudProfile.Spec.ProviderConfig))
				}).WithPolling(5 * time.Second).Should(Succeed())
			}, SpecTimeout(time.Minute))

			test(s)
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
