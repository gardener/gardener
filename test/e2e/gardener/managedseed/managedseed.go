// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
)

// ItShouldCreateManagedSeed creates the ManagedSeed object
func ItShouldCreateManagedSeed(tc *ManagedSeedContext) {
	GinkgoHelper()

	It("Create ManagedSeed", func(ctx SpecContext) {
		tc.Log.Info("Creating ManagedSeed")

		Eventually(ctx, func() error {
			if err := tc.GardenClient.Create(ctx, tc.ManagedSeed); !apierrors.IsAlreadyExists(err) {
				return err
			}
			return StopTrying("ManagedSeed already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForManagedSeedToBeReady waits for the ManagedSeed to be ready
func ItShouldWaitForManagedSeedToBeReady(tc *ManagedSeedContext) {
	GinkgoHelper()

	It("Should wait for ManagedSeed to be ready", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.ManagedSeed), tc.ManagedSeed)).To(Succeed())
			g.Expect(health.CheckManagedSeed(tc.ManagedSeed)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(10*time.Minute))
}

// ItShouldAnnotateManagedSeed sets the given annotation within the managedseed metadata to the specified value and patches the managedseed object
func ItShouldAnnotateManagedSeed(tc *ManagedSeedContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate ManagedSeed", func(ctx SpecContext) {
		patch := client.MergeFrom(tc.ManagedSeed.DeepCopy())

		for key, value := range annotations {
			tc.Log.Info("Setting annotation", "annotation", key, "value", value)
			metav1.SetMetaDataAnnotation(&tc.ManagedSeed.ObjectMeta, key, value)
		}

		Eventually(ctx, func() error {
			return tc.GardenClient.Patch(ctx, tc.ManagedSeed, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeleteManagedSeed deletes the managed seed object
func ItShouldDeleteManagedSeed(tc *ManagedSeedContext) {
	GinkgoHelper()

	It("Delete ManagedSeed", func(ctx SpecContext) {
		tc.Log.Info("Deleting ManagedSeed")

		Eventually(ctx, func() error {
			return tc.GardenClient.Delete(ctx, tc.ManagedSeed)
		}).Should(Succeed())
	})
}

// ItShouldWaitForManagedSeedToBeDeleted waits for the managedseed object to be gone
func ItShouldWaitForManagedSeedToBeDeleted(tc *ManagedSeedContext) {
	GinkgoHelper()

	It("Wait for ManagedSeed to be deleted", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			err := tc.GardenKomega.Get(tc.ManagedSeed)()
			if err == nil {
				tc.Log.Info("Waiting for deletion", "status", tc.ManagedSeed.Status)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		tc.Log.Info("ManagedSeed has been deleted")
	}, SpecTimeout(15*time.Minute))
}
