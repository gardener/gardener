// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardenlet_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Gardenlet controller test", func() {
	var gardenlet *seedmanagementv1alpha1.Gardenlet

	BeforeEach(func() {
		gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				KubeconfigSecret: &corev1.SecretReference{
					Name:      "gardenlet-kubeconfig",
					Namespace: gardenNamespaceSeed.Name,
				},
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"bar": "baz",
						},
					},
					Spec: gardencorev1beta1.SeedSpec{
						Backup: &gardencorev1beta1.SeedBackup{
							Provider: "test",
							Region:   ptr.To("bar"),
						},
					},
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      seed.Name,
				Namespace: "garden", // must be in the garden namespace
				Labels:    map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.GardenletSpec{
				Deployment: seedmanagementv1alpha1.GardenletSelfDeployment{
					Helm: seedmanagementv1alpha1.GardenletHelm{OCIRepository: ociRepository},
					GardenletDeployment: seedmanagementv1alpha1.GardenletDeployment{
						ReplicaCount:         ptr.To[int32](1),
						RevisionHistoryLimit: ptr.To[int32](1),
						Image: &seedmanagementv1alpha1.Image{
							PullPolicy: ptr.To(corev1.PullIfNotPresent),
						},
					},
				},
				Config: *gardenletConfig,
			},
		}
	})

	JustBeforeEach(func() {
		By("Create Gardenlet")
		Expect(testClient.Create(ctx, gardenlet)).To(Succeed())
		log.Info("Created Gardenlet for test", "gardenlet", client.ObjectKeyFromObject(gardenlet))

		DeferCleanup(func() {
			By("Delete Gardenlet")
			Expect(testClient.Delete(ctx, gardenlet)).To(Or(Succeed(), BeNotFoundError()))

			By("Wait for Gardenlet to be gone")
			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)
			}).Should(BeNotFoundError())
		})
	})

	It("should set the GardenletReconciled status to true, create seed secrets specified in spec.backup.secretRef and spec.secretRef field of seed template and deploy gardenlet", func() {
		By("Gardenlet status should reflect successful reconciliation")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			g.Expect(gardenlet.Status.ObservedGeneration).To(Equal(gardenlet.Generation))
			condition := v1beta1helper.GetCondition(gardenlet.Status.Conditions, seedmanagementv1alpha1.GardenletReconciled)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
		}).Should(Succeed())

		By("Verify that gardenlet is deployed")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener-system-critical"}, &schedulingv1.PriorityClass{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespaceSeed.Name}, &rbacv1.Role{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: gardenNamespaceSeed.Name}, &rbacv1.RoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-kubeconfig-bootstrap", Namespace: gardenNamespaceSeed.Name}, &corev1.Secret{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceSeed.Name}, &corev1.Service{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceSeed.Name}, &corev1.ServiceAccount{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: gardenNamespaceSeed.Name}, &appsv1.Deployment{})).To(Succeed())
		}).Should(Succeed())

		By("Update some value")
		patch := client.MergeFrom(gardenlet.DeepCopy())
		gardenlet.Spec.Deployment.RevisionHistoryLimit = ptr.To[int32](1337)
		Expect(testClient.Patch(ctx, gardenlet, patch)).To(Succeed())

		By("Gardenlet status should reflect successful reconciliation")
		Eventually(func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			g.Expect(gardenlet.Status.ObservedGeneration).To(Equal(gardenlet.Generation))
			condition := v1beta1helper.GetCondition(gardenlet.Status.Conditions, seedmanagementv1alpha1.GardenletReconciled)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
		}).Should(Succeed())

		By("Verify that value change was rolled out")
		Eventually(func(g Gomega) *int32 {
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: gardenNamespaceSeed.Name}}
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(deployment), deployment)).To(Succeed())
			return deployment.Spec.RevisionHistoryLimit
		}).Should(PointTo(Equal(int32(1337))))
	})
})
