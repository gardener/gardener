// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldCreateManagedSeed creates the managedseed object
func ItShouldCreateManagedSeed(s *ManagedSeedContext) {
	GinkgoHelper()

	It("Create ManagedSeed", func(ctx SpecContext) {
		s.Log.Info("Creating ManagedSeed")

		Eventually(ctx, func() error {
			if err := s.GardenClient.Create(ctx, s.ManagedSeed); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("managedseed already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForManagedSeedToBeReady waits for the ManagedSeed to be ready
func ItShouldWaitForManagedSeedToBeReady(s *ManagedSeedContext) {
	GinkgoHelper()

	It("Should wait for ManagedSeed to be ready", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.ManagedSeed), s.ManagedSeed)).To(Succeed())
			g.Expect(health.CheckManagedSeed(s.ManagedSeed)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))
}

// ItShouldInitializeSeedContext should get the resulting seed object of the managedseed and initialize the seed context of the ManagedSeedContext
func ItShouldInitializeSeedContext(s *ManagedSeedContext) {
	GinkgoHelper()

	It("Initialize Seed context", func(ctx SpecContext) {
		seed := &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: s.ManagedSeed.Name,
			},
		}

		Eventually(ctx, s.GardenKomega.Get(seed)).Should(Succeed())

		s.WithSeed(seed)
	}, SpecTimeout(time.Minute))
}

// ItShouldAnnotateManagedSeed sets the given annotation within the managedseed metadata to the specified value and patches the managedseed object
func ItShouldAnnotateManagedSeed(s *ManagedSeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate ManagedSeed", func(ctx SpecContext) {
		patch := client.MergeFrom(s.ManagedSeed.DeepCopy())

		for key, value := range annotations {
			s.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&s.ManagedSeed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return s.GardenClient.Patch(ctx, s.ManagedSeed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeleteManagedSeed deletes the managed seed object
func ItShouldDeleteManagedSeed(s *ManagedSeedContext) {
	GinkgoHelper()

	It("Delete ManagedSeed", func(ctx SpecContext) {
		s.Log.Info("Deleting ManagedSeed")

		Eventually(ctx, func() error {
			return s.GardenClient.Delete(ctx, s.ManagedSeed)
		}).Should(Succeed())
	})
}

// ItShouldWaitForManagedSeedToBeDeleted waits for the managedseed object to be gone
func ItShouldWaitForManagedSeedToBeDeleted(s *ManagedSeedContext) {
	GinkgoHelper()

	It("Wait for ManagedSeed to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := s.GardenKomega.Get(s.ManagedSeed)()
			if err == nil {
				s.Log.Info("Waiting for deletion", "status", s.ManagedSeed.Status)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		s.Log.Info("ManagedSeed has been deleted")
	}, SpecTimeout(15*time.Minute))
}
