// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Care controller tests", func() {
	var seed *gardencorev1beta1.Seed

	JustBeforeEach(func() {
		By("Setup manager")
		mgr, err := manager.New(restConfig, manager.Options{
			Scheme:  testScheme,
			Metrics: metricsserver.Options{BindAddress: "0"},
			Cache: cache.Options{
				DefaultNamespaces: map[string]cache.Config{
					testNamespace.Name: {},
					// kube-system namespace is added because in the controller we fetch cluster identity from
					// kube-system namespace and expect it to return not found error, but if don't create cache for it a
					// cache error will be returned.
					metav1.NamespaceSystem: {},
					// Seed namespace is added because controller reads secrets from this namespace
					seedNamespace.Name: {},
				},
				ByObject: map[client.Object]cache.ByObject{
					&gardencorev1beta1.Seed{}: {
						Label: labels.SelectorFromSet(labels.Set{testID: testRunID}),
					},
				},
			},
			Controller: controllerconfig.Controller{
				SkipNameValidation: ptr.To(true),
			},
			MapperProvider: apiutil.NewDynamicRESTMapper,
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		By("Register controller")
		Expect((&care.Reconciler{
			Config: gardenletconfigv1alpha1.SeedCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			Namespace: &testNamespace.Name,
			SeedName:  seedName,
		}).AddToManager(mgr, mgr, mgr)).To(Succeed())

		By("Start manager")
		mgrContext, mgrCancel := context.WithCancel(ctx)

		go func() {
			defer GinkgoRecover()
			Expect(mgr.Start(mgrContext)).To(Succeed())
		}()

		DeferCleanup(func() {
			By("Stop manager")
			mgrCancel()
		})

		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name:   seedName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: gardencorev1beta1.SeedSpec{
				Provider: gardencorev1beta1.SeedProvider{
					Region: "region",
					Type:   "providerType",
				},
				Ingress: &gardencorev1beta1.Ingress{
					Domain: "seed.example.com",
					Controller: gardencorev1beta1.IngressController{
						Kind: "nginx",
					},
				},
				DNS: gardencorev1beta1.SeedDNS{
					Provider: &gardencorev1beta1.SeedDNSProvider{
						Type: "providerType",
						SecretRef: corev1.SecretReference{
							Name:      "some-secret",
							Namespace: "some-namespace",
						},
					},
				},
				Networks: gardencorev1beta1.SeedNetworks{
					Pods:     "10.0.0.0/16",
					Services: "10.1.0.0/16",
					Nodes:    ptr.To("10.2.0.0/16"),
				},
			},
		}

		By("Create Seed")
		Expect(testClient.Create(ctx, seed)).To(Succeed())
		log.Info("Created Seed for test", "seed", seed.Name)

		DeferCleanup(func() {
			By("Delete Seed")
			Expect(testClient.Delete(ctx, seed)).To(Succeed())

			By("Ensure Seed is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
			}).Should(BeNotFoundError())
		})
	})

	Context("when ManagedResources for the Seed exist", func() {
		managedResourceName := "foo"

		BeforeEach(func() {
			By("Create ManagedResource")
			managedResource := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResource)).To(Succeed())
			log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: testNamespace.Name}})).To(Succeed())
			})
		})

		It("should set condition to False because some ManagedResource statuses are outdated", func() {
			By("Expect SeedSystemComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.Conditions
			}).Should(ContainCondition(
				OfType(gardencorev1beta1.SeedSystemComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect SeedSystemComponentsHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.Conditions
			}).Should(ContainCondition(
				OfType(gardencorev1beta1.SeedSystemComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("SystemComponentsRunning"),
				WithMessageSubstrings("All system components are healthy."),
			))
		})

		It("should delete stale pods in all namespaces except kube-system", func() {
			var (
				pod1 = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-", Namespace: testNamespace.Name},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "foo-container", Image: "foo"}}},
				}
				pod2 = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "pod-", Namespace: metav1.NamespaceSystem},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "foo-container", Image: "foo"}}},
				}
			)

			Expect(testClient.Create(ctx, pod1)).To(Succeed())
			Expect(testClient.Create(ctx, pod2)).To(Succeed())

			pod1.Status.Reason = "Evicted"
			pod2.Status.Reason = "Evicted"
			Expect(testClient.Status().Update(ctx, pod1)).To(Succeed())
			Expect(testClient.Status().Update(ctx, pod2)).To(Succeed())

			Eventually(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(pod1), &corev1.Pod{})
			}).Should(BeNotFoundError())

			Consistently(func() error {
				return testClient.Get(ctx, client.ObjectKeyFromObject(pod2), &corev1.Pod{})
			}).Should(Succeed())
		})
	})
})

func updateManagedResourceStatusToHealthy(name string) {
	By("Update status to healthy for ManagedResource " + name)
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace.Name}}
	ExpectWithOffset(1, testClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())

	managedResource.Status.ObservedGeneration = managedResource.Generation
	managedResource.Status.Conditions = []gardencorev1beta1.Condition{
		{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	ExpectWithOffset(1, testClient.Status().Update(ctx, managedResource)).To(Succeed())
}
