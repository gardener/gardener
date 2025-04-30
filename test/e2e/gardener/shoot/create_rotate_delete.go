// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	"github.com/gardener/gardener/test/utils/access"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
)

func testCredentialRotation(s *ShootContext, shootVerifiers, utilsverifiers rotationutils.Verifiers, startRotationAnnotation, completeRotationAnnotation string, inPlaceUpdate bool) {
	// the verifier interface requires that we pass a context to some of the verifier functions
	// this is not needed anymore for refactored tests as these use the SpecContext supplied by the "It" statement
	// Also we cannot pass a nil as the context argument as this makes the linter unhappy :(
	// TODO(Wieneo): Remove context argument from verifier functions / interface once all verifiers got refactored

	// shoot verifiers are dedicated verifiers for this test and were already refactored to separate "It" statements
	// we can just execute the verifier function
	shootVerifiers.Before(context.TODO())

	// utils verifiers are shared verifiers which still use separate "By" statements for structuring tests and expect to be executed within an "It" statement
	// This is a problem as we removed the "top-level" "It" statements during the refactoring of this test
	// Until all verifiers are refactored, we need to instantiate separate "It" statements for all shared verifiers to allow for assertions
	for _, k := range utilsverifiers {
		It(fmt.Sprintf("Verify before for %T", k), func(ctx SpecContext) {
			k.Before(ctx)
		}, SpecTimeout(5*time.Minute))
	}

	if startRotationAnnotation != "" {
		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: startRotationAnnotation,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Shoot)

		It("Rotation should be in preparing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Shoot), s.Shoot)).To(Succeed())
				shootVerifiers.ExpectPreparingStatus(g)
				utilsverifiers.ExpectPreparingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		if inPlaceUpdate {
			ItShouldVerifyInPlaceUpdateStart(s)
		}

		ItShouldWaitForShootToBeReconciledAndHealthy(s)
		shootVerifiers.AfterPrepared(context.TODO())
		for _, k := range utilsverifiers {
			It(fmt.Sprintf("Verify after prepared for %T", k), func(ctx SpecContext) {
				k.AfterPrepared(ctx)
			})
		}
	}

	testCredentialRotationComplete(s, shootVerifiers, utilsverifiers, completeRotationAnnotation)
}

func testCredentialRotationComplete(s *ShootContext, shootVerifiers, utilsverifiers rotationutils.Verifiers, completeRotationAnnotation string) {
	if completeRotationAnnotation != "" {
		ItShouldAnnotateShoot(s, map[string]string{
			v1beta1constants.GardenerOperation: completeRotationAnnotation,
		})

		ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Shoot)

		It("Rotation in completing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Shoot), s.Shoot)).To(Succeed())
				shootVerifiers.ExpectCompletingStatus(g)
				utilsverifiers.ExpectCompletingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(s)

		shootVerifiers.AfterCompleted(context.TODO())
		for _, k := range utilsverifiers {
			It(fmt.Sprintf("Verify after completed for %T", k), func(ctx SpecContext) {
				k.AfterCompleted(ctx)
			})
		}
	}

	shootVerifiers.Cleanup(context.TODO())
	for _, k := range utilsverifiers {
		if cleanup, ok := k.(rotationutils.CleanupVerifier); ok {
			It(fmt.Sprintf("Cleanup for %s", reflect.TypeOf(k).String()), func(ctx SpecContext) {
				cleanup.Cleanup(ctx)
			})
		}
	}
}

