// SPDX-FileCopyrightText: Contributors to the Gardener project
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

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/test/e2e"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/e2e/gardener/seed"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/inclusterclient"
	"github.com/gardener/gardener/test/e2e/gardener/shoot/internal/rotation"
	"github.com/gardener/gardener/test/utils/access"
	rotationutils "github.com/gardener/gardener/test/utils/rotation"
	"github.com/gardener/gardener/test/utils/shoots/update/inplace"
)

func testCredentialRotation(tc *ShootContext, shootVerifiers, utilsverifiers rotationutils.Verifiers, startRotationAnnotation, completeRotationAnnotation string, inPlaceUpdate bool) {
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
		ItShouldAnnotateShoot(tc, map[string]string{
			v1beta1constants.GardenerOperation: startRotationAnnotation,
		})

		It(fmt.Sprintf("Should not have operation annotation after requesting %s", startRotationAnnotation), func(ctx SpecContext) {
			EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Shoot)
		}, SpecTimeout(2*time.Minute))

		It("Rotation should be in preparing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Shoot), tc.Shoot)).To(Succeed())
				shootVerifiers.ExpectPreparingStatus(g)
				utilsverifiers.ExpectPreparingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		if inPlaceUpdate {
			inplace.ItShouldVerifyInPlaceUpdateStart(tc, true, true)
		}

		ItShouldWaitForShootToBeReconciledAndHealthy(tc)

		if inPlaceUpdate {
			inplace.ItShouldVerifyInPlaceUpdateCompletion(tc)
		}

		shootVerifiers.AfterPrepared(context.TODO())
		for _, k := range utilsverifiers {
			It(fmt.Sprintf("Verify after prepared for %T", k), func(ctx SpecContext) {
				k.AfterPrepared(ctx)
			})
		}
	}

	testCredentialRotationComplete(tc, shootVerifiers, utilsverifiers, completeRotationAnnotation)
}

