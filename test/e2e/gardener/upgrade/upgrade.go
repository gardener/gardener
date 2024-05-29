// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package upgrade

import (
	"context"
	"os"
	"strings"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
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

	// We must not set the node CIDR in the pre-upgrade phase of the upgrade tests because it will not be handled
	// by the Infrastructure and Worker controllers until https://github.com/gardener/gardener/pull/9752 is released
	// (v1.96). Hence, we set it for upgrade tests from v1.95 in the post-upgrade phase.
	// TODO(timebertt): drop this after v1.96 has been released
	tmpPostUpgradeMigration := func(ctx context.Context, f *framework.ShootFramework) {
		GinkgoHelper()

		if !strings.HasPrefix(gardenerPreviousVersion, "v1.95.") {
			Skip("Only relevant for upgrade tests from v1.95")
		}

		if v1beta1helper.IsWorkerless(f.Shoot) {
			Skip("Not relevant for workerless shoots")
		}

		By("Configuring shoot nodes CIDR and reconciling Infrastructure")
		// In the new gardener version, the VPN connection only works if the nodes CIDR is specified.
		// Otherwise, communication with the machine pods will not be allowed anymore because we dropped the respective
		// NetworkPolicies.
		patch := client.MergeFrom(f.Shoot.DeepCopy())

		// Add shoot node CIDR which was not specified in the pre-upgrade phase.
		f.Shoot.Spec.Networking.Nodes = ptr.To("10.10.0.0/16")

		// reconcile the Infrastructure so that the new dedicated IPPools are created by provider-local
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationMaintain)
		Expect(f.GardenClient.Client().Patch(ctx, f.Shoot, patch)).To(Succeed())

		By("Waiting until MachineClass is reconciled")
		// We need to wait until the Worker controller adds the IPPool name in the MachineClass, before recreating the
		// machines. Otherwise, they won't get an IP address from the nodes CIDR.
		Eventually(ctx, func(g Gomega) {
			machineClassList := &machinev1alpha1.MachineClassList{}
			g.Expect(f.SeedClient.Client().List(ctx, machineClassList, client.InNamespace(f.Shoot.Status.TechnicalID))).To(Succeed())

			g.Expect(machineClassList.Items).NotTo(BeEmpty())
			machineClass := machineClassList.Items[0]
			g.Expect(string(machineClass.ProviderSpec.Raw)).To(ContainSubstring("ipPoolNameV4"))
		}).WithTimeout(10 * time.Minute).WithPolling(10 * time.Second).Should(Succeed())

		By("Recreating all machines")
		// recreate all machines so that they get an IP address from the new IPPool, i.e., from the specified node CIDR
		Expect(
			f.SeedClient.Client().DeleteAllOf(ctx, &machinev1alpha1.Machine{}, client.InNamespace(f.Shoot.Status.TechnicalID)),
		).To(Succeed())
		Expect(
			f.SeedClient.Client().DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace(f.Shoot.Status.TechnicalID), client.MatchingLabels{"app": "machine"}),
		).To(Succeed())

		if !v1beta1helper.HibernationIsEnabled(f.Shoot) {
			// dependency-watchdog (in its newest form) scales down controllers for single-node clusters too fast (e.g., if
			// the node lease is expired). e2e test shoots are single-node clusters, so we need to clean up the orphaned lease
			// from the previous step.
			Expect(f.ShootClient.Client().DeleteAllOf(ctx, &coordinationv1.Lease{}, client.InNamespace("kube-node-lease"))).To(Succeed())
		}

		By("Waiting for shoot to get healthy")
		Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())
	}

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
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
				DeferCleanup(cancel)
			})

			It("should create a shoot", func() {
				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})

			It("deploying zero-downtime validator job to ensure no downtime while after upgrading gardener", func() {
				shootSeedNamespace := f.Shoot.Status.TechnicalID
				job, err = highavailability.DeployZeroDownTimeValidatorJob(
					ctx,
					f.ShootFramework.SeedClient.Client(), "update", shootSeedNamespace,
					shootupdatesuite.GetKubeAPIServerAuthToken(
						ctx,
						f.ShootFramework.SeedClient,
						shootSeedNamespace,
					),
				)
				Expect(err).NotTo(HaveOccurred())
				shootupdatesuite.WaitForJobToBeReady(ctx, f.ShootFramework.SeedClient.Client(), job)
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			var (
				ctx        context.Context
				cancel     context.CancelFunc
				seedClient client.Client
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
				seedClient = f.ShootFramework.SeedClient.Client()
			})

			It("verifying no downtime while upgrading gardener", func() {
				job = &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "zero-down-time-validator-update",
						Namespace: f.Shoot.Status.TechnicalID,
					}}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
				Expect(job.Status.Failed).Should(BeZero())
				Expect(seedClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			})

			It("should be able to delete a shoot which was created in previous release", func() {
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
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 30*time.Minute)
				DeferCleanup(cancel)
			})

			It("should create a shoot", func() {
				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})
		})

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 60*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("post-upgrade migration steps for shoot nodes CIDR", func() {
				tmpPostUpgradeMigration(ctx, f.ShootFramework)
			})

			It("should be able to upgrade a non-HA shoot which was created in previous gardener release to HA with failure tolerance type '"+os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE")+"'", func() {
				highavailability.UpgradeAndVerify(ctx, f.ShootFramework, getFailureToleranceType())
			})

			It("should be able to delete a shoot which was created in previous gardener release", func() {
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
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
			})

			It("should create a shoot", func() {
				Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
				f.Verify()
			})

			It("should hibernate a shoot", func() {
				Expect(f.GetShoot(ctx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
				Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())
			})
		})

		When("Post-upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Offset(1), Label("post-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, f.Shoot)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, f.Shoot)
				Expect(err).NotTo(HaveOccurred())
			})

			It("post-upgrade migration steps for shoot nodes CIDR", func() {
				tmpPostUpgradeMigration(ctx, f.ShootFramework)
			})

			It("should be able to wake up a shoot which was hibernated in previous gardener release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.WakeUpShoot(ctx, f.Shoot)).To(Succeed())
			})

			It("should delete a shoot which was created in previous gardener release", func() {
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