func testCredentialRotationWithoutWorkersRollout(s *ShootContext, shootVerifiers rotationutils.Verifiers, utilsverifiers rotationutils.Verifiers) {
	shootVerifiers.Before(context.TODO())
	for _, k := range utilsverifiers {
		It(fmt.Sprintf("Verify before for %T", k), func(ctx SpecContext) {
			k.Before(ctx)
		}, SpecTimeout(5*time.Minute))
	}

	machinePodNamesBeforeTest := ItShouldFindAllMachinePodsBefore(s)

	ItShouldAnnotateShoot(s, map[string]string{
		v1beta1constants.GardenerOperation: v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	})

	ItShouldEventuallyNotHaveOperationAnnotation(s.GardenKomega, s.Shoot)

	It("Rotation in preparing without workers rollout status", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootVerifiers.ExpectPreparingWithoutWorkersRolloutStatus(g)
			utilsverifiers.ExpectPreparingWithoutWorkersRolloutStatus(g)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Ensure workers were not rolled out", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootVerifiers.ExpectWaitingForWorkersRolloutStatus(g)
			utilsverifiers.ExpectWaitingForWorkersRolloutStatus(g)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldCompareMachinePodNamesAfter(s, machinePodNamesBeforeTest)

	It("Ensure all worker pools are marked as 'pending for roll out'", func() {
		for _, worker := range s.Shoot.Spec.Provider.Workers {
			Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in CA rotation status")

			Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in service account key rotation status")
		}
	})

	var lastWorkerPoolName string
	It("Remove last worker pool from spec", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			lastWorkerPoolName = s.Shoot.Spec.Provider.Workers[len(s.Shoot.Spec.Provider.Workers)-1].Name
			s.Shoot.Spec.Provider.Workers = slices.DeleteFunc(s.Shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
				return worker.Name == lastWorkerPoolName
			})
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Last worker pool no longer pending rollout", func() {
		Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
		Expect(slices.ContainsFunc(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
	})

	It("Trigger rollout of pending worker pools", func(ctx SpecContext) {
		workerNames := sets.New[string]()
		for _, rollout := range s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}
		for _, rollout := range s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}

		// as this annotation is computed dynamically, we can't use the "ItShouldAnnotateShoot" function
		// this is because the ginkgo tree construction would just pass the empty output string to the annotate function
		rolloutWorkersAnnotation := v1beta1constants.OperationRotateRolloutWorkers + "=" + strings.Join(workerNames.UnsortedList(), ",")
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			metav1.SetMetaDataAnnotation(&s.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, rolloutWorkersAnnotation)
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(s)

	It("Credential rotation in status prepared", func() {
		Expect(s.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
		Expect(s.Shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
	})

	shootVerifiers.AfterPrepared(context.TODO())
	for _, k := range utilsverifiers {
		It(fmt.Sprintf("Verify after prepared for %s", reflect.TypeOf(k).String()), func(ctx SpecContext) {
			k.AfterPrepared(ctx)
		}, SpecTimeout(5*time.Minute))
	}

	testCredentialRotationComplete(s, shootVerifiers, utilsverifiers, v1beta1constants.OperationRotateCredentialsComplete)
}

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create Shoot, Rotate Credentials and Delete Shoot", Label("credentials-rotation"), func() {
		test := func(s *ShootContext, withoutWorkersRollout, inPlaceUpdate bool) {
			ItShouldCreateShoot(s)
			ItShouldWaitForShootToBeReconciledAndHealthy(s)
			ItShouldInitializeShootClient(s)
			ItShouldGetResponsibleSeed(s)
			ItShouldInitializeSeedClient(s)

			// isolated test for ssh key rotation (does not trigger node rolling update)
			if !v1beta1helper.IsWorkerless(s.Shoot) && !withoutWorkersRollout && !inPlaceUpdate {
				testCredentialRotation(s, rotationutils.Verifiers{&rotation.SSHKeypairVerifier{ShootContext: s}}, nil, v1beta1constants.ShootOperationRotateSSHKeypair, "", false)
			}

			// because of the ongoing refactoring efforts, we currently have two sorts of verifiers
			// - refactored verifiers / verifiers dedicated to this test scenario, which use separate It's for structuring the tests
			// - unrefactored verifiers / shared verifiers, which use "By" statements to structure tests
			//
			// until all tests and thereby verifiers are refactored, we need to distinguish how we execute the verifier functions
			// TODO(Wieneo): Consolidate verifiers once operator e2e tests are refactored

			shootVerifiers := rotationutils.Verifiers{
				// basic verifiers checking secrets
				&rotation.CAVerifier{ShootContext: s},
				&rotation.ShootAccessVerifier{ShootContext: s},
			}
			utilsVerifiers := rotationutils.Verifiers{
				&rotationutils.ObservabilityVerifier{
					GetObservabilitySecretFunc: func(ctx context.Context) (*corev1.Secret, error) {
						secret := &corev1.Secret{}
						return secret, s.GardenClient.Get(ctx, client.ObjectKey{Namespace: s.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(s.Shoot.Name, "monitoring")}, secret)
					},
					GetObservabilityEndpoint: func(secret *corev1.Secret) string {
						return secret.Annotations["plutono-url"]
					},
					GetObservabilityRotation: func() *gardencorev1beta1.ObservabilityRotation {
						return s.Shoot.Status.Credentials.Rotation.Observability
					},
				},
				&rotationutils.ETCDEncryptionKeyVerifier{
					GetETCDSecretNamespace: func() string {
						return s.Shoot.Status.TechnicalID
					},
					GetRuntimeClient: func() client.Client {
						return s.SeedClient
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
						return s.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
					},
					EncryptionKey:  v1beta1constants.SecretNameETCDEncryptionKey,
					RoleLabelValue: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
				},
				&rotationutils.ServiceAccountKeyVerifier{
					GetServiceAccountKeySecretNamespace: func() string {
						return s.Shoot.Status.TechnicalID
					},
					GetRuntimeClient: func() client.Client {
						return s.SeedClient
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetServiceAccountKeyRotation: func() *gardencorev1beta1.ServiceAccountKeyRotation {
						return s.Shoot.Status.Credentials.Rotation.ServiceAccountKey
					},
				},
				// advanced verifiers testing things from the user's perspective
				&rotationutils.EncryptedDataVerifier{
					NewTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
						return access.CreateShootClientFromAdminKubeconfig(ctx, s.GardenClientSet, s.Shoot)
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
			}

			if !v1beta1helper.IsWorkerless(s.Shoot) && !withoutWorkersRollout && !inPlaceUpdate {
				shootVerifiers = append(shootVerifiers, &rotation.SSHKeypairVerifier{ShootContext: s})
			}

			var machinePodNamesBeforeTest sets.Set[string]

			if inPlaceUpdate {
				machinePodNamesBeforeTest = ItShouldFindAllMachinePodsBefore(s)
				ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(s)
			}

			if !withoutWorkersRollout {
				// test rotation for every rotation type
				testCredentialRotation(s, shootVerifiers, utilsVerifiers, v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsComplete, inPlaceUpdate)
			} else {
				testCredentialRotationWithoutWorkersRollout(s, shootVerifiers, utilsVerifiers)
			}

			if inPlaceUpdate {
				ItShouldCompareMachinePodNamesAfter(s, machinePodNamesBeforeTest)
				ItShouldVerifyInPlaceUpdateCompletion(s)
			}

			if !v1beta1helper.IsWorkerless(s.Shoot) {
				// renew shoot clients after rotation
				ItShouldInitializeShootClient(s)
				inclusterclient.VerifyInClusterAccessToAPIServer(s)
			}

			ItShouldDeleteShoot(s)
			ItShouldWaitForShootToBeDeleted(s)
		}

		Context("Shoot with workers", Label("basic"), func() {
			Context("with workers rollout", Label("with-workers-rollout"), Ordered, func() {
				test(NewTestContext().ForShoot(DefaultShoot("e2e-rotate")), false, false)
			})

			Context("with workers rollout, in-place update strategy", Label("with-workers-rollout", "in-place"), Ordered, func() {
				var s *ShootContext
				BeforeTestSetup(func() {
					shoot := DefaultShoot("e2e-rot-ip")

					worker1 := DefaultWorker("auto", ptr.To(gardencorev1beta1.AutoInPlaceUpdate))
					worker1.Minimum = 2
					worker1.Maximum = 2
					worker1.MaxUnavailable = ptr.To(intstr.FromInt(1))
					worker1.MaxSurge = ptr.To(intstr.FromInt(0))

					worker2 := DefaultWorker("manual", ptr.To(gardencorev1beta1.ManualInPlaceUpdate))

					shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{worker1, worker2}

					s = NewTestContext().ForShoot(shoot)
				})

				test(s, false, true)
			})

			Context("without workers rollout", Label("without-workers-rollout"), Ordered, func() {
				var s *ShootContext
				BeforeTestSetup(func() {
					shoot := DefaultShoot("e2e-rotate")
					shoot.Name = "e2e-rot-noroll"
					// Add a second worker pool when worker rollout should not be performed such that we can make proper
					// assertions of the shoot status
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, DefaultWorker(shoot.Spec.Provider.Workers[0].Name+"-2", nil))

					s = NewTestContext().ForShoot(shoot)
				})

				test(s, true, false)
			})

		})

		Context("Workerless Shoot", Label("workerless"), Ordered, func() {
			test(NewTestContext().ForShoot(DefaultWorkerlessShoot("e2e-rotate")), false, false)
		})
	})
})
