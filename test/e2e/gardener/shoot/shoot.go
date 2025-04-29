// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"html/template"

	"github.com/Masterminds/sprig/v3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	. "github.com/gardener/gardener/test/e2e/gardener"
	"github.com/gardener/gardener/test/utils/access"
	shootoperation "github.com/gardener/gardener/test/utils/shoots/operation"
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

// RegisterShootFlags defines and registers flags used by this test package
func RegisterShootFlags() {
	flag.StringVar(&projectNamespace, "project-namespace", "", "specify the gardener project namespace to run tests")
	flag.StringVar(&existingShootName, "existing-shoot-name", "", "specify an existing shoot to run tests against")
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

			completed, reason := shootoperation.ReconciliationSuccessful(s.Shoot)
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

func ItShouldRenderAndDeployTemplateToShoot(s *ShootContext, templateName string, values any) {
	GinkgoHelper()

	// This function was copied from test/framework/template.go
	It("Render and deploy template to shoot", func(ctx SpecContext) {
		templateFilepath := filepath.Join("/Users/I748357/go/src/github.com/gardener/gardener/test/framework/resources/templates", templateName)
		_, err := os.Stat(templateFilepath)
		Expect(err).NotTo(HaveOccurred(), "could not find template in %q", templateFilepath)

		tpl, err := template.
			New(templateName).
			Funcs(sprig.HtmlFuncMap()).
			ParseFiles(templateFilepath)
		Expect(err).NotTo(HaveOccurred(), "unable to parse template in %s: %w", templateFilepath, err)

		var writer bytes.Buffer
		err = tpl.Execute(&writer, values)
		Expect(err).NotTo(HaveOccurred(), "unable to execute template %s: %w", templateFilepath, err)

		manifestReader := kubernetes.NewManifestReader(writer.Bytes())
		err = s.ShootClientSet.Applier().ApplyManifest(ctx, manifestReader, kubernetes.DefaultMergeFuncs)
		Expect(err).NotTo(HaveOccurred(), "unable to apply template %s: %w", templateFilepath, err)
	}, SpecTimeout(time.Minute))
}

func ItShouldWaitForPodsInShootToBeReady(s *ShootContext, namespace string, podLabels labels.Selector) {
	GinkgoHelper()

	It("Wait for pods in Shoot to be ready", func(ctx SpecContext) {
		Eventually(ctx, func() error {
			podList := &corev1.PodList{}
			err := s.ShootClient.List(ctx, podList, client.InNamespace(namespace), client.MatchingLabelsSelector{Selector: podLabels})
			if err != nil {
				return err
			}

			for _, pod := range podList.Items {
				if pod.Status.Phase != corev1.PodRunning {
					return fmt.Errorf("pod %s/%s is not running", pod.Namespace, pod.Name)
				}
			}

			return nil
		}).Should(Succeed())
	}, SpecTimeout(time.Minute*5))
}
