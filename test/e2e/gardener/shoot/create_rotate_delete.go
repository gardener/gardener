// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
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

func testCredentialRotation(parentCtx context.Context, v rotationutils.Verifiers, f *framework.ShootCreationFramework, startRotationAnnotation, completeRotationAnnotation string) {
	ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
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
		EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot) }).Should(Succeed())

		v.AfterPrepared(ctx)
	}

	testCredentialRotationComplete(parentCtx, v, f, completeRotationAnnotation)
}

func testCredentialRotationComplete(parentCtx context.Context, v rotationutils.Verifiers, f *framework.ShootCreationFramework, completeRotationAnnotation string) {
	if completeRotationAnnotation != "" {
		By("Complete credentials rotation")
		ctx, cancel := context.WithTimeout(parentCtx, 20*time.Minute)
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

	By("Cleanup")
	cleanupCtx, cleanupCancel := context.WithTimeout(parentCtx, 2*time.Minute)
	defer cleanupCancel()

	v.Cleanup(cleanupCtx)
}

func testCredentialRotationWithoutWorkersRollout(parentCtx context.Context, v rotationutils.Verifiers, f *framework.ShootCreationFramework) {
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
	defer cancel()

	By("Before credential rotation")
	v.Before(ctx)

	By("Find all machine pods to ensure later that they weren't rolled out")
	beforeStartMachinePodList := &corev1.PodList{}
	ExpectWithOffset(1, f.ShootFramework.SeedClient.Client().List(ctx, beforeStartMachinePodList, client.InNamespace(f.Shoot.Status.TechnicalID), client.MatchingLabels{
		"app":              "machine",
		"machine-provider": "local",
	})).To(Succeed())

	beforeStartMachinePodNames := sets.New[string]()
	for _, item := range beforeStartMachinePodList.Items {
		beforeStartMachinePodNames.Insert(item.Name)
	}

	By("Start credentials rotation without workers rollout")
	patch := client.MergeFrom(f.Shoot.DeepCopy())
	metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout)
	EventuallyWithOffset(1, func() error {
		return f.GardenClient.Client().Patch(ctx, f.Shoot, patch)
	}).Should(Succeed())

	EventuallyWithOffset(1, func(g Gomega) {
		g.ExpectWithOffset(1, f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot)).To(Succeed())
		g.ExpectWithOffset(1, f.Shoot.Annotations).NotTo(HaveKey(v1beta1constants.GardenerOperation))
		v.ExpectPreparingWithoutWorkersRolloutStatus(g)
	}).Should(Succeed())

	ExpectWithOffset(1, f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())
	EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot) }).Should(Succeed())

	By("Ensure workers were not rolled out")
	EventuallyWithOffset(1, func(g Gomega) {
		v.ExpectWaitingForWorkersRolloutStatus(g)
	}).Should(Succeed())

	afterStartMachinePodList := &corev1.PodList{}
	ExpectWithOffset(1, f.ShootFramework.SeedClient.Client().List(ctx, afterStartMachinePodList, client.InNamespace(f.Shoot.Status.TechnicalID), client.MatchingLabels{
		"app":              "machine",
		"machine-provider": "local",
	})).To(Succeed())

	afterStartMachinePodNames := sets.New[string]()
	for _, item := range afterStartMachinePodList.Items {
		afterStartMachinePodNames.Insert(item.Name)
	}

	ExpectWithOffset(1, beforeStartMachinePodNames.Equal(afterStartMachinePodNames)).To(BeTrue())

	By("Ensure all worker pools are marked as 'pending for roll out'")
	for _, worker := range f.Shoot.Spec.Provider.Workers {
		ExpectWithOffset(1, slices.ContainsFunc(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == worker.Name
		})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in CA rotation status")

		ExpectWithOffset(1, slices.ContainsFunc(f.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == worker.Name
		})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in service account key rotation status")
	}

	By("Remove last worker pool from spec and check that it is no longer pending for roll out")
	patch = client.MergeFrom(f.Shoot.DeepCopy())
	lastWorkerPoolName := f.Shoot.Spec.Provider.Workers[len(f.Shoot.Spec.Provider.Workers)-1].Name
	f.Shoot.Spec.Provider.Workers = slices.DeleteFunc(f.Shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
		return worker.Name == lastWorkerPoolName
	})
	EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Patch(ctx, f.Shoot, patch) }).Should(Succeed())

	ExpectWithOffset(1, f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())
	EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot) }).Should(Succeed())

	ExpectWithOffset(1, slices.ContainsFunc(f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
		return rollout.Name == lastWorkerPoolName
	})).To(BeFalse())
	ExpectWithOffset(1, slices.ContainsFunc(f.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
		return rollout.Name == lastWorkerPoolName
	})).To(BeFalse())

	By("Trigger rollout of pending worker pools")
	workerNames := sets.New[string]()
	for _, rollout := range f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts {
		workerNames.Insert(rollout.Name)
	}
	for _, rollout := range f.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts {
		workerNames.Insert(rollout.Name)
	}

	patch = client.MergeFrom(f.Shoot.DeepCopy())
	metav1.SetMetaDataAnnotation(&f.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, v1beta1constants.OperationRotateRolloutWorkers+"="+strings.Join(workerNames.UnsortedList(), ","))
	EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Patch(ctx, f.Shoot, patch) }).Should(Succeed())

	ExpectWithOffset(1, f.WaitForShootToBeReconciled(ctx, f.Shoot)).To(Succeed())
	EventuallyWithOffset(1, func() error { return f.GardenClient.Client().Get(ctx, client.ObjectKeyFromObject(f.Shoot), f.Shoot) }).Should(Succeed())

	ExpectWithOffset(1, f.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
	v.AfterPrepared(ctx)

	testCredentialRotationComplete(parentCtx, v, f, v1beta1constants.OperationRotateCredentialsComplete)
}

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	test := func(shoot *gardencorev1beta1.Shoot, withoutWorkersRollout bool) {
		f := defaultShootCreationFramework()
		f.Shoot = shoot

		It("Create Shoot, Rotate Credentials and Delete Shoot", Offset(1), Label("credentials-rotation"), func() {
			ctx, cancel := context.WithTimeout(parentCtx, 30*time.Minute)
			defer cancel()

			By("Create Shoot")
			Expect(f.CreateShootAndWaitForCreation(ctx, false)).To(Succeed())
			f.Verify()

			// TODO: add back VerifyInClusterAccessToAPIServer once this test has been refactored to ordered containers
			// if !v1beta1helper.IsWorkerless(s.Shoot) {
			// 	inclusterclient.VerifyInClusterAccessToAPIServer(s)
			// }

			// isolated test for ssh key rotation (does not trigger node rolling update)
			if !v1beta1helper.IsWorkerless(f.Shoot) && !withoutWorkersRollout {
				testCredentialRotation(parentCtx, rotationutils.Verifiers{&rotation.SSHKeypairVerifier{ShootCreationFramework: f}}, f, v1beta1constants.ShootOperationRotateSSHKeypair, "")
			}

			v := rotationutils.Verifiers{
				// basic verifiers checking secrets
				&rotation.CAVerifier{ShootCreationFramework: f},
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

			if !v1beta1helper.IsWorkerless(f.Shoot) && !withoutWorkersRollout {
				v = append(v, &rotation.SSHKeypairVerifier{ShootCreationFramework: f})
			}

			if !withoutWorkersRollout {
				// test rotation for every rotation type
				testCredentialRotation(parentCtx, v, f, v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsComplete)
			} else {
				testCredentialRotationWithoutWorkersRollout(parentCtx, v, f)
			}

			By("Renew shoot client after credentials rotation")
			Expect(f.ShootFramework.AddShoot(parentCtx, f.Shoot.Name, f.Shoot.Namespace)).To(Succeed())

			// TODO: add back VerifyInClusterAccessToAPIServer once this test has been refactored to ordered containers
			// if !v1beta1helper.IsWorkerless(s.Shoot) {
			// 	inclusterclient.VerifyInClusterAccessToAPIServer(s)
			// }

			By("Delete Shoot")
			ctx, cancel = context.WithTimeout(parentCtx, 20*time.Minute)
			defer cancel()
			Expect(f.DeleteShootAndWaitForDeletion(ctx, f.Shoot)).To(Succeed())
		})
	}

	Context("Shoot with workers", Label("basic"), func() {
		test(e2e.DefaultShoot("e2e-rotate"), false)

		Context("without workers rollout", Label("without-workers-rollout"), func() {
			shoot := e2e.DefaultShoot("e2e-rotate")
			shoot.Name = "e2e-rot-noroll"
			// Add a second worker pool when worker rollout should not be performed such that we can make proper
			// assertions of the shoot status
			shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers[0])
			shoot.Spec.Provider.Workers[len(shoot.Spec.Provider.Workers)-1].Name += "2"

			test(shoot, true)
		})
	})

	Context("Workerless Shoot", Label("workerless"), func() {
		test(e2e.DefaultWorkerlessShoot("e2e-rotate"), false)
	})
})
