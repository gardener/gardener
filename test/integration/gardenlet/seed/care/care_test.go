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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusterautoscaler"
	"github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	"github.com/gardener/gardener/pkg/operation/botanist/component/dependencywatchdog"
	"github.com/gardener/gardener/pkg/operation/botanist/component/etcd"
	"github.com/gardener/gardener/pkg/operation/botanist/component/hvpa"
	"github.com/gardener/gardener/pkg/operation/botanist/component/istio"
	"github.com/gardener/gardener/pkg/operation/botanist/component/kubestatemetrics"
	"github.com/gardener/gardener/pkg/operation/botanist/component/nginxingress"
	"github.com/gardener/gardener/pkg/operation/botanist/component/seedsystem"
	"github.com/gardener/gardener/pkg/operation/botanist/component/vpa"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Seed Care controller tests", func() {
	var (
		requiredManagedResources = []string{
			etcd.Druid,
			clusteridentity.ManagedResourceControlName,
			clusterautoscaler.ManagedResourceControlName,
			kubestatemetrics.ManagedResourceName,
			seedsystem.ManagedResourceName,
			vpa.ManagedResourceControlName,
			istio.ManagedResourceControlName,
			istio.ManagedResourceIstioSystemName,
			hvpa.ManagedResourceName,
			dependencywatchdog.ManagedResourceDependencyWatchdogEndpoint,
			dependencywatchdog.ManagedResourceDependencyWatchdogProbe,
			nginxingress.ManagedResourceName,
		}

		seed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
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
					Nodes:    pointer.String("10.2.0.0/16"),
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
		BeforeEach(func() {
			for _, name := range requiredManagedResources {
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
			for _, name := range requiredManagedResources {
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
			for _, name := range requiredManagedResources[1:] {
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
			for _, name := range requiredManagedResources {
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
