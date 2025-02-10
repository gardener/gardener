// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package extension_test

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	extensioncontroller "github.com/gardener/gardener/pkg/operator/controller/extension/extension"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension controller tests", func() {
	var (
		garden                         *operatorv1alpha1.Garden
		extensionBar                   *operatorv1alpha1.Extension
		extensionFoo                   *operatorv1alpha1.Extension
		managedResourceRuntimeBar      *resourcesv1alpha1.ManagedResource
		managedResourceRuntimeFoo      *resourcesv1alpha1.ManagedResource
		managedResourceRegistrationBar *resourcesv1alpha1.ManagedResource
		managedResourceRegistrationFoo *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&managedresources.IntervalWait, 100*time.Millisecond))
		DeferCleanup(test.WithVar(&extensioncontroller.RequeueGardenResourceNotReady, 100*time.Millisecond))

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:        gardenName,
				Labels:      map[string]string{testID: testRunID},
				Annotations: map[string]string{v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName: "foo-kubeconfig"},
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
						Version: "1.26.3",
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
		extensionBar = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:   "provider-bar",
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: "BackupBucket",
						Type: "bar",
					},
					{
						Kind: "Worker",
						Type: "bar",
					},
					{
						Kind: "Infrastructure",
						Type: "bar",
					},
				},
				Deployment: &operatorv1alpha1.Deployment{
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &ociRepositoryProviderLocalChart,
							},
						},
					},
				},
			},
		}
		extensionFoo = &operatorv1alpha1.Extension{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "provider-foo",
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Resources: []gardencorev1beta1.ControllerResource{
					{
						Kind: "DNSRecord",
						Type: "foo",
					},
					{
						Kind: "Worker",
						Type: "foo",
					},
					{
						Kind: "Infrastructure",
						Type: "foo",
					},
				},
				Deployment: &operatorv1alpha1.Deployment{
					AdmissionDeployment: &operatorv1alpha1.AdmissionDeploymentSpec{
						RuntimeCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &ociRepositoryAdmissionRuntimeChart,
							},
						},
						VirtualCluster: &operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &ociRepositoryAdmissionApplicationChart,
							},
						},
					},
					ExtensionDeployment: &operatorv1alpha1.ExtensionDeploymentSpec{
						DeploymentSpec: operatorv1alpha1.DeploymentSpec{
							Helm: &operatorv1alpha1.ExtensionHelm{
								OCIRepository: &ociRepositoryProviderLocalChart,
							},
						},
					},
				},
			},
		}

		managedResourceRuntimeBar = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-provider-bar-garden",
				Namespace: testNamespace.Name,
			},
		}

		managedResourceRuntimeFoo = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-provider-foo-garden",
				Namespace: testNamespace.Name,
			},
		}

		managedResourceRegistrationBar = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-registration-provider-bar",
				Namespace: testNamespace.Name,
			},
		}

		managedResourceRegistrationFoo = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "extension-registration-provider-foo",
				Namespace: testNamespace.Name,
			},
		}
	})

	It("should reconcile all required cluster resources in the virtual and runtime garden cluster", func() {
		By("Create extension bar")
		Expect(testClient.Create(ctx, extensionBar)).To(Succeed())
		log.Info("Created extension for test", "garden", extensionBar.Name)
		DeferCleanup(func() {
			By("Delete extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionBar))).To(Succeed())
			By("Ensure extension is gone")
			Eventually(func() error { return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar) }).Should(BeNotFoundError())
		})

		By("Wait until extension is reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar)).To(Succeed())
			return extensionBar.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("NoGardenFound"),
		))

		By("Create Garden")
		Expect(testClient.Create(ctx, garden)).To(Succeed())
		log.Info("Created Garden for test", "garden", garden.Name)

		DeferCleanup(func() {
			By("Delete Garden")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, garden))).To(Succeed())

			By("Forcefully remove finalizers from garden")
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, garden))).To(Succeed())

			By("Ensure Garden is gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
			}).Should(BeNotFoundError())

			By("Delete extension registration for provider-bar")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionBar))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionBar))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, managedResourceRegistrationBar))).To(Succeed())

			By("Delete extension registration for provider-foo")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, managedResourceRegistrationFoo))).To(Succeed())
		})

		By("Update Garden to ready state")
		garden.Status = operatorv1alpha1.GardenStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				LastUpdateTime: metav1.Now(),
				State:          gardencorev1beta1.LastOperationStateProcessing,
				Type:           gardencorev1beta1.LastOperationTypeReconcile,
			},
		}
		Expect(testClient.Status().Update(ctx, garden)).To(Succeed())
		garden.Status = operatorv1alpha1.GardenStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				LastUpdateTime: metav1.Now(),
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				Type:           gardencorev1beta1.LastOperationTypeReconcile,
				Progress:       100,
			},
		}
		Expect(testClient.Status().Update(ctx, garden)).To(Succeed())

		By("Wait until extension is successfully reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar)).To(Succeed())
			g.Expect(extensionBar.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionBar.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionBar.Status.Conditions))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRegistrationBar), managedResourceRegistrationBar)).To(Succeed())

		By("Create extension foo with admission controller")
		Expect(testClient.Create(ctx, extensionFoo)).To(Succeed())
		DeferCleanup(func() {
			By("Delete extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())

			By("Ensure extension is gone")
			Eventually(func() error { return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo) }).Should(BeNotFoundError())
		})

		By("Wait for admission virtual managed resource and set it as applied, healthy and not progressing")
		Eventually(func() error {
			return patchManagedResourceAsHealthyAndComplete("extension-admission-virtual-provider-foo")
		}).Should(Succeed())

		By("Wait for admission runtime managed resource and set it as applied, healthy and not progressing")
		Eventually(func() error {
			return patchManagedResourceAsHealthyAndComplete("extension-admission-runtime-provider-foo")
		}).Should(Succeed())

		By("Wait until extension is successfully reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)).To(Succeed())
			g.Expect(extensionFoo.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionFoo.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionFoo.Status.Conditions))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRegistrationFoo), managedResourceRegistrationFoo)).To(Succeed())

		By("Validate that extensions are not deployed in runtime cluster yet")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeFoo), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())

		By("Mark extension bar in runtime cluster as not required")
		Eventually(func() error {
			extensionBar.Status.Conditions = v1beta1helper.MergeConditions(extensionBar.Status.Conditions, gardencorev1beta1.Condition{
				LastUpdateTime:     metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastTransitionTime: metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Type:               "RequiredRuntime",
				Status:             "False",
			})

			return testClient.Status().Update(ctx, extensionBar)
		}).Should(Succeed())

		By("Validate that extension bar is still not deployed in runtime cluster")
		Consistently(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())

		By("Deploy extension bar in runtime cluster by marking it as required")
		Eventually(func() error {
			extensionBar.Status.Conditions = v1beta1helper.MergeConditions(extensionBar.Status.Conditions, gardencorev1beta1.Condition{
				LastUpdateTime:     metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastTransitionTime: metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Type:               "RequiredRuntime",
				Status:             "True",
			})

			return testClient.Status().Update(ctx, extensionBar)
		}).Should(Succeed())

		By("Wait for runtime managed resource and set it as applied, healthy and not progressing")
		Eventually(func() error {
			return patchManagedResourceAsHealthyAndComplete(managedResourceRuntimeBar.Name)
		}).Should(Succeed())

		By("Wait until extension is successfully reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar)).To(Succeed())
			g.Expect(extensionBar.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionBar.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionBar.Status.Conditions))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeFoo), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})).To(Succeed())

		By("Deploy extension foo in runtime cluster by marking it as required")
		Eventually(func() error {
			extensionFoo.Status.Conditions = v1beta1helper.MergeConditions(extensionFoo.Status.Conditions, gardencorev1beta1.Condition{
				LastUpdateTime:     metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastTransitionTime: metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Type:               "RequiredRuntime",
				Status:             "True",
			})
			return testClient.Status().Update(ctx, extensionFoo)
		}).Should(Succeed())

		By("Wait for runtime managed resource and set it as applied, healthy and not progressing")
		Eventually(func() error {
			return patchManagedResourceAsHealthyAndComplete(managedResourceRuntimeFoo.Name)
		}).Should(Succeed())

		By("Wait until extension is successfully reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)).To(Succeed())
			g.Expect(extensionFoo.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionFoo.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionFoo.Status.Conditions))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeFoo), &resourcesv1alpha1.ManagedResource{})).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})).To(Succeed())

		By("Delete extension bar in runtime cluster by marking it as not required")
		Eventually(func() error {
			extensionBar.Status.Conditions = v1beta1helper.MergeConditions(extensionBar.Status.Conditions, gardencorev1beta1.Condition{
				LastUpdateTime:     metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastTransitionTime: metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Type:               "RequiredRuntime",
				Status:             "False",
			})
			return testClient.Status().Update(ctx, extensionBar)
		}).Should(Succeed())

		By("Wait for runtime managed resource to be deleted")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())

		By("Wait until extension is successfully reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar)).To(Succeed())
			g.Expect(extensionBar.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionBar.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionBar.Status.Conditions))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeFoo), &resourcesv1alpha1.ManagedResource{})).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeBar), &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())

		By("Delete extension foo")
		Expect(testClient.Delete(ctx, extensionFoo)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRegistrationFoo), &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Namespace: testNamespace.Name, Name: "extension-admission-runtime-provider-foo"}, &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Namespace: testNamespace.Name, Name: "extension-admission-virtual-provider-foo"}, &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())

		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)).To(Succeed())
			g.Expect(extensionFoo.Finalizers).To(ConsistOf("gardener.cloud/operator"))
			return extensionFoo.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.ExtensionInstalled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("InstalledInRuntime"),
		), fmt.Sprintf("Failed conditions expected to be healthy:%+v", extensionFoo.Status.Conditions))

		By("Mark extension foo in runtime cluster as not required")
		Eventually(func() error {
			extensionFoo.Status.Conditions = v1beta1helper.MergeConditions(extensionFoo.Status.Conditions, gardencorev1beta1.Condition{
				LastUpdateTime:     metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				LastTransitionTime: metav1.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
				Type:               "RequiredRuntime",
				Status:             "False",
			})

			return testClient.Status().Update(ctx, extensionFoo)
		}).Should(Succeed())

		By("Wait for extension to be gone")
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(managedResourceRuntimeFoo), &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)
		}).Should(BeNotFoundError())

		By("Delete garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(BeNotFoundError())
	})
})

func patchManagedResourceAsHealthyAndComplete(name string) error {
	mr := &resourcesv1alpha1.ManagedResource{}
	if err := testClient.Get(ctx, client.ObjectKey{Namespace: testNamespace.Name, Name: name}, mr); err != nil {
		return err
	}
	mr.Status.Conditions = []gardencorev1beta1.Condition{
		{Type: resourcesv1alpha1.ResourcesApplied, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesHealthy, Status: gardencorev1beta1.ConditionTrue, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
		{Type: resourcesv1alpha1.ResourcesProgressing, Status: gardencorev1beta1.ConditionFalse, LastTransitionTime: metav1.Now(), LastUpdateTime: metav1.Now()},
	}
	mr.Status.ObservedGeneration = mr.Generation
	return testClient.Status().Update(ctx, mr)
}
