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

package shoot

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	var test = func(f *framework.ShootCreationFramework) {
		// Explicitly enable the static token kubeconfig to test the kubeconfig rotation.
		f.Shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = pointer.Bool(true)

		It("Create Shoot, Rotate Credentials and Delete Shoot", Offset(1), Label("credentials-rotation"), func() {
			ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()

			By("Create Shoot")
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			v := rotationutils.Verifiers{
				// basic verifiers checking secrets
				&rotation.CAVerifier{ShootCreationFramework: f},
				&rotation.KubeconfigVerifier{ShootCreationFramework: f},
				&rotation.ObservabilityVerifier{ShootCreationFramework: f},
				&rotationutils.ETCDEncryptionKeyVerifier{
					RuntimeClient:               f.ShootFramework.SeedClient.Client(),
					Namespace:                   f.Shoot.Status.TechnicalID,
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
						return f.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
					},
				},
				&rotationutils.ServiceAccountKeyVerifier{
					RuntimeClient:               f.ShootFramework.SeedClient.Client(),
					Namespace:                   f.Shoot.Status.TechnicalID,
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetServiceAccountKeyRotation: func() *gardencorev1beta1.ServiceAccountKeyRotation {
						return f.Shoot.Status.Credentials.Rotation.ServiceAccountKey
					},
				},

				// advanced verifiers testing things from the user's perspective
				&rotationutils.SecretEncryptionVerifier{NewTargetClientFunc: func() (kubernetes.Interface, error) {
					return access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
				}},
				&rotation.ShootAccessVerifier{ShootCreationFramework: f},
			}

			if !v1beta1helper.IsWorkerless(f.Shoot) {
				v = append(v, &rotation.SSHKeypairVerifier{ShootCreationFramework: f})
			}

			DeferCleanup(func() {
				ctx, cancel := context.WithTimeout(parentCtx, 2*time.Minute)
				defer cancel()

				v.Cleanup(ctx)
			})

			v.Before(ctx)

			By("Start credentials rotation")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()

			patch := client.MergeFrom(f.Shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateCredentialsStart)
			Eventually(func() error {
				return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
				g.Expect(f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				v.ExpectPreparingStatus(g)
			}).Should(Succeed())

			Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

			Eventually(func() error {
				return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
			}).Should(Succeed())

			v.AfterPrepared(ctx)

			By("Complete credentials rotation")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()

			patch = client.MergeFrom(f.Shoot.DeepCopy())
			metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateCredentialsComplete)
			Eventually(func() error {
				return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
			}).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
				g.Expect(f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
				v.ExpectCompletingStatus(g)
			}).Should(Succeed())

			Expect(f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			}).Should(Succeed())

			v.AfterCompleted(ctx)

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", func() {
		f := defaultShootCreationFramework()
		f.Shoot = e2e.DefaultShoot("e2e-rotate", false)

		test(f)
	})

	Context("Workerless Shoot", func() {
		f := defaultShootCreationFramework()
		f.Shoot = e2e.DefaultShoot("e2e-rotate", true)

		test(f)
	})
})
