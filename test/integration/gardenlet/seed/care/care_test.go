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

package care_test

import (
	"context"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/component/autoscaling/clusterautoscaler"
	"github.com/gardener/gardener/pkg/component/autoscaling/hvpa"
	"github.com/gardener/gardener/pkg/component/autoscaling/vpa"
	"github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/component/etcd"
	"github.com/gardener/gardener/pkg/component/networking/nginxingress"
	"github.com/gardener/gardener/pkg/component/nodemanagement/dependencywatchdog"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/kubestatemetrics"
	"github.com/gardener/gardener/pkg/component/observability/monitoring/prometheusoperator"
	seedsystem "github.com/gardener/gardener/pkg/component/seed/system"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	"github.com/gardener/gardener/pkg/gardenlet/controller/seed/care"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	thirdpartyapiutil "github.com/gardener/gardener/third_party/controller-runtime/pkg/apiutil"
)

var _ = Describe("Seed Care controller tests", func() {
	var (
		seed           *gardencorev1beta1.Seed
		loggingEnabled bool
		valiEnabled    bool
	)

	BeforeEach(func() {
		loggingEnabled = false
		valiEnabled = false
	})

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
			MapperProvider: func(config *rest.Config, _ *http.Client) (meta.RESTMapper, error) {
				return thirdpartyapiutil.NewDynamicRESTMapper(config)
			},
		})
		Expect(err).NotTo(HaveOccurred())
		mgrClient = mgr.GetClient()

		By("Register controller")
		Expect((&care.Reconciler{
			Config: config.SeedCareControllerConfiguration{
				SyncPeriod: &metav1.Duration{Duration: 500 * time.Millisecond},
			},
			Namespace:      &testNamespace.Name,
			SeedName:       seedName,
			LoggingEnabled: loggingEnabled,
			ValiEnabled:    valiEnabled,
		}).AddToManager(ctx, mgr, mgr, mgr)).To(Succeed())

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

	Context("when ManagedResources for the Seed are missing", func() {
		It("should set condition to False", func() {
			By("Expect SeedSystemComponentsHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)).To(Succeed())
				return seed.Status.Conditions
			}).Should(ContainCondition(
				OfType(gardencorev1beta1.SeedSystemComponentsHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("ResourceNotFound"),
				WithMessageSubstrings("not found"),
			))
		})
	})

	Context("when ManagedResources for the Seed exist", func() {
		requiredManagedResources := []string{
			etcd.Druid,
			clusteridentity.ManagedResourceControlName,
			clusterautoscaler.ManagedResourceControlName,
			kubestatemetrics.ManagedResourceName,
			seedsystem.ManagedResourceName,
			vpa.ManagedResourceControlName,
			hvpa.ManagedResourceName,
			dependencywatchdog.ManagedResourceDependencyWatchdogWeeder,
			dependencywatchdog.ManagedResourceDependencyWatchdogProber,
			nginxingress.ManagedResourceName,
			"istio",
			"istio-system",
			prometheusoperator.ManagedResourceName,
			"prometheus-cache",
			"prometheus-seed",
			"prometheus-aggregate",
		}

		test := func(managedResourceNames []string) {
			BeforeEach(func() {
				for _, name := range managedResourceNames {
					By("Create ManagedResource for " + name)
					managedResource := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:      name,
							Namespace: testNamespace.Name,
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							SecretRefs: []corev1.LocalObjectReference{{Name: "foo-secret"}},
						},
					}
					Expect(testClient.Create(ctx, managedResource)).To(Succeed())
					log.Info("Created ManagedResource for test", "managedResource", client.ObjectKeyFromObject(managedResource))
				}
			})

			AfterEach(func() {
				for _, name := range managedResourceNames {
					By("Delete ManagedResource for " + name)
					Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: testNamespace.Name}})).To(Succeed())
				}
			})

			It("should set condition to False because all ManagedResource statuses are outdated", func() {
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

			It("should set condition to False because some ManagedResource statuses are outdated", func() {
				for _, name := range managedResourceNames[1:] {
					updateManagedResourceStatusToHealthy(name)
				}

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
				for _, name := range managedResourceNames {
					updateManagedResourceStatusToHealthy(name)
				}

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
		}

		Context("logging and vali are disabled", func() {
			test(requiredManagedResources)
		})

		Context("logging and vali are enabled", func() {
			BeforeEach(func() {
				loggingEnabled = true
				valiEnabled = true
			})

			test(append(requiredManagedResources,
				"fluent-operator",
				"fluent-operator-custom-resources",
				"fluent-bit",
				"vali",
			))
		})

		Context("alertmanager enabled", func() {
			BeforeEach(func() {
				smtpSecret := &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						GenerateName: "smtp-",
						Namespace:    seedNamespace.Name,
						Labels:       map[string]string{v1beta1constants.GardenRole: v1beta1constants.GardenRoleAlerting},
					},
					Data: map[string][]byte{"auth_type": []byte("smtp")},
				}
				Expect(testClient.Create(ctx, smtpSecret)).To(Succeed())
				DeferCleanup(func() {
					Expect(testClient.Delete(ctx, smtpSecret)).To(Succeed())
				})
			})

			test(append(requiredManagedResources, "alertmanager-seed"))
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
