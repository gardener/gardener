// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/apiserveravailability"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	shootmigration "github.com/gardener/gardener/test/utils/shoots/migration"
)

var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-migration"), func() {
	test := func(tc *ShootContext, liveMigrate bool) {
		// Assign seedName so that shoot does not get scheduled to the seed that will be used as target.
		tc.Shoot.Spec.SeedName = new(getSeedName(false))

		BeforeAll(func() {
			tc.Init()
		})

		ItShouldCreateShoot(tc)
		ItShouldWaitForShootToBeReconciledAndHealthy(tc)
		ItShouldGetResponsibleSeed(tc)
		seed.ItShouldInitializeSeedClient(tc.SeedContext)

		// The shoot client (admin kubeconfig) is needed for the in-cluster access check (worker shoots) and for the API
		// server availability probe during live migration (both worker and workerless shoots).
		if liveMigrate || (!v1beta1helper.IsWorkerless(tc.Shoot) && !v1beta1helper.HibernationIsEnabled(tc.Shoot)) {
			ItShouldInitializeShootClient(tc)
		}
		if !v1beta1helper.IsWorkerless(tc.Shoot) && !v1beta1helper.HibernationIsEnabled(tc.Shoot) {
			inclusterclient.VerifyInClusterAccessToAPIServer(tc)
		}

		var (
			seedClientSourceCluster client.Client
			secretsBeforeMigration  map[string]corev1.Secret
			availabilityProbe       = &apiserveravailability.Probe{}
		)

		It("Record current seed client", func() {
			seedClientSourceCluster = tc.SeedClient
		})

		machinePodNamesBeforeTest := ItShouldFindAllMachinePodsBefore(tc, func() client.Client { return seedClientSourceCluster })

		It("Populate comparison elements before migration", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				secretsBeforeMigration, err = shootmigration.GetPersistedSecrets(ctx, tc.SeedClientSet.Client(), tc.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		if liveMigrate {
			availabilityProbe.ItShouldStartProbe(tc)
			ItShouldAnnotateShoot(tc, map[string]string{v1beta1constants.AnnotationMigrationLiveMigrate: "true"})
		}

		It("Migrate Shoot", func(ctx SpecContext) {
			patch := client.MergeFrom(tc.Shoot.DeepCopy())
			tc.Shoot.Spec.SeedName = new(getSeedName(true))
			Eventually(ctx, func() error {
				return tc.GardenClient.SubResource("binding").Patch(ctx, tc.Shoot, patch)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(tc)
		ItShouldGetResponsibleSeed(tc)
		seed.ItShouldInitializeSeedClient(tc.SeedContext)

		if liveMigrate {
			availabilityProbe.ItShouldStopProbeAndEnsureNoDowntime(tc)
		}

		It("Verify that all secrets have been migrated without regeneration", func(ctx SpecContext) {
			var secretsAfterMigration map[string]corev1.Secret
			Eventually(ctx, func() error {
				var err error
				secretsAfterMigration, err = shootmigration.GetPersistedSecrets(ctx, tc.SeedClientSet.Client(), tc.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
			Expect(shootmigration.ComparePersistedSecrets(secretsBeforeMigration, secretsAfterMigration)).To(Succeed())
		}, SpecTimeout(time.Minute))

		It("Verify that there are no orphaned resources in the source seed", func(ctx SpecContext) {
			Expect(shootmigration.CheckForOrphanedNonNamespacedResources(ctx, tc.Shoot.Namespace, seedClientSourceCluster)).To(Succeed())
		}, SpecTimeout(time.Minute))

		// the "infrastructure" (aka the machine pods), still exist in the "kind-gardener-local" cluster
		ItShouldCompareMachinePodNamesAfter(tc, func() client.Client { return seedClientSourceCluster }, machinePodNamesBeforeTest)

		if !v1beta1helper.IsWorkerless(tc.Shoot) && !v1beta1helper.HibernationIsEnabled(tc.Shoot) {
			ItShouldInitializeShootClient(tc)
			inclusterclient.VerifyInClusterAccessToAPIServer(tc)
		}

		ItShouldDeleteShoot(tc)
		ItShouldWaitForShootToBeDeleted(tc)
	}

	Context("Shoot with workers", Ordered, PriorityLonger, func() {
		test(NewShootContext(DefaultShoot("e2e-migrate")), false)
	})

	Context("Workerless Shoot", Label("workerless"), Ordered, PriorityLong, func() {
		test(NewShootContext(DefaultWorkerlessShoot("e2e-migrate")), false)
	})

	Context("Hibernated Shoot", Label("hibernated"), Ordered, PriorityLong, func() {
		shoot := DefaultShoot("e2e-mgr-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: new(true),
		}
		test(NewShootContext(shoot), false)
	})

	Context("Live Migration Shoot", Label("live-migration"), Ordered, PriorityLong, func() {
		test(NewShootContext(DefaultShoot("e2e-live-mgr")), true)
	})

	Context("Live Migration Workerless Shoot", Label("workerless", "live-migration"), Ordered, PriorityLong, func() {
		test(NewShootContext(DefaultWorkerlessShoot("e2e-live-mgr")), true)
	})
})

func getSeedName(isTarget bool) (seedName string) {
	seedName = "local"
	if isTarget {
		seedName = "local2"
	}
	return
}
