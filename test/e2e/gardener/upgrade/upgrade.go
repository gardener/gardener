// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	shootupdatesuite "github.com/gardener/gardener/test/utils/shoots/update"
	"github.com/gardener/gardener/test/utils/shoots/update/highavailability"
)

var _ = Describe("Gardener upgrade Tests for", func() {
	var (
		gardenerPreviousVersion    = os.Getenv("GARDENER_PREVIOUS_VERSION")
		gardenerPreviousGitVersion = os.Getenv("GARDENER_PREVIOUS_RELEASE")
		gardenerCurrentVersion     = os.Getenv("GARDENER_NEXT_VERSION")
		gardenerCurrentGitVersion  = os.Getenv("GARDENER_NEXT_RELEASE")
		projectNamespace           = "garden-local"
	)

	test_e2e_upgrade := func(shoot *gardencorev1beta1.Shoot) {
		var (
			parentCtx = context.Background()
			job       *batchv1.Job
			err       error
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
		)

		f.Shoot = shoot
		f.Shoot.Namespace = projectNamespace

		When("Pre-Upgrade (Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Offset(1), Label("pre-upgrade"), func() {
			It("should create a shoot", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
				defer cancel()

				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})

			It("deploying zero-downtime validator job to ensure no downtime while after upgrading gardener", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
				defer cancel()

				controlPlaneNamespace := f.Shoot.Status.TechnicalID
				job, err = highavailability.DeployZeroDownTimeValidatorJob(
					ctx,
					f.ShootFramework.SeedClient.Client(), "update", controlPlaneNamespace,
					shootupdatesuite.GetKubeAPIServerAuthToken(
						ctx,
						f.ShootFramework.SeedClient,
						controlPlaneNamespace,
					),
				)
				Expect(err).NotTo(HaveOccurred())
				shootupdatesuite.WaitForJobToBeReady(ctx, f.ShootFramework.SeedClient.Client(), job)
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			var seedClient client.Client

			BeforeAll(func() {
				Expect(f.GetShoot(parentCtx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(parentCtx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
				seedClient = f.ShootFramework.SeedClient.Client()
			})

			It("verifying no downtime while upgrading gardener", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 5*time.Minute)
				defer cancel()

				job = &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "zero-down-time-validator-update", Namespace: f.Shoot.Status.TechnicalID}}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
				Expect(job.Status.Failed).Should(BeZero())
				Expect(seedClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			})

			It("should be able to delete a shoot which was created in previous release", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
				defer cancel()

				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
			})
		})
	}

	Context("Shoot with workers::e2e-upgrade", func() {
		test_e2e_upgrade(e2e.DefaultShoot("e2e-upgrade"))
	})

	Context("Workerless Shoot::e2e-upgrade", Label("workerless"), func() {
		test_e2e_upgrade(e2e.DefaultWorkerlessShoot("e2e-upgrade"))
	})

	// This test will create a non-HA control plane shoot in Gardener version vX.X.X
	// and then upgrades shoot's control plane to HA once successfully upgraded Gardener version to vY.Y.Y.
	test_e2e_upgrade_ha := func(shoot *gardencorev1beta1.Shoot) {
		var (
			parentCtx = context.Background()
			err       error
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
		)

		f.Shoot = shoot
		f.Shoot.Namespace = projectNamespace
		f.Shoot.Spec.ControlPlane = nil

		When("(Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Offset(1), Label("pre-upgrade"), func() {
			It("should create a shoot", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
				defer cancel()

				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			BeforeAll(func() {
				Expect(f.GetShoot(parentCtx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(parentCtx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be able to upgrade a non-HA shoot which was created in previous gardener release to HA with failure tolerance type '"+os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE")+"'", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
				defer cancel()

				highavailability.UpgradeAndVerify(ctx, f.ShootFramework, getFailureToleranceType())
			})

			It("should be able to delete a shoot which was created in previous gardener release", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
				defer cancel()

				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
			})
		})
	}

	Context("Shoot with workers::e2e-upg-ha", Label("high-availability"), func() {
		test_e2e_upgrade_ha(e2e.DefaultShoot("e2e-upg-ha"))
	})

	Context("Workerless Shoot::e2e-upg-ha", Label("high-availability", "workerless"), func() {
		test_e2e_upgrade_ha(e2e.DefaultWorkerlessShoot("e2e-upg-ha"))
	})

	test_e2e_upgrade_hib := func(shoot *gardencorev1beta1.Shoot) {
		var (
			parentCtx = context.Background()
			err       error
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
		)

		f.GardenerFramework.Config.SkipAccessingShoot = true
		f.Shoot = shoot
		f.Shoot.Namespace = projectNamespace

		When("Pre-upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("pre-upgrade"), func() {
			It("should create a shoot", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
				defer cancel()

				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})

			It("should hibernate a shoot", func() {
				Expect(f.GetShoot(parentCtx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(parentCtx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())

				ctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
				defer cancel()

				Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())
			})
		})

		When("Post-upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			BeforeAll(func() {
				Expect(f.GetShoot(parentCtx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(parentCtx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be able to wake up a shoot which was hibernated in previous gardener release", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
				defer cancel()

				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.WakeUpShoot(ctx, f.Shoot)).To(Succeed())
			})

			It("should delete a shoot which was created in previous gardener release", func() {
				ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
				defer cancel()

				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerCurrentVersion))
				Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
			})
		})
	}

	Context("Shoot with workers::e2e-upg-hib", func() {
		test_e2e_upgrade_hib(e2e.DefaultShoot("e2e-upg-hib"))
	})

	Context("Workerless Shoot::e2e-upg-hib", Label("workerless"), func() {
		test_e2e_upgrade_hib(e2e.DefaultWorkerlessShoot("e2e-upg-hib"))
	})
})

// getFailureToleranceType returns a failureToleranceType based on env variable SHOOT_FAILURE_TOLERANCE_TYPE value
func getFailureToleranceType() gardencorev1beta1.FailureToleranceType {
	var failureToleranceType gardencorev1beta1.FailureToleranceType

	switch os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE") {
	case "zone":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeZone
	case "node":
		failureToleranceType = gardencorev1beta1.FailureToleranceTypeNode
	}
	return failureToleranceType
}
