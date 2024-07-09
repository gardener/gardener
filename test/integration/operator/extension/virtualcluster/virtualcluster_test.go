// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package virtualcluster_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Extension controller tests", func() {
	var (
		garden         *operatorv1alpha1.Garden
		extensionLocal *operatorv1alpha1.Extension
		extensionFoo   *operatorv1alpha1.Extension
		ctrlDepLocal   *gardencorev1.ControllerDeployment
		ctrlRegLocal   *gardencorev1beta1.ControllerRegistration
		ctrlDepFoo     *gardencorev1.ControllerDeployment
		ctrlRegFoo     *gardencorev1beta1.ControllerRegistration
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
		extensionLocal = &operatorv1alpha1.Extension{
			ObjectMeta: metav1.ObjectMeta{
				Name:   extensionName,
				Labels: map[string]string{testID: testRunID},
			},
		}
		extensionFoo = &operatorv1alpha1.Extension{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:   "provider-foo",
				Labels: map[string]string{testID: testRunID},
			},
			Spec: operatorv1alpha1.ExtensionSpec{
				Deployment: &operatorv1alpha1.Deployment{
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

		ctrlDepLocal = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-local",
			},
		}
		ctrlRegLocal = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-local",
			},
		}

		ctrlDepFoo = &gardencorev1.ControllerDeployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-foo",
			},
		}
		ctrlRegFoo = &gardencorev1beta1.ControllerRegistration{
			ObjectMeta: metav1.ObjectMeta{
				Name: "provider-foo",
			},
		}
	})

	It("should reconcile virtual cluster resources", func() {
		By("Create well-known extension local")
		Expect(testClient.Create(ctx, extensionLocal)).To(Succeed())
		log.Info("Created extension for test", "garden", extensionLocal.Name)
		DeferCleanup(func() {
			By("Delete extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionLocal))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionLocal))).To(Succeed())
			By("Ensure extension is gone")
			Eventually(func() error { return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionLocal), extensionLocal) }).Should(BeNotFoundError())
		})

		By("Wait until extension is reconciled")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionLocal), extensionLocal)).To(Succeed())
			return extensionLocal.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.VirtualClusterExtensionReconciled),
			WithStatus(gardencorev1beta1.ConditionFalse),
			WithReason("NoGardenFound"),
		))

		By("Verify extension is defaulted")
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionLocal), extensionLocal)).To(Succeed())
		Expect(extensionLocal.Spec.Resources).ToNot(BeEmpty())

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
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionLocal))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, ctrlRegLocal))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, ctrlDepLocal))).To(Succeed())

			By("Delete controller-{registration,deployment} for provider-foo")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, ctrlRegFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, ctrlDepFoo))).To(Succeed())
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

		By("Wait until provider-local virtual cluster resources are ready")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionLocal), extensionLocal)).To(Succeed())
			return extensionLocal.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.VirtualClusterExtensionReconciled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ctrlRegLocal), ctrlRegLocal)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ctrlDepLocal), ctrlDepLocal)).To(Succeed())

		By("Install another extension")
		Expect(testClient.Create(ctx, extensionFoo)).To(Succeed())
		DeferCleanup(func() {
			By("Delete extension")
			Expect(client.IgnoreNotFound(testClient.Delete(ctx, extensionFoo))).To(Succeed())
			Expect(client.IgnoreNotFound(controllerutils.RemoveAllFinalizers(ctx, testClient, extensionFoo))).To(Succeed())

			By("Ensure extension is gone")
			Eventually(func() error { return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo) }).Should(BeNotFoundError())
		})

		By("Wait until provider-foo virtual cluster resources are ready")
		Eventually(func(g Gomega) []gardencorev1beta1.Condition {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)).To(Succeed())
			return extensionFoo.Status.Conditions
		}).Should(ContainCondition(
			OfType(operatorv1alpha1.VirtualClusterExtensionReconciled),
			WithStatus(gardencorev1beta1.ConditionTrue),
			WithReason("ReconcileSuccessful"),
		))

		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ctrlRegFoo), ctrlRegFoo)).To(Succeed())
		Expect(testClient.Get(ctx, client.ObjectKeyFromObject(ctrlDepFoo), ctrlDepFoo)).To(Succeed())

		By("Delete extension foo")
		Expect(testClient.Delete(ctx, extensionFoo)).To(Succeed())

		Eventually(func() error {
			return (testClient.Get(ctx, client.ObjectKeyFromObject(ctrlRegFoo), ctrlRegFoo))
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return (testClient.Get(ctx, client.ObjectKeyFromObject(ctrlDepFoo), ctrlDepFoo))
		}).Should(BeNotFoundError())
		Eventually(func() error {
			return mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionFoo), extensionFoo)
		}).Should(BeNotFoundError())

		By("Delete garden")
		Expect(testClient.Delete(ctx, garden)).To(Succeed())
		Eventually(func() error {
			return (testClient.Get(ctx, client.ObjectKeyFromObject(garden), garden))
		}).Should(BeNotFoundError())

		By("Verify provider-local has no finalizers")
		Eventually(func() ([]string, error) {
			err := mgrClient.Get(ctx, client.ObjectKeyFromObject(extensionLocal), extensionLocal)
			if err != nil {
				return nil, err
			}
			return extensionLocal.Finalizers, nil
		}).Should(BeEmpty())
	})
})