// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	shootmigration "github.com/gardener/gardener/test/utils/shoots/migration"
)

var _ = Describe("Shoot Tests", Label("Shoot", "control-plane-migration"), func() {
	test := func(s *ShootContext) {
		// Assign seedName so that shoot does not get scheduled to the seed that will be used as target.
		s.Shoot.Spec.SeedName = ptr.To(getSeedName(false))

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		ItShouldInitializeSeedClient(s)

		if !v1beta1helper.IsWorkerless(s.Shoot) && !v1beta1helper.HibernationIsEnabled(s.Shoot) {
			ItShouldInitializeShootClient(s)
			// We can only verify in-cluster access to the API server before the migration in local e2e tests.
			// After the migration, the shoot API server's hostname still points to the source seed, because
			// the /etc/hosts entry is never updated. Hence, we talk to the API server for starting in-cluster
			// clients. That's also why the ShootMigrationTest is configured to skip all interactions with the
			// shoot API server for local e2e tests.
			inclusterclient.VerifyInClusterAccessToAPIServer(s)
		}

		var (
			seedClientSourceCluster client.Client
			secretsBeforeMigration  map[string]corev1.Secret
		)

		It("Record current seed client", func() {
			seedClientSourceCluster = s.SeedClient
		})

		It("Populate comparison elements before migration", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				secretsBeforeMigration, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		It("Migrate Shoot", func(ctx SpecContext) {
			patch := client.MergeFrom(s.Shoot.DeepCopy())
			s.Shoot.Spec.SeedName = ptr.To(getSeedName(true))
			Eventually(ctx, func() error {
				return s.GardenClient.SubResource("binding").Patch(ctx, s.Shoot, patch)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		ItShouldInitializeSeedClient(s)

		It("Verify that all secrets have been migrated without regeneration", func(ctx SpecContext) {
			var secretsAfterMigration map[string]corev1.Secret
			Eventually(ctx, func() error {
				var err error
				secretsAfterMigration, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
			Expect(shootmigration.ComparePersistedSecrets(secretsBeforeMigration, secretsAfterMigration)).To(Succeed())
		}, SpecTimeout(time.Minute))

		It("Verify that there are no orphaned resources in the source seed", func(ctx SpecContext) {
			Expect(shootmigration.CheckForOrphanedNonNamespacedResources(ctx, s.Shoot.Namespace, seedClientSourceCluster)).To(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	}

	Context("Shoot with workers", Ordered, func() {
		test(NewTestContext().ForShoot(DefaultShoot("e2e-migrate")))
	})

	Context("Workerless Shoot", Label("workerless"), Ordered, func() {
		test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-migrate")))
	})

	Context("Hibernated Shoot", Label("hibernated"), Ordered, func() {
		shoot := DefaultShoot("e2e-mgr-hib")
		shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
			Enabled: ptr.To(true),
		}
		test(NewTestContext().ForShoot(shoot))
	})
})

func getSeedName(isTarget bool) (seedName string) {
	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "node":
		seedName = "local-ha-single-zone"
		if isTarget {
			seedName = "local2-ha-single-zone"
		}
	default:
		seedName = "local"
		if isTarget {
			seedName = "local2"
		}
	}

	return
}
