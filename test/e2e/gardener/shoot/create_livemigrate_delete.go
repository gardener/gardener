// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

var _ = Describe("Shoot Tests", Label("Shoot", "live-control-plane-migration"), func() {
	test := func(s *ShootContext) {
		// Live control plane migration is supported for both shoots with workers and workerless shoots. The worker-only
		// steps (in-cluster access, machine pod comparison) are skipped for workerless shoots.
		workerless := v1beta1helper.IsWorkerless(s.Shoot)

		// A live control plane migration requires a highly-available control plane. Configure HA before the shoot is
		// created so that it is HA already when the migration is triggered. The failure tolerance type is taken from the
		// SHOOT_FAILURE_TOLERANCE_TYPE env variable and defaults to 'node' for the local setup.
		failureToleranceType := GetFailureToleranceType()
		if failureToleranceType == "" {
			failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
		}
		s.Shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{
			HighAvailability: &gardencorev1beta1.HighAvailability{
				FailureTolerance: gardencorev1beta1.FailureTolerance{
					Type: failureToleranceType,
				},
			},
		}

		// Assign seedName so that the shoot does not get scheduled to the seed that will be used as target.
		s.Shoot.Spec.SeedName = new(getSeedName(false))

		BeforeAll(func() {
			s.Init()
		})

		ItShouldCreateShoot(s)
		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		seed.ItShouldInitializeSeedClient(s.SeedContext)

		// The shoot client (admin kubeconfig) is required by the API server availability probe below for both worker and
		// workerless shoots. The in-cluster access verification additionally needs worker nodes, so it is worker-only.
		ItShouldInitializeShootClient(s)
		if !workerless {
			inclusterclient.VerifyInClusterAccessToAPIServer(s)
		}

		var (
			seedClientSourceCluster client.Client
			secretsBeforeMigration  map[string]corev1.Secret
			availabilityProbe       = &apiserveravailability.Probe{}

			machinePodNamesBeforeTest = sets.New[string]()
		)

		It("Record current seed client", func() {
			seedClientSourceCluster = s.SeedClient
		})

		if !workerless {
			machinePodNamesBeforeTest = ItShouldFindAllMachinePodsBefore(s, func() client.Client { return seedClientSourceCluster })
		}

		It("Populate comparison elements before migration", func(ctx SpecContext) {
			Eventually(ctx, func() error {
				var err error
				secretsBeforeMigration, err = shootmigration.GetPersistedSecrets(ctx, s.SeedClientSet.Client(), s.Shoot.Status.TechnicalID)
				return err
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		// Continuously probe the shoot API server's external endpoint across the whole migration to assert that the live
		// migration does not cause API server downtime during the etcd and DNS cutover. This is the central guarantee of
		// GEP-39 (zero kube-apiserver downtime) and is asserted for both worker and workerless shoots.
		availabilityProbe.ItShouldStartProbe(s)

		// A live control plane migration is triggered by setting the live-migrate intent annotation on the shoot and
		// changing spec.seedName to the destination seed via the binding subresource.
		ItShouldAnnotateShoot(s, map[string]string{v1beta1constants.AnnotationMigrationLiveMigrate: "true"})

		It("Live migrate Shoot", func(ctx SpecContext) {
			patch := client.MergeFrom(s.Shoot.DeepCopy())
			s.Shoot.Spec.SeedName = new(getSeedName(true))
			Eventually(ctx, func() error {
				return s.GardenClient.SubResource("binding").Patch(ctx, s.Shoot, patch)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		ItShouldGetResponsibleSeed(s)
		seed.ItShouldInitializeSeedClient(s.SeedContext)

		availabilityProbe.ItShouldStopProbeAndEnsureNoDowntime(s)

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

		if !workerless {
			// the "infrastructure" (aka the machine pods), still exist in the "kind-gardener-local" cluster
			ItShouldCompareMachinePodNamesAfter(s, func() client.Client { return seedClientSourceCluster }, machinePodNamesBeforeTest)

			ItShouldInitializeShootClient(s)
			inclusterclient.VerifyInClusterAccessToAPIServer(s)
		}

		ItShouldDeleteShoot(s)
		ItShouldWaitForShootToBeDeleted(s)
	}

	Context("Shoot with workers", Ordered, PriorityLonger, func() {
		test(NewTestContext().ForShoot(DefaultShoot("e2e-live-mgr")))
	})
})
