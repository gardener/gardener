// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

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
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

func testCredentialRotation(ctx context.Context, v rotationutils.Verifiers, f *framework.ShootCreationFramework, startRotationAnnotation string, completeRotationAnnotation string) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	By("Before credential rotation")
	v.Before(ctx)

	if startRotationAnnotation != "" {
		By("Start credentials rotation")
		patch := client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, startRotationAnnotation)
		EventuallyWithOffset(1, func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		EventuallyWithOffset(1, func(g Gomega) {
			g.ExpectWithOffset(1, f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			g.ExpectWithOffset(1, f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectPreparingStatus(g)
		}).Should(Succeed())

		ExpectWithOffset(1, f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		EventuallyWithOffset(1, func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())

		v.AfterPrepared(ctx)
	}

	if completeRotationAnnotation != "" {
		By("Complete credentials rotation")
		ctx, cancel = context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		patch := client.MergeFrom(f.Shoot.DeepCopy())
		metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, completeRotationAnnotation)
		EventuallyWithOffset(1, func() error {
			return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
		}).Should(Succeed())

		EventuallyWithOffset(1, func(g Gomega) {
			g.ExpectWithOffset(1, f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
			g.ExpectWithOffset(1, f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
			v.ExpectCompletingStatus(g)
		}).Should(Succeed())

		ExpectWithOffset(1, f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())

		EventuallyWithOffset(1, func() error {
			return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)
		}).Should(Succeed())

		v.AfterCompleted(ctx)
	}

	func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cleanupCancel()

		v.Cleanup(cleanupCtx)
	}()
}

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

			// isolated test for ssh key rotation (does not trigger node rolling update)
			if !v1beta1helper.IsWorkerless(f.Shoot) {
				testCredentialRotation(parentCtx, rotationutils.Verifiers{&rotation.SSHKeypairVerifier{ShootCreationFramework: f}}, f, v1beta1constants.ShootOperationRotateSSHKeypair, "")
			}

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
						return secret.Annotations["plutono-url"]
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
					NewTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
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

			// test rotation for every rotation type
			testCredentialRotation(parentCtx, v, f, v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsComplete)

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
