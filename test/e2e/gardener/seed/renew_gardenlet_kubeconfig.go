// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"context"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"

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
			s        *SeedContext
			verifier rotation.GardenletKubeconfigRotationVerifier
		)

		BeforeTestSetup(func() {
			testContext := NewTestContext().Init()

			// Find the first seed which is not "e2e-managedseed". Seed name differs between test scenarios, e.g., non-ha/ha.
			// However, this test should not use "e2e-managedseed", because it is created and deleted in a separate e2e test.
			// This e2e test already includes tests for the "Renew gardenlet kubeconfig" functionality. Additionally,
			// it might be already gone before the kubeconfig was renewed.
			ctx := context.Background()
			seedList := &gardencorev1beta1.SeedList{}
			if err := testContext.GardenClient.List(ctx, seedList); err != nil {
				testContext.Log.Error(err, "Failed to list seeds")
				Fail(err.Error())
			}

			seedIndex := slices.IndexFunc(seedList.Items, func(item gardencorev1beta1.Seed) bool {
				return item.Name != DefaultManagedSeedName()
			})

			if seedIndex == -1 {
				Fail("failed to find applicable seed")
			}

			s = testContext.ForSeed(&seedList.Items[seedIndex])
			ItShouldInitializeSeedClient(s)
		})

		It("Create gardenlet kubeconfig rotation verifier", func(_ SpecContext) {
			// #nosec: G101 -- This is a secret name reference, not a hardcoded credential.
			verifier = rotation.GardenletKubeconfigRotationVerifier{
				GardenReader:                       s.GardenClient,
				SeedReader:                         s.SeedClient,
				Seed:                               s.Seed,
				GardenletKubeconfigSecretName:      "gardenlet-kubeconfig",
				GardenletKubeconfigSecretNamespace: "garden",
			}
		})

		It("Verify before gardenlet kubeconfig rotation", func(ctx SpecContext) {
			verifier.Before(ctx)
		}, SpecTimeout(time.Minute))

		ItShouldAnnotateSeed(s, map[string]string{
			v1beta1constants.GardenerOperation: v1beta1constants.GardenerOperationRenewKubeconfig,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Seed)

		It("Verify after gardenlet kubeconfig rotation", func(ctx SpecContext) {
			verifier.After(ctx, false)
		}, SpecTimeout(time.Minute))

		// Restarting gardenlet makes the seed unhealthy until the seed Prometheus has scraped the new gardenlet pod. Gardenlet
		// doesn't start any shoot operation in the meantime, so wait for the seed to be fully ready again before the next
		// specs start.
		ItShouldWaitForSeedToBeReady(s)
	})
})
