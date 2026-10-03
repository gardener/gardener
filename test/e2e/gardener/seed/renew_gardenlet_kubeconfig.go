// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Seed Tests", Label("Seed", "default"), func() {
	// This test is Serial because renewing the kubeconfig restarts gardenlet. This interrupts operations of concurrently
	// running shoot specs and blocks new shoot operations until the seed is healthy again. Use Serial with care, as it
	// increases the overall test duration.
	Describe("Renew gardenlet kubeconfig", Ordered, Serial, PriorityFast, func() {
		var (
			tc = NewSeedContext()

			verifier rotation.GardenletKubeconfigRotationVerifier
		)

		BeforeAll(func(ctx SpecContext) {
			tc.Init()

			// Find the first seed which is not "e2e-managedseed". Seed name differs between test scenarios, e.g., non-ha/ha.
			// However, this test should not use "e2e-managedseed", because it is created and deleted in a separate e2e test.
			// This e2e test already includes tests for the "Renew gardenlet kubeconfig" functionality. Additionally,
			// it might be already gone before the kubeconfig was renewed.
			var seed gardencorev1beta1.Seed
			Eventually(ctx, tc.GardenKomega.ObjectList(&gardencorev1beta1.SeedList{})).Should(
				HaveField("Items", ContainElement(HaveField("Name", Not(Equal(DefaultManagedSeedName()))), &seed)),
				"should find an applicable seed",
			)

			tc.SetSeed(&seed)
		}, NodeTimeout(time.Minute))

		ItShouldInitializeSeedClient(tc)

		It("Create gardenlet kubeconfig rotation verifier", func(_ SpecContext) {
			// #nosec: G101 -- This is a secret name reference, not a hardcoded credential.
			verifier = rotation.GardenletKubeconfigRotationVerifier{
				GardenReader:                       tc.GardenClient,
				SeedReader:                         tc.SeedClient,
				Seed:                               tc.Seed,
				GardenletKubeconfigSecretName:      "gardenlet-kubeconfig",
				GardenletKubeconfigSecretNamespace: "garden",
			}
		})

		It("Verify before gardenlet kubeconfig rotation", func(ctx SpecContext) {
			verifier.Before(ctx)
		}, SpecTimeout(time.Minute))

		ItShouldAnnotateSeed(tc, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
		})

		It("Should remove the operation annotation after requesting the gardenlet kubeconfig renewal", func(ctx SpecContext) {
			EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Seed)
		}, SpecTimeout(2*time.Minute))

		It("Verify after gardenlet kubeconfig rotation", func(ctx SpecContext) {
			verifier.After(ctx, false)
		}, SpecTimeout(time.Minute))

		// Restarting gardenlet makes the seed unhealthy until the seed Prometheus has scraped the new gardenlet pod. Gardenlet
		// doesn't start any shoot operation in the meantime, so wait for the seed to be fully ready again before the next
		// specs start.
		ItShouldWaitForSeedToBeReady(tc)
	})
})
