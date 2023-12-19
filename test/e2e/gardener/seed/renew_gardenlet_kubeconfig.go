// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	Describe("Renew gardenlet kubeconfig", func() {
		var seed *gardencorev1beta1.Seed

		BeforeEach(func() {
			// find the first seed, it doesn't matter which one (seed name differs between test scenarios, e.g., non-ha/ha)
			seedList := &gardencorev1beta1.SeedList{}
			Expect(testClient.List(ctx, seedList, client.Limit(1))).To(Succeed())
			seed = seedList.Items[0].DeepCopy()
			log.Info("Renewing gardenlet kubeconfig", "seedName", seed.Name)
		})

		It("should renew the gardenlet garden kubeconfig when triggered by annotation", func() {
			verifier := rotation.GardenletKubeconfigRotationVerifier{
				GardenReader:                       testClient,
				SeedReader:                         testClient,
				Seed:                               seed,
				GardenletKubeconfigSecretName:      "gardenlet-kubeconfig",
				GardenletKubeconfigSecretNamespace: "garden",
			}

			By("Verify before state")
			verifier.Before(ctx)

			By("Trigger renewal of gardenlet garden kubeconfig")
			patch := client.MergeFrom(seed.DeepCopy())
			metav1.SetMetaDataAnnotation(&seed.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.GardenerOperationRenewKubeconfig)
			Eventually(func() error {
				return testClient.Patch(ctx, seed, patch)
			}).Should(Succeed())

			By("Wait for operation annotation to be removed from Seed")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				g.Expect(seed.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			}).Should(Succeed())

			By("Verify result")
			verifier.After(ctx, false)
		})
	})
})
