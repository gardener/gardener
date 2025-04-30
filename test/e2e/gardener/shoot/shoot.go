// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"flag"
	"fmt"
	"slices"
	"time"

	machinev1alpha1 "github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/provider-local/machine-provider/local"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/framework"
	"github.com/gardener/gardener/test/utils/access"
)

// This file contains common shoot-related test steps (`It`s) used by multiple test cases (ordered containers).
// Each It should represent a single atomic step in the test execution, e.g., shoot creation is a separate step from
// waiting for it to be created. This allows for flexible reuse in different test cases ("mix and match"). E.g., one
// test case might need to wait for the shoot to be created, while another one expects that the shoot creation fails.
// Also, each It should accept a SpecContext parameter to implement suitable step-specific timeouts using the
// SpecTimeout decorator.
// Ideally, all there are no raw Expect statements. Instead, all network-related operations like API calls are wrapped
// in an Eventually statement to implement retries for making e2e less susceptible for intermittent failures.

var (
	projectNamespace  string
	existingShootName string
)

// LoadLegacyFlags initializes the above variables by looking up the flag values from the test framework. This is done
// because we cannot register flags with the same name as the test framework as long as there are still gardener e2e
// tests using it.
// TODO(timebertt): drop this function and re-define the flags here once the test/e2e/gardener package no longer uses
// the test framework (when finishing https://github.com/gardener/gardener/issues/11379)
func LoadLegacyFlags() {
	projectNamespace = flag.Lookup("project-namespace").Value.String()
	existingShootName = flag.Lookup("existing-shoot-name").Value.String()
}

