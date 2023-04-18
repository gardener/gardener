// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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

	Context("Shoot::e2e-upgrade", func() {
		var (
			parentCtx = context.Background()
			job       *batchv1.Job
			err       error
			shootTest = e2e.DefaultShoot("e2e-upgrade")
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
		)

		shootTest.Namespace = projectNamespace
		f.Shoot = shootTest

		When("Pre-Upgrade (Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Label("pre-upgrade"), func() {
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

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("post-upgrade"), func() {
			var (
				ctx        context.Context
				cancel     context.CancelFunc
				seedClient client.Client
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, shootTest)
				Expect(err).NotTo(HaveOccurred())
				seedClient = f.ShootFramework.SeedClient.Client()
			})

			It("verifying no downtime while upgrading gardener", func() {
				job = &batchv1.Job{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "zero-down-time-validator-update",
						Namespace: shootTest.Status.TechnicalID,
					}}
				Expect(seedClient.Get(ctx, client.ObjectKeyFromObject(job), job)).To(Succeed())
				Expect(job.Status.Failed).Should(BeZero())
				Expect(seedClient.Delete(ctx, job, client.PropagationPolicy(metav1.DeletePropagationForeground))).To(Succeed())
			})

			It("should able to delete a shoot which was created in previous release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, shootTest)).To(Succeed())
			})
		})
	})

	// This test will create a non-HA control plane shoot in Gardener version vX.X.X
	// and then upgrades shoot's control plane to HA once successfully upgraded Gardener version to vY.Y.Y.
	Context("Shoot::e2e-upgrade-ha", Label("high-availability"), func() {
		var (
			parentCtx = context.Background()
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{GardenerConfig: e2e.DefaultGardenConfig(projectNamespace)})
			shootTest = e2e.DefaultShoot("e2e-upgrade-ha")
			err       error
		)

		shootTest.Namespace = projectNamespace
		shootTest.Spec.ControlPlane = nil
		f.Shoot = shootTest

		When("(Gardener version:'"+gardenerPreviousVersion+"', Git version:'"+gardenerPreviousGitVersion+"')", Ordered, Label("pre-upgrade"), func() {
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

		When("Post-Upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("post-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 60*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, shootTest)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should be able to upgrade a non-HA shoot which was created in previous gardener release to HA with failure tolerance type '"+
				os.Getenv("SHOOT_FAILURE_TOLERANCE_TYPE")+"'", func() {
				highavailability.UpgradeAndVerify(ctx, f.ShootFramework, getFailureToleranceType())
			})

			It("should be able to delete a shoot which was created in previous gardener release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.GardenerFramework.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
			})
		})
	})

	Context("Shoot::e2e-upgrade-hib", func() {
		var (
			parentCtx = context.Background()
			f         = framework.NewShootCreationFramework(&framework.ShootCreationConfig{
				GardenerConfig: e2e.DefaultGardenConfig(projectNamespace),
			})
			shootTest = e2e.DefaultShoot("e2e-upgrade-hib")
			err       error
		)
		shootTest.Namespace = projectNamespace
		f.Shoot = shootTest

		When("Pre-upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("pre-upgrade"), func() {
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
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
				f.ShootFramework, err = f.NewShootFramework(ctx, shootTest)
				Expect(err).NotTo(HaveOccurred())
				Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())
			})
		})

		When("Post-upgrade (Gardener version:'"+gardenerCurrentVersion+"', Git version:'"+gardenerCurrentGitVersion+"')", Ordered, Label("post-upgrade"), func() {
			var (
				ctx    context.Context
				cancel context.CancelFunc
			)

			BeforeAll(func() {
				ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
				DeferCleanup(cancel)
				Expect(f.GetShoot(ctx, shootTest)).To(Succeed())
			})

			It("should be able to wake up a shoot which was hibernated in previous gardener release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerPreviousVersion))
				Expect(f.WakeUpShoot(ctx, shootTest)).To(Succeed())
			})

			It("should delete a shoot which was created in previous gardener release", func() {
				Expect(f.Shoot.Status.Gardener.Version).Should(Equal(gardenerCurrentVersion))
				Expect(f.DeleteShootAndWaitForDeletion(ctx, shootTest)).To(Succeed())
			})
		})
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
