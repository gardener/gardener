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

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension controller tests", func() {
	var (
		garden                    *operatorv1alpha1.Garden
		extensionBar              *operatorv1alpha1.Extension
		extensionFoo              *operatorv1alpha1.Extension
		controllerDeploymentBar   *gardencorev1.ControllerDeployment
		controllerRegistrationBar *gardencorev1beta1.ControllerRegistration
		controllerDeploymentFoo   *gardencorev1.ControllerDeployment
		controllerRegistrationFoo *gardencorev1beta1.ControllerRegistration
	)

	BeforeEach(func() {
		DeferCleanup(test.WithVar(&managedresources.IntervalWait, 100*time.Millisecond))

		garden = &operatorv1alpha1.Garden{
			ObjectMeta: metav1.ObjectMeta{
				Name:        gardenName,
				Labels:      map[string]string{testID: testRunID},
				Annotations: map[string]string{v1beta1constants.AnnotationKeyGenericTokenKubeconfigSecretName: "foo-kubeconfig"},
			},
			Spec: operatorv1alpha1.GardenSpec{
				RuntimeCluster: operatorv1alpha1.RuntimeCluster{
					Networking: operatorv1alpha1.RuntimeNetworking{
						Pods:     "10.1.0.0/16",
						Services: "10.2.0.0/16",
					},
					Ingress: operatorv1alpha1.Ingress{
						Domains: []string{"ingress.runtime-garden.local.gardener.cloud"},
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
						Domains: []string{"virtual-garden.local.gardener.cloud"},
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
						Services: "100.64.0.0/13",
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
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: ptr.To("bar"),
								},
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
								OCIRepository: &gardencorev1.OCIRepository{
									Ref: ptr.To("foo"),
								},
							},
						},
					},
				},
			},
		}

		controllerDeploymentBar = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-bar",
			},
		}
		controllerRegistrationBar = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-bar",
			},
		}

		controllerDeploymentFoo = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-foo",
			},
		}
		controllerRegistrationFoo = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-foo",
			},
		}
	})

	It("should reconcile virtual cluster resources", func() {
		By("Create extension bar")
		Expect(testClient.Create(ctx, extensionBar)).To(Succeed())
		log.Info("Created extension for test", "garden", extensionBar.Name)
		DeferCleanup(func() {
			By("Delete extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionBar))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionBar))).To(Succeed())
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

			By("Delete controller-{registration,deployment} for provider-local")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionBar))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerRegistrationBar))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerDeploymentBar))).To(Succeed())

			By("Delete controller-{registration,deployment} for provider-foo")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerRegistrationFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, controllerDeploymentFoo))).To(Succeed())
		})

		By("Update Garden to ready state")
		garden.Status = operatorv1alpha1.GardenStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				LastUpdateTime: metav1.Now(),
				State:          gardencorev1beta1.LastOperationStateProcessing,
				Type:           gardencorev1beta1.LastOperationTypeCreate,
			},
		}
		Expect(testClient.Status().Update(ctx, garden)).To(Succeed())
		garden.Status = operatorv1alpha1.GardenStatus{
			LastOperation: &gardencorev1beta1.LastOperation{
				LastUpdateTime: metav1.Now(),
				State:          gardencorev1beta1.LastOperationStateSucceeded,
				Type:           gardencorev1beta1.LastOperationTypeCreate,
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

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistrationBar), controllerRegistrationBar)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeploymentBar), controllerDeploymentBar)).To(Succeed())

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

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistrationFoo), controllerRegistrationFoo)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeploymentFoo), controllerDeploymentFoo)).To(Succeed())

		By("Delete extension foo")
		Expect(testClient.Delete(ctx, extensionFoo)).To(Succeed())

		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(controllerRegistrationFoo), controllerRegistrationFoo)
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(controllerDeploymentFoo), controllerDeploymentFoo)
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Namespace: testNamespace.Name, Name: "extension-admission-virtual-provider-foo"}, &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKey{Namespace: testNamespace.Name, Name: "extension-admission-runtime-provider-foo"}, &resourcesv1alpha1.ManagedResource{})
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)
		}).Should(BeNotFoundError())

		By("Delete garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())
		Eventually(func() error {
			return testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden)
		}).Should(BeNotFoundError())

		By("Verify extension bar has no finalizers")
		Eventually(func() ([]string, error) {
			err := mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionBar), extensionBar)
			if err != nil {
				return nil, err
			}
			return extensionBar.Finalizers, nil
		}).Should(BeEmpty())
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