func testCredentialRotationComplete(tc *ShootContext, shootVerifiers, utilsverifiers rotationutils.Verifiers, completeRotationAnnotation string) {
	if completeRotationAnnotation != "" {
		ItShouldAnnotateShoot(tc, map[string]string{
			v1beta1constants.GardenerOperation: completeRotationAnnotation,
		})

		It(fmt.Sprintf("Should not have operation annotation after requesting %s", completeRotationAnnotation), func(ctx SpecContext) {
			EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Shoot)
		}, SpecTimeout(2*time.Minute))

		It("Rotation in completing status", func(ctx SpecContext) {
			Eventually(ctx, func(g Gomega) {
				g.Expect(tc.GardenClient.Get(ctx, client.ObjectKeyFromObject(tc.Shoot), tc.Shoot)).To(Succeed())
				shootVerifiers.ExpectCompletingStatus(g)
				utilsverifiers.ExpectCompletingStatus(g)
			}).Should(Succeed())
		}, SpecTimeout(time.Minute))

		ItShouldWaitForShootToBeReconciledAndHealthy(tc)

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

func testCredentialRotationWithoutWorkersRollout(tc *ShootContext, shootVerifiers rotationutils.Verifiers, utilsverifiers rotationutils.Verifiers, inPlaceUpdate bool) {
	shootVerifiers.Before(context.TODO())
	for _, k := range utilsverifiers {
		It(fmt.Sprintf("Verify before for %T", k), func(ctx SpecContext) {
			k.Before(ctx)
		}, SpecTimeout(5*time.Minute))
	}

	machinePodNamesBeforeTest := ItShouldFindAllMachinePodsBefore(tc, func() client.Client { return tc.SeedClient })

	ItShouldAnnotateShoot(tc, map[string]string{
		v1beta1constants.GardenerOperation: v1beta1constants.OperationRotateCredentialsStartWithoutWorkersRollout,
	})

	It("Should not have operation annotation after requesting rotation without workers rollout", func(ctx SpecContext) {
		EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Shoot)
	}, SpecTimeout(2*time.Minute))

	It("Rotation in preparing without workers rollout status", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootVerifiers.ExpectPreparingWithoutWorkersRolloutStatus(g)
			utilsverifiers.ExpectPreparingWithoutWorkersRolloutStatus(g)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(tc)

	It("Ensure workers were not rolled out", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			shootVerifiers.ExpectWaitingForWorkersRolloutStatus(g)
			utilsverifiers.ExpectWaitingForWorkersRolloutStatus(g)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldCompareMachinePodNamesAfter(tc, func() client.Client { return tc.SeedClient }, machinePodNamesBeforeTest)

	It("Ensure all worker pools are marked as 'pending for roll out'", func() {
		for _, worker := range tc.Shoot.Spec.Provider.Workers {
			Expect(slices.ContainsFunc(tc.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in CA rotation status")

			Expect(slices.ContainsFunc(tc.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
				return rollout.Name == worker.Name
			})).To(BeTrue(), "worker pool "+worker.Name+" should be pending for roll out in service account key rotation status")
		}
	})

	var lastWorkerPoolName string
	It("Remove last worker pool from spec", func(ctx SpecContext) {
		Eventually(ctx, tc.GardenKomega.Update(tc.Shoot, func() {
			lastWorkerPoolName = tc.Shoot.Spec.Provider.Workers[len(tc.Shoot.Spec.Provider.Workers)-1].Name
			tc.Shoot.Spec.Provider.Workers = slices.DeleteFunc(tc.Shoot.Spec.Provider.Workers, func(worker gardencorev1beta1.Worker) bool {
				return worker.Name == lastWorkerPoolName
			})
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(tc)

	It("Last worker pool no longer pending rollout", func() {
		Expect(slices.ContainsFunc(tc.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
		Expect(slices.ContainsFunc(tc.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts, func(rollout gardencorev1beta1.PendingWorkersRollout) bool {
			return rollout.Name == lastWorkerPoolName
		})).To(BeFalse())
	})

	It("Trigger rollout of pending worker pools", func(ctx SpecContext) {
		workerNames := sets.New[string]()
		for _, rollout := range tc.Shoot.Status.Credentials.Rotation.CertificateAuthorities.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}
		for _, rollout := range tc.Shoot.Status.Credentials.Rotation.ServiceAccountKey.PendingWorkersRollouts {
			workerNames.Insert(rollout.Name)
		}

		// as this annotation is computed dynamically, we can't use the "ItShouldAnnotateShoot" function
		// this is because the ginkgo tree construction would just pass the empty output string to the annotate function
		rolloutWorkersAnnotation := v1beta1constants.OperationRotateRolloutWorkers + "=" + strings.Join(workerNames.UnsortedList(), ",")
		Eventually(ctx, tc.GardenKomega.Update(tc.Shoot, func() {
			metav1.SetMetaDataAnnotation(&tc.Shoot.ObjectMeta, v1beta1constants.GardenerOperation, rolloutWorkersAnnotation)
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))

	// In case of rotation without workers rollout, the in-place update status in only populated when the rollout for that worker pool is triggered
	if inPlaceUpdate {
		inplace.ItShouldVerifyInPlaceUpdateStart(tc, true, true)
	}

	ItShouldWaitForShootToBeReconciledAndHealthy(tc)

	if inPlaceUpdate {
		inplace.ItShouldVerifyInPlaceUpdateCompletion(tc)
	}

	It("Credential rotation in status prepared", func() {
		Expect(tc.Shoot.Status.Credentials.Rotation.CertificateAuthorities.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
		Expect(tc.Shoot.Status.Credentials.Rotation.ServiceAccountKey.Phase).To(Equal(gardencorev1beta1.RotationPrepared))
	})

	shootVerifiers.AfterPrepared(context.TODO())
	for _, k := range utilsverifiers {
		It(fmt.Sprintf("Verify after prepared for %s", reflect.TypeOf(k).String()), func(ctx SpecContext) {
			k.AfterPrepared(ctx)
		}, SpecTimeout(5*time.Minute))
	}

	testCredentialRotationComplete(tc, shootVerifiers, utilsverifiers, v1beta1constants.OperationRotateCredentialsComplete)
}

// Testing the annotation requires that we assert that a rollout has been triggered and finishes successfully.
// Current check verifies that a new MachineSet gets created after the annotation is set by
// checking the creation timestamp of the current MachineSet, annotating the Shoot, then checking that
// the creation timestamp of the new MachineSet is newer than the old one.
func testManualWorkersRollout(tc *ShootContext) {
	var oldMachineSetCreationTimestamps map[string]time.Time

	It("Should fetch old machine set creation timestamps", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			poolName := tc.Shoot.Spec.Provider.Workers[0].Name
			oldMachineSetCreationTimestamps = make(map[string]time.Time)

			machineDeployments := &machinev1alpha1.MachineDeploymentList{}
			g.Expect(tc.SeedClient.List(ctx, machineDeployments, client.InNamespace(tc.Shoot.Status.TechnicalID), client.MatchingLabels{"worker.gardener.cloud/pool": poolName})).To(Succeed())
			g.Expect(machineDeployments.Items).NotTo(BeEmpty(), "expected at least one MachineDeployment for worker pool %s", poolName)

			machineSetList := &machinev1alpha1.MachineSetList{}
			g.Expect(tc.SeedClient.List(ctx, machineSetList, client.InNamespace(tc.Shoot.Status.TechnicalID))).To(Succeed())

			ownerToMachineSets := gardenerutils.BuildOwnerToMachineSetsMap(machineSetList.Items)

			for _, machineDeployment := range machineDeployments.Items {
				machineSetListForDeployment := ownerToMachineSets[machineDeployment.Name]
				g.Expect(machineSetListForDeployment).NotTo(BeEmpty(), "no MachineSets found for MachineDeployment %s", machineDeployment.Name)
				g.Expect(machineSetListForDeployment).To(HaveLen(1), "expected exactly one MachineSet for MachineDeployment %s", machineDeployment.Name)

				oldMachineSetCreationTimestamps[machineDeployment.Name] = machineSetListForDeployment[0].CreationTimestamp.Time
			}
		}).Should(Succeed())
	}, SpecTimeout(10*time.Second))

	ItShouldAnnotateShoot(tc, map[string]string{
		v1beta1constants.GardenerOperation: "rollout-workers=" + tc.Shoot.Spec.Provider.Workers[0].Name,
	})

	It("Should not have operation annotation after requesting rollout of the first worker pool", func(ctx SpecContext) {
		EventuallyNotHaveOperationAnnotation(ctx, tc.GardenKomega, tc.Shoot)
	}, SpecTimeout(2*time.Minute))

	It("Should fetch new MachineSet creation timestamps and ensure they're newer", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			poolName := tc.Shoot.Spec.Provider.Workers[0].Name

			machineDeployments := &machinev1alpha1.MachineDeploymentList{}
			g.Expect(tc.SeedClient.List(ctx, machineDeployments, client.InNamespace(tc.Shoot.Status.TechnicalID), client.MatchingLabels{"worker.gardener.cloud/pool": poolName})).To(Succeed())
			g.Expect(machineDeployments.Items).NotTo(BeEmpty(), "expected at least one MachineDeployment for worker pool %s", poolName)

			machineSetList := &machinev1alpha1.MachineSetList{}
			g.Expect(tc.SeedClient.List(ctx, machineSetList, client.InNamespace(tc.Shoot.Status.TechnicalID))).To(Succeed())

			ownerToMachineSets := gardenerutils.BuildOwnerToMachineSetsMap(machineSetList.Items)

			for _, machineDeployment := range machineDeployments.Items {
				machineSetListForDeployment := ownerToMachineSets[machineDeployment.Name]
				g.Expect(machineSetListForDeployment).NotTo(BeEmpty(), "no MachineSets found for MachineDeployment %s", machineDeployment.Name)
				g.Expect(machineSetListForDeployment).To(HaveLen(1), "expected exactly one MachineSet for MachineDeployment %s", machineDeployment.Name)

				newMachineSetCreationTimestamp := machineSetListForDeployment[0].CreationTimestamp.Time
				oldTimestamp, exists := oldMachineSetCreationTimestamps[machineDeployment.Name]
				g.Expect(exists).To(BeTrue(), "no old timestamp found for MachineDeployment %s", machineDeployment.Name)
				g.Expect(oldTimestamp.Before(newMachineSetCreationTimestamp)).To(BeTrue(), "new MachineSet creation timestamp should be newer than the old one for MachineDeployment %s", machineDeployment.Name)
			}
		}).Should(Succeed())
	}, SpecTimeout(5*time.Minute))

	ItShouldWaitForShootToBeReconciledAndHealthy(tc)
}

var _ = Describe("Shoot Tests", Label("Shoot", "default"), func() {
	Describe("Create Shoot, Rotate Credentials and Delete Shoot", Label("credentials-rotation"), func() {
		test := func(tc *ShootContext, withoutWorkersRollout, workersRollout, withInPlaceUpdatePools bool) {
			BeforeAll(func() {
				tc.Init()
			})

			ItShouldCreateShoot(tc)
			ItShouldWaitForShootToBeReconciledAndHealthy(tc)
			ItShouldInitializeShootClient(tc)
			ItShouldGetResponsibleSeed(tc)
			seed.ItShouldInitializeSeedClient(&tc.SeedContext)

			// isolated test for ssh key rotation (does not trigger node rolling update)
			if !v1beta1helper.IsWorkerless(tc.Shoot) && !withoutWorkersRollout {
				testCredentialRotation(tc, rotationutils.Verifiers{&rotation.SSHKeypairVerifier{ShootContext: tc}}, nil, v1beta1constants.ShootOperationRotateSSHKeypair, "", false)
			}

			// because of the ongoing refactoring efforts, we currently have two sorts of verifiers
			// - refactored verifiers / verifiers dedicated to this test scenario, which use separate It's for structuring the tests
			// - unrefactored verifiers / shared verifiers, which use "By" statements to structure tests
			//
			// until all tests and thereby verifiers are refactored, we need to distinguish how we execute the verifier functions
			// TODO(Wieneo): Consolidate verifiers once operator e2e tests are refactored

			shootVerifiers := rotationutils.Verifiers{
				// basic verifiers checking secrets
				&rotation.CAVerifier{ShootContext: tc},
				&rotation.ShootAccessVerifier{ShootContext: tc},
			}
			utilsVerifiers := rotationutils.Verifiers{
				&rotationutils.ObservabilityVerifier{
					GetObservabilitySecretFunc: func(ctx context.Context) (*corev1.Secret, error) {
						secret := &corev1.Secret{}
						return secret, tc.GardenClient.Get(ctx, client.ObjectKey{Namespace: tc.Shoot.Namespace, Name: gardenerutils.ComputeShootProjectResourceName(tc.Shoot.Name, "monitoring")}, secret)
					},
					GetObservabilityEndpoint: func(secret *corev1.Secret) string {
						return secret.Annotations["plutono-url"]
					},
					GetObservabilityRotation: func() *gardencorev1beta1.ObservabilityRotation {
						return tc.Shoot.Status.Credentials.Rotation.Observability
					},
				},
				&rotationutils.ETCDEncryptionKeyVerifier{
					GetETCDSecretNamespace: func() string {
						return tc.Shoot.Status.TechnicalID
					},
					GetRuntimeClient: func() client.Client {
						return tc.SeedClient
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetETCDEncryptionKeyRotation: func() *gardencorev1beta1.ETCDEncryptionKeyRotation {
						return tc.Shoot.Status.Credentials.Rotation.ETCDEncryptionKey
					},
					EncryptionKey:  v1beta1constants.SecretNameETCDEncryptionKey,
					RoleLabelValue: v1beta1constants.SecretNamePrefixETCDEncryptionConfiguration,
				},
				&rotationutils.ServiceAccountKeyVerifier{
					GetServiceAccountKeySecretNamespace: func() string {
						return tc.Shoot.Status.TechnicalID
					},
					GetRuntimeClient: func() client.Client {
						return tc.SeedClient
					},
					SecretsManagerLabelSelector: rotation.ManagedByGardenletSecretsManager,
					GetServiceAccountKeyRotation: func() *gardencorev1beta1.ServiceAccountKeyRotation {
						return tc.Shoot.Status.Credentials.Rotation.ServiceAccountKey
					},
				},
				// advanced verifiers testing things from the user's perspective
				&rotationutils.EncryptedDataVerifier{
					NewTargetClientFunc: func(ctx context.Context) (kubernetes.Interface, error) {
						return access.CreateShootClientFromAdminKubeconfig(ctx, tc.GardenClientSet, tc.Shoot)
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

			if !v1beta1helper.IsWorkerless(tc.Shoot) && !withoutWorkersRollout {
				shootVerifiers = append(shootVerifiers, &rotation.SSHKeypairVerifier{ShootContext: tc})
			}

			var nodesOfInPlaceWorkersBeforeTest sets.Set[string]

			if withInPlaceUpdatePools {
				It("should get the nodes of worker with in-place update strategy", func(ctx SpecContext) {
					nodesOfInPlaceWorkersBeforeTest = inplace.FindNodesOfInPlaceWorkers(ctx, tc.Log, tc.ShootClient, tc.Shoot)
				}, SpecTimeout(2*time.Minute))
				inplace.ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(tc)
			}

			if !withoutWorkersRollout {
				// test rotation for every rotation type
				testCredentialRotation(tc, shootVerifiers, utilsVerifiers, v1beta1constants.OperationRotateCredentialsStart, v1beta1constants.OperationRotateCredentialsComplete, withInPlaceUpdatePools)
			} else {
				testCredentialRotationWithoutWorkersRollout(tc, shootVerifiers, utilsVerifiers, withInPlaceUpdatePools)
			}

			if !v1beta1helper.IsWorkerless(tc.Shoot) {
				// renew shoot clients after rotation
				ItShouldInitializeShootClient(tc)
				inclusterclient.VerifyInClusterAccessToAPIServer(tc)
			}

			if withInPlaceUpdatePools {
				It("should compare the node names after the test", func(ctx SpecContext) {
					totalInPlaceWorkersMaxSurge := inplace.GetTotalInPlaceWorkersMaxSurge(tc.Shoot)
					tc.Log.Info("Total in-place workers max surge", "maxSurge", totalInPlaceWorkersMaxSurge)

					nodesOfInPlaceWorkersAfterTest := inplace.FindNodesOfInPlaceWorkers(ctx, tc.Log, tc.ShootClient, tc.Shoot)
					tc.Log.Info("Nodes of in-place workers before test and after test", "beforeNodes", nodesOfInPlaceWorkersBeforeTest.UnsortedList(), "afterNodes", nodesOfInPlaceWorkersAfterTest.UnsortedList())

					Expect(nodesOfInPlaceWorkersAfterTest.Intersection(nodesOfInPlaceWorkersBeforeTest)).To(HaveLen(nodesOfInPlaceWorkersBeforeTest.Len() - totalInPlaceWorkersMaxSurge))
				}, SpecTimeout(2*time.Minute))
			}

			if workersRollout {
				testManualWorkersRollout(tc)
			}

			ItShouldDeleteShoot(tc)
			ItShouldWaitForShootToBeDeleted(tc)
		}

		Context("Shoot with workers", Label("basic"), func() {
			Context("with workers rollout", Label("with-workers-rollout"), Ordered, PriorityLonger, func() {
				shoot := DefaultShoot("e2e-rotate")

				worker1 := DefaultWorker("auto", new(gardencorev1beta1.AutoInPlaceUpdate))
				worker1.Minimum = 2
				worker1.Maximum = 2
				worker1.MaxUnavailable = new(intstr.FromInt(1))
				worker1.MaxSurge = new(intstr.FromInt(0))

				worker2 := DefaultWorker("manual", new(gardencorev1beta1.ManualInPlaceUpdate))

				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker1, worker2)

				test(NewShootContext(shoot), false, false, true)
			})

			Context("without workers rollout", Label("without-workers-rollout"), Ordered, PriorityLonger, func() {
				shoot := DefaultShoot("e2e-rot-noroll")

				worker2 := DefaultWorker("auto", new(gardencorev1beta1.AutoInPlaceUpdate))
				worker2.Minimum = 2
				worker2.Maximum = 2
				worker2.MaxUnavailable = new(intstr.FromInt(1))
				worker2.MaxSurge = new(intstr.FromInt(0))

				worker3 := DefaultWorker("manual", new(gardencorev1beta1.ManualInPlaceUpdate))

				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker2, worker3)

				// Add an extra worker pool when worker rollout should not be performed such that we can make proper
				// assertions of the shoot status
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, DefaultWorker(shoot.Spec.Provider.Workers[0].Name+"-nr", nil))

				test(NewShootContext(shoot), true, true, true)
			})
		})

		Context("Workerless Shoot", Label("workerless"), Ordered, PriorityLong, func() {
			test(NewShootContext(DefaultWorkerlessShoot("e2e-rotate")), false, false, false)
		})
	})
})
