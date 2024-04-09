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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	e2e "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	"github.com/gardener/gardener/test/utils/access"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		// Setting the kubernetes versions to < 1.27 as enableStaticTokenKubeconfig cannot be enabled
		// for Shoot clusters with k8s version >= 1.27.
		f.Shoot.Spec.Kubernetes.Version = "1.26.0"

		// Explicitly enable the static token kubeconfig to test the kubeconfig rotation.
		f.Shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)

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
				&rotationutils.ObservabilityVerifier{
					GetObservabilitySecretFunc: func(ctx context.Context) (*corev1.Secret, error) {
						secret := &corev1.Secret{}
						return secret, f.GardenClient.Client().Get(ctx, client.ObjectKey{Namespace: f.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(f.Shoot.Name, "monitoring")}, secret)
					},
					GetObservabilityEndpoint: func(secret *corev1.Secret) string {
						return secret.Annotations["url"]
					},
					GetObservabilityRotation: func() *gardencorev1beta1.ObservabilityRotation {
						return f.Shoot.Status.Credentials.Rotation.Observability
					},
				},
				&rotationutils.ETCDEncryptionKeyVerifier{
					RuntimeClient:               f.ShootFramework.SeedClient.Client(),
					Namespace:                   f.Shoot.Status.TechnicalID,
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
						return f.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
					},
					EncryptionKey:  v1beta1constants.SecretNameETCDEncryptionKey,
					RoleLabelValue: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
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
				&rotationutils.EncryptedDataVerifier{
					NewTargetClientFunc: func() (kubernetes.Interface, error) {
						return access.CreateShootClientFromAdminKubeconfig(ctx, f.GardenClient, f.Shoot)
					},
					Resources: []rotationutils.EncryptedResource{
						{
							NewObject: func() client.Object {
								return &corev1.Secret{
									ObjectMeta: metav1.ObjectMeta{GenerateName: "test-foo-", Namespace: "default"},
									StringData: map[string]string{"content": "foo"},
								}
							},
							NewEmptyList: func() client.ObjectList { return &corev1.SecretList{} },
						},
					},
				},
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

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-rotate"))
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-rotate"))
	})
})
