// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package care_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension Care controller tests", func() {
	var (
		garden    *operatorv1alpha1.Garden
		extension *operatorv1alpha1.Extension
	)

	BeforeEach(func() {
		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:   gardenName,
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     []string{"10.1.0.0/16"},
						Services: []string{"10.2.0.0/16"},
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "ingress.runtime-garden.local.gardener.cloud"}},
						Controller: gardencorev1beta1.IngressController{
							Kind: "nginx",
						},
					},
					Provider: operatorv1alpha1.Provider{
						Zones: []string{"a", "b", "c"},
					},
					Settings: &operatorv1alpha1.Settings{
						VerticalPodAutoscaler: &operatorv1alpha1.SettingVerticalPodAutoscaler{
							Enabled: ptr.To(true),
						},
					},
				},
				VirtualCluster: operatorv1alpha1.VirtualCluster{
					DNS: operatorv1alpha1.DNS{
						Domains: []operatorv1alpha1.DNSDomain{{Name: "virtual-garden.local.gardener.cloud"}},
					},
					Gardener: operatorv1alpha1.Gardener{
						ClusterIdentity: "test",
					},
					Kubernetes: operatorv1alpha1.Kubernetes{
						Version: "1.31.1",
					},
					Maintenance: operatorv1alpha1.Maintenance{
						TimeWindow: gardencorev1beta1.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Networking: operatorv1alpha1.Networking{
						Services: []string{"100.64.0.0/13"},
					},
				},
			},
		}

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(testClient.Delete(ctx, garden)).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())
		})
	})

	Context("when Extension exists", func() {
		extensionName := "foo"
		managedResourceName := "extension-foo-garden"

		BeforeEach(func() {
			By("Create Extension")
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
			}
			Expect(testClient.Create(ctx, extension)).To(Succeed())
			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))

			By("Set Extension to status required")
			now := metav1.Now()
			extension.Status = operatorv1alpha1.ExtensionStatus{
				Conditions: []gardencorev1beta1.Condition{
					{Type: operatorv1alpha1.ExtensionRequiredRuntime, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: now, LastUpdateTime: now},
				},
			}
			Expect(testClient.Status().Update(ctx, extension)).To(Succeed())

			DeferCleanup(func() {
				By("Delete Extension")
				Expect(testClient.Delete(ctx, &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: extensionName}})).To(Succeed())
			})

			By("Create ManagedResource for runtime cluster")
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
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResource))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension in runtime cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: testNamespace.Name}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceName)

			By("Expect ExtensionHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ExtensionComponentsRunning"),
				WithMessageSubstrings("All extension components are healthy."),
			))
		})
	})

	Context("when Extension admission exists", func() {
		extensionName := "bar"
		managedResourceRuntimeName := "extension-admission-runtime-bar"
		managedResourceVirtualName := "extension-admission-virtual-bar"

		BeforeEach(func() {
			By("Create Extension")
			extension = &operatorv1alpha1.Extension{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
				Spec: operatorv1alpha1.ExtensionSpec{
					Deployment: &operatorv1alpha1.Deployment{
						AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{},
					},
				},
			}
			Expect(testClient.Create(ctx, extension)).To(Succeed())
			log.Info("Created Extension for test", "extension", client.ObjectKeyFromObject(extension))

			DeferCleanup(func() {
				By("Delete Extension")
				Expect(testClient.Delete(ctx, &operatorv1alpha1.Extension{ObjectMeta: metav1.ObjectMeta{Name: extensionName}})).To(Succeed())
			})

			By("Create ManagedResource for runtime cluster")
			managedResourceRuntime := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceRuntimeName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					Class:      ptr.To("seed"),
					SecretRefs: []corev1.LocalObjectReference{{Name: "bar-runtime-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResourceRuntime)).To(Succeed())
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResourceRuntime))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension admission in runtime cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceRuntimeName, Namespace: testNamespace.Name}})).To(Succeed())
			})

			By("Create ManagedResource for virtual cluster")
			managedResourceVirtual := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceVirtualName,
					Namespace: testNamespace.Name,
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					SecretRefs: []corev1.LocalObjectReference{{Name: "bar-virtual-secret"}},
				},
			}
			Expect(testClient.Create(ctx, managedResourceVirtual)).To(Succeed())
			log.Info("Created ManagedResource", "managedResource", client.ObjectKeyFromObject(managedResourceVirtual))

			DeferCleanup(func() {
				By("Delete ManagedResource for extension admission in virtual cluster")
				Expect(testClient.Delete(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceVirtualName, Namespace: testNamespace.Name}})).To(Succeed())
			})
		})

		It("should set condition to False because all ManagedResource statuses are outdated", func() {
			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to False because status of virtual ManagedResource statuses is outdated", func() {
			updateManagedResourceStatusToHealthy(managedResourceRuntimeName)

			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to False because status of runtime ManagedResource statuses is outdated", func() {
			updateManagedResourceStatusToHealthy(managedResourceVirtualName)

			By("Expect ExtensionHealthy condition to be False")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionFalse),
				WithReason("OutdatedStatus"),
				WithMessageSubstrings("observed generation of managed resource"),
			))
		})

		It("should set condition to True because all ManagedResource statuses are healthy", func() {
			updateManagedResourceStatusToHealthy(managedResourceVirtualName)
			updateManagedResourceStatusToHealthy(managedResourceRuntimeName)

			By("Expect ExtensionHealthy condition to be True")
			Eventually(func(g Gomega) []gardencorev1beta1.Condition {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extension), extension)).To(Succeed())
				return extension.Status.Conditions
			}).Should(ContainCondition(
				OfType(operatorv1alpha1.ExtensionAdmissionHealthy),
				WithStatus(gardencorev1beta1.ConditionTrue),
				WithReason("ExtensionAdmissionComponentsRunning"),
				WithMessageSubstrings("All extension admission components are healthy."),
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
