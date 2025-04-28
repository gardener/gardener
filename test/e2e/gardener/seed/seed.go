// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldAnnotateSeed sets the given annotation within the seed metadata to the specified value and patches the seed object
func ItShouldAnnotateSeed(s *SeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Seed", func(ctx SpecContext) {
		patch := client.MergeFrom(s.Seed.DeepCopy())

		for key, value := range annotations {
			s.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&s.Seed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return s.GardenClient.Patch(ctx, s.Seed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForSeedToBeReady waits for the seed object to be ready
func ItShouldWaitForSeedToBeReady(s *SeedContext) {
	GinkgoHelper()

	It("Should wait for seed to be ready", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Seed), s.Seed)).To(Succeed())
			g.Expect(health.CheckSeed(s.Seed, s.Seed.Status.Gardener)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(10*time.Minute))
}

// ItShouldWaitForSeedToBeDeleted waits for the seed object to be gone
func ItShouldWaitForSeedToBeDeleted(s *SeedContext) {
	GinkgoHelper()

	It("Wait for Seed to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := s.GardenKomega.Get(s.Seed)()
			if err == nil {
				s.Log.Info("Waiting for deletion", "lastOperation", s.Seed.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		s.Log.Info("Seed has been deleted")
	}, SpecTimeout(10*time.Minute))
}