// ItShouldCreateShoot creates the shoot. If an existing shoot is specified, the step is skipped.
func ItShouldCreateShoot(s *ShootContext) {
	GinkgoHelper()

	It("Create Shoot", func(ctx SpecContext) {
		if existingShootName != "" {
			s.Shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      existingShootName,
					Namespace: projectNamespace,
				},
			}
			s.Log = s.Log.WithValues("shoot", client.ObjectKeyFromObject(s.Shoot))

			Eventually(s.GardenKomega.Get(s.Shoot)).Should(Succeed())
			s.Log.Info("Using existing shoot")

			Skip("Using existing shoot instead of creating a new one")
		}

		s.Log.Info("Creating Shoot")

		Eventually(ctx, func() error {
			if err := s.GardenClient.Create(ctx, s.Shoot); !apierrors.IsAlreadyExists(err) {
				return err
			}

			return StopTrying("shoot already exists")
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldUpdateShootToHighAvailability updates shoot to high availability configuration with the given failure
// tolerance type.
func ItShouldUpdateShootToHighAvailability(s *ShootContext, failureToleranceType gardencorev1beta1.FailureToleranceType) {
	GinkgoHelper()

	It("Update Shoot to High Availability", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			if s.Shoot.Spec.ControlPlane == nil {
				s.Shoot.Spec.ControlPlane = &gardencorev1beta1.ControlPlane{}
			}

			s.Shoot.Spec.ControlPlane.HighAvailability = &gardencorev1beta1.HighAvailability{
				FailureTolerance: gardencorev1beta1.FailureTolerance{
					Type: failureToleranceType,
				},
			}
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldHibernateShoot hibernates the shoot.
func ItShouldHibernateShoot(s *ShootContext) {
	GinkgoHelper()

	It("Hibernate Shoot", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
				Enabled: ptr.To(true),
			}
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWakeUpShoot wakes up the shoot.
func ItShouldWakeUpShoot(s *ShootContext) {
	GinkgoHelper()

	It("Wake Up Shoot", func(ctx SpecContext) {
		Eventually(ctx, s.GardenKomega.Update(s.Shoot, func() {
			s.Shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
				Enabled: ptr.To(false),
			}
		})).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldDeleteShoot deletes the shoot. If an existing shoot is specified, the step is skipped.
func ItShouldDeleteShoot(s *ShootContext) {
	GinkgoHelper()

	It("Delete Shoot", func(ctx SpecContext) {
		if existingShootName != "" {
			Skip("Skip deleting existing shoot")
		}

		s.Log.Info("Deleting Shoot")

		Eventually(ctx, func(g Gomega) {
			g.Expect(gardenerutils.ConfirmDeletion(ctx, s.GardenClient, s.Shoot)).To(Succeed())
			g.Expect(s.GardenClient.Delete(ctx, s.Shoot)).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldWaitForShootToBeReconciledAndHealthy waits for the shoot to be reconciled successfully and healthy.
func ItShouldWaitForShootToBeReconciledAndHealthy(s *ShootContext) {
	GinkgoHelper()

	It("Wait for Shoot to be reconciled", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) bool {
			g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())

			completed, reason := framework.ShootReconciliationSuccessful(s.Shoot)
			if !completed {
				s.Log.Info("Waiting for reconciliation and healthiness", "lastOperation", s.Shoot.Status.LastOperation, "reason", reason)
			}
			return completed
		}).WithPolling(30 * time.Second).Should(BeTrue())

		s.Log.Info("Shoot has been reconciled and is healthy")
	}, SpecTimeout(30*time.Minute))
}

// ItShouldWaitForShootToBeDeleted waits for the shoot to be gone. If an existing shoot is specified, the step is
// skipped.
func ItShouldWaitForShootToBeDeleted(s *ShootContext) {
	GinkgoHelper()

	It("Wait for Shoot to be deleted", func(ctx SpecContext) {
		if existingShootName != "" {
			Skip("Skip deleting existing shoot")
		}

		Eventually(ctx, func() error {
			err := s.GardenKomega.Get(s.Shoot)()
			if err == nil {
				s.Log.Info("Waiting for deletion", "lastOperation", s.Shoot.Status.LastOperation)
			}
			return err
		}).WithPolling(30 * time.Second).Should(BeNotFoundError())

		s.Log.Info("Shoot has been deleted")
	}, SpecTimeout(20*time.Minute))
}

// ItShouldInitializeShootClient requests a kubeconfig for the shoot and initializes the context's shoot clients.
func ItShouldInitializeShootClient(s *ShootContext) {
	GinkgoHelper()

	It("Initialize Shoot client", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			clientSet, err := access.CreateShootClientFromAdminKubeconfig(ctx, s.GardenClientSet, s.Shoot)
			if err != nil {
				return err
			}

			s.WithShootClientSet(clientSet)
			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldGetResponsibleSeed retrieves the Seed object responsible for the shoot and stores it in ShootContext.Seed.
func ItShouldGetResponsibleSeed(s *ShootContext) {
	GinkgoHelper()

	It("Get the responsible Seed", func(ctx SpecContext) {
		s.Seed = &gardencorev1beta1.Seed{}

		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenKomega.Get(s.Shoot)()).To(Succeed())

			s.Seed.Name = gardenerutils.GetResponsibleSeedName(gardenerutils.GetShootSeedNames(s.Shoot))
			g.Expect(s.Seed.Name).NotTo(BeEmpty())
			g.Expect(s.GardenKomega.Get(s.Seed)()).To(Succeed())
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldInitializeSeedClient initializes the context's seed clients from the garden/seed-<name> kubeconfig secret.
// Requires ItShouldGetResponsibleSeed to be called first.
func ItShouldInitializeSeedClient(s *ShootContext) {
	GinkgoHelper()

	It("Initialize Seed client", func(ctx SpecContext) {
		Expect(s.Seed).NotTo(BeNil(), "ItShouldGetResponsibleSeed should be called first")

		seedSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "seed-" + s.Seed.Name,
				Namespace: "garden",
			},
		}
		Eventually(ctx, s.GardenKomega.Object(seedSecret)).Should(
			HaveField("Data", HaveKey(kubernetes.KubeConfig)),
			"secret %v should contain the seed kubeconfig",
		)

		clientSet, err := kubernetes.NewClientFromSecretObject(seedSecret,
			kubernetes.WithClientOptions(client.Options{Scheme: kubernetes.SeedScheme}),
			kubernetes.WithDisabledCachedClient(),
		)
		Expect(err).NotTo(HaveOccurred())
		s.WithSeedClientSet(clientSet)
	}, SpecTimeout(time.Minute))
}

// ItShouldAnnotateShoot sets the given annotation within the shoot metadata to the specified value and patches the shoot object
func ItShouldAnnotateShoot(s *ShootContext, annotations map[string]string) {
	GinkgoHelper()

	It("Annotate Shoot", func(ctx SpecContext) {
		patch := client.MergeFrom(s.Shoot.DeepCopy())

		for annotationKey, annotationValue := range annotations {
			s.Log.Info("Setting annotation", "annotation", annotationKey, "value", annotationValue)
			metav1.SetMetaDataAnnotation(&s.Shoot.ObjectMeta, annotationKey, annotationValue)
		}

		Eventually(ctx, func() error {
			return s.GardenClient.Patch(ctx, s.Shoot, patch)
		}).Should(Succeed())
	}, SpecTimeout(time.Minute))
}

// ItShouldLabelManualInPlaceNodesWithSelectedForUpdate labels all manual in-place nodes with the selected-for-update label.
// In the actual scenario, this should be done by the user, but for testing purposes, we do it here.
func ItShouldLabelManualInPlaceNodesWithSelectedForUpdate(s *ShootContext) {
	GinkgoHelper()

	It("should label all the manual in-place nodes with selected-for-update", func(ctx SpecContext) {
		for _, pool := range s.Shoot.Spec.Provider.Workers {
			if !v1beta1helper.IsUpdateStrategyManualInPlace(pool.UpdateStrategy) {
				continue
			}

			nodeList := &corev1.NodeList{}
			Eventually(
				ctx,
				s.ShootKomega.List(nodeList, client.MatchingLabels{v1beta1constants.LabelWorkerPool: pool.Name}),
			).Should(Succeed(), "nodes for pool %s should be listed", pool.Name)

			for _, node := range nodeList.Items {
				if metav1.HasLabel(node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate) {
					continue
				}

				Eventually(ctx, s.ShootKomega.Update(&node, func() {
					metav1.SetMetaDataLabel(&node.ObjectMeta, machinev1alpha1.LabelKeyNodeSelectedForUpdate, "true")
				})).Should(Succeed(), "node %s should be labeled", node.Name)
			}
		}
	}, SpecTimeout(2*time.Minute))
}

// ItShouldFindAllMachinePodsBefore finds all machine pods before running the required tests and returns their names.
func ItShouldFindAllMachinePodsBefore(s *ShootContext) sets.Set[string] {
	GinkgoHelper()

	machinePodNamesBeforeTest := sets.New[string]()

	It("Find all machine pods to ensure later that they weren't rolled out", func(ctx SpecContext) {
		beforeStartMachinePodList := &corev1.PodList{}
		Eventually(ctx, s.SeedKomega.List(beforeStartMachinePodList, client.InNamespace(s.Shoot.Status.TechnicalID), client.MatchingLabels{
			"app":              "machine",
			"machine-provider": "local",
		})).Should(Succeed())

		for _, item := range beforeStartMachinePodList.Items {
			machinePodNamesBeforeTest.Insert(item.Name)
		}
	}, SpecTimeout(time.Minute))

	return machinePodNamesBeforeTest
}

// ItShouldCompareMachinePodNamesAfter compares the machine pod names before and after running the required tests.
func ItShouldCompareMachinePodNamesAfter(s *ShootContext, machinePodNamesBeforeTest sets.Set[string]) {
	GinkgoHelper()

	It("Compare machine pod names", func(ctx SpecContext) {
		machinePodListAfterTest := &corev1.PodList{}
		Eventually(ctx, s.SeedKomega.List(machinePodListAfterTest, client.InNamespace(s.Shoot.Status.TechnicalID), client.MatchingLabels{
			"app":              "machine",
			"machine-provider": "local",
		})).Should(Succeed())

		machinePodNamesAfterTest := sets.New[string]()
		for _, item := range machinePodListAfterTest.Items {
			machinePodNamesAfterTest.Insert(item.Name)
		}

		Expect(machinePodNamesBeforeTest.UnsortedList()).To(ConsistOf(machinePodNamesAfterTest.UnsortedList()))
	}, SpecTimeout(time.Minute))
}

// ItShouldRewriteOS rewrites the /etc/os-release file for all machine pods to ensure that the version is overwritten for tests.
// This is a workaround for the fact that the machine image version is not available in the os-release file in the local provider.
func ItShouldRewriteOS(s *ShootContext) {
	GinkgoHelper()

	It("should rewrite the /etc/os-release for all machines", func(ctx SpecContext) {
		podList := &corev1.PodList{}
		Eventually(ctx, s.SeedKomega.List(podList,
			client.InNamespace(s.Shoot.Status.TechnicalID),
			client.MatchingLabels{
				"app":              "machine",
				"machine-provider": "local",
			},
		)).Should(Succeed())

		for _, pod := range podList.Items {
			node := &corev1.Node{}
			Eventually(func() error {
				return s.ShootClient.Get(ctx, client.ObjectKey{Namespace: pod.Namespace, Name: pod.Name}, node)
			}).Should(Succeed(), "should get node for pod %s", pod.Name)

			Expect(node.Labels).To(HaveKey(v1beta1constants.LabelWorkerPool))

			poolIndex := slices.IndexFunc(s.Shoot.Spec.Provider.Workers, func(pool gardencorev1beta1.Worker) bool {
				return pool.Name == node.Labels[v1beta1constants.LabelWorkerPool]
			})
			Expect(poolIndex).To(BeNumerically(">", -1))

			_, _, err := s.SeedClientSet.PodExecutor().Execute(ctx,
				pod.Namespace,
				pod.Name,
				local.MachinePodContainerName,
				"sed",
				"-i", "-E",
				fmt.Sprintf(
					`s/^PRETTY_NAME="[^"]*"/PRETTY_NAME="Machine Image Version %s (version overwritten for tests, check VERSION_ID for actual version)"/`,
					*s.Shoot.Spec.Provider.Workers[poolIndex].Machine.Image.Version,
				),
				"/etc/os-release",
			)

			Expect(err).NotTo(HaveOccurred(), "should rewrite /etc/os-release for pod %s", pod.Name)
		}
	}, SpecTimeout(2*time.Minute))
}

// ItShouldVerifyInPlaceUpdateStart verifies that the starting of in-place update  by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateStart(s *ShootContext) {
	GinkgoHelper()

	It("Verify in-place update start", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Shoot), s.Shoot)).Should(Succeed())

			g.Expect(s.Shoot.Status.InPlaceUpdates).NotTo(BeNil())
			g.Expect(s.Shoot.Status.InPlaceUpdates.PendingWorkerUpdates).NotTo(BeNil())
			g.Expect(s.Shoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate).NotTo(BeEmpty())
			g.Expect(s.Shoot.Status.InPlaceUpdates.PendingWorkerUpdates.ManualInPlaceUpdate).NotTo(BeEmpty())
			g.Expect(s.Shoot.Status.Constraints).To(ContainCondition(
				OfType(gardencorev1beta1.ShootManualInPlaceWorkersUpdated),
				WithReason("WorkerPoolsWithManualInPlaceUpdateStrategyPending"),
				Or(WithStatus(gardencorev1beta1.ConditionFalse), WithStatus(gardencorev1beta1.ConditionProgressing)),
			))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}

// ItShouldVerifyInPlaceUpdateCompletion verifies that the in-place update was completed successfully by checking the
// .status.inPlaceUpdates and the ManualInPlaceWorkersUpdated constraint of the Shoot.
func ItShouldVerifyInPlaceUpdateCompletion(s *ShootContext) {
	GinkgoHelper()

	It("Verify in-place update completion", func(ctx SpecContext) {
		Eventually(ctx, func(g Gomega) {
			g.Expect(s.GardenClient.Get(ctx, client.ObjectKeyFromObject(s.Shoot), s.Shoot)).Should(Succeed())

			g.Expect(s.Shoot.Status.InPlaceUpdates).To(BeNil())
			g.Expect(s.Shoot.Status.Constraints).NotTo(ContainCondition(
				OfType(gardencorev1beta1.ShootManualInPlaceWorkersUpdated),
			))
		}).Should(Succeed())
	}, SpecTimeout(2*time.Minute))
}
