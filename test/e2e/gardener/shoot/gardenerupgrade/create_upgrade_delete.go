// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenerupgrade

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	. "github.com/gardener/gardener/test/e2e/gardener/shoot/internal"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var _ = Describe("Gardener Upgrade Tests", func() {
	Describe("Create Shoot, Upgrade Gardener version, Delete Shoot", func() {
		test := func(s *ShootContext) {
			var (
				zeroDowntimeJobName = "update"
				zeroDowntimeJob     *batchv1.Job
			)

			Describe("Pre-Upgrade"+gardenerInfoPreUpgrade, Label("pre-upgrade"), func() {
				ItShouldCreateShoot(s)
				ItShouldWaitForShootToBeReconciledAndHealthy(s)
				ItShouldGetResponsibleSeed(s)
				ItShouldInitializeSeedClient(s)

				It("Deploy zero-downtime validator job to ensure no API server downtime while upgrading Gardener", func(ctx SpecContext) {
					By("Fetch kube-apiserver auth token for zero-downtime validator job")
					deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: v1beta1constants.DeploymentNameKubeAPIServer, Namespace: s.Shoot.Status.TechnicalID}}
					Eventually(s.SeedKomega.Get(deployment)).Should(Succeed())
					authToken := deployment.Spec.Template.Spec.Containers[0].ReadinessProbe.HTTPGet.HTTPHeaders[0].Value

					Eventually(ctx, func() error {
						var err error
						zeroDowntimeJob, err = highavailability.DeployZeroDownTimeValidatorJob(ctx, s.SeedClient, zeroDowntimeJobName, s.Shoot.Status.TechnicalID, authToken)
						return err
					}).Should(Succeed())
				}, SpecTimeout(time.Minute))

				It("Wait until zero-downtime validator job is ready", func(ctx SpecContext) {
					// TODO(rfranzke): Refactor this function (move it to another package closer to here, and revisit
					//  its implementation.
					shootupdatesuite.WaitForJobToBeReady(ctx, s.SeedClient, zeroDowntimeJob)
				}, SpecTimeout(5*time.Minute))
			})

			Describe("Post-Upgrade"+gardenerInfoPostUpgrade, Label("post-upgrade"), func() {
				BeforeTestSetup(func() {
					It("Read Shoot from API server", func(ctx SpecContext) {
						Eventually(ctx, s.GardenKomega.Get(s.Shoot)).Should(Succeed())
					}, SpecTimeout(time.Minute))
				})

				It("should ensure Shoot was hibernated with previous Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				})

				// TODO(rfranzke): Make this 'It' reusable for the default Shoot create_update_delete test after
				//  https://github.com/gardener/gardener/pull/11540 has been merged.
				It("Ensure there was no downtime while upgrading Gardener", func(ctx SpecContext) {
					zeroDowntimeJob = highavailability.EmptyZeroDownTimeValidatorJob(zeroDowntimeJobName, s.Shoot.Status.TechnicalID)
					Eventually(ctx, s.SeedKomega.Get(zeroDowntimeJob)).Should(Succeed())
					Expect(zeroDowntimeJob.Status.Failed).Should(BeZero())
				}, SpecTimeout(time.Minute))

				It("should ensure Shoot was woken up with current Gardener version", func() {
					Expect(s.Shoot.Status.Gardener.Version).Should(Equal(gardenerCurrentVersion))
				})

				ItShouldDeleteShoot(s)
				ItShouldWaitForShootToBeDeleted(s)

				// TODO(rfranzke): Make this 'AfterAll' reusable for the default Shoot create_update_delete test after
				//  https://github.com/gardener/gardener/pull/11540 has been merged.
				AfterAll(func(ctx SpecContext) {
					By("Clean up zero-downtime validator job")
					Eventually(ctx, func() error {
						return s.SeedClient.Delete(ctx, zeroDowntimeJob, client.PropagationPolicy(metav1.DeletePropagationForeground))
					}).Should(Or(Succeed(), BeNotFoundError()))
				}, NodeTimeout(time.Minute))
			})
		}

		Context("Shoot with workers", Ordered, func() {
			test(NewTestContext().ForShoot(DefaultShoot("e2e-upgrade")))
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-upgrade")))
		})
	})
})
