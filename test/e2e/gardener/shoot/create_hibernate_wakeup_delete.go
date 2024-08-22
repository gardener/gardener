// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	k8sclient "sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/utils/retry"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/node"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		It("Create, Hibernate, Wake up and Delete Shoot", Offset(1), func() {
			ctx, cancel := context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()

			if shoot.Spec.CloudProfileName == nil && shoot.Spec.CloudProfile != nil && shoot.Spec.CloudProfile.Kind == "NamespacedCloudProfile" {
				By("Create NamespacedCloudProfile")
				Expect(f.GardenClient.Client().Create(ctx, e2e.DefaultNamespacedCloudProfile())).To(Or(Succeed(), BeAlreadyExistsError()))

				By("Wait for new NamespacedCloudProfile to be reconciled")
				Expect(retry.UntilTimeout(ctx, 10*time.Second, 60*time.Second, func(ctx context.Context) (done bool, err error) {
					namespacedCloudProfile := &gardencorev1beta1.NamespacedCloudProfile{}
					err = f.GardenClient.Client().Get(ctx, k8sclient.ObjectKeyFromObject(e2e.DefaultNamespacedCloudProfile()), namespacedCloudProfile)
					if err != nil {
						return retry.SevereError(err)
					}
					if namespacedCloudProfile.Status.ObservedGeneration != namespacedCloudProfile.Generation {
						return retry.MinorError(fmt.Errorf("namespaced cloud profile exists but has not been reconciled yet"))
					}
					return retry.Ok()
				})).To(Succeed())

				DeferCleanup(func() {
					By("Delete NamespacedCloudProfile")
					ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
					defer cancel()
					Expect(f.GardenClient.Client().Delete(ctx, e2e.DefaultNamespacedCloudProfile())).To(Or(Succeed(), BeNotFoundError()))
				})
			}

			By("Create Shoot")
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				By("Verify Bootstrapping of Nodes with node-critical components")
				// We verify the node readiness feature in this specific e2e test because it uses a single-node shoot cluster.
				// The default shoot e2e test deals with multiple nodes, deleting all of them and waiting for them to be recreated
				// might increase the test duration undesirably.
				ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
				defer cancel()
				node.VerifyNodeCriticalComponentsBootstrapping(ctx, f.ShootFramework)
			}

			By("Hibernate Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 10*time.Minute)
			defer cancel()
			Expect(f.HibernateShoot(ctx, f.Shoot)).To(Succeed())

			By("Wake up Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.WakeUpShoot(ctx, f.Shoot)).To(Succeed())

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 15*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-wake-up"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-wake-up"))
	})

	Context("Shoot with workers with NamespacedCloudProfile", Label("basic"), func() {
		shoot := e2e.DefaultShoot("e2e-wake-up-ncp")
		shoot.Spec.CloudProfile = &gardencorev1beta1.CloudProfileReference{
			Kind: "NamespacedCloudProfile",
			Name: "my-profile",
		}
		test(shoot)
	})
})
