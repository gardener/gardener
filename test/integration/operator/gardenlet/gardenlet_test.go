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
	var (
		gardenlet           *seedmanagementv1alpha1.Gardenlet
		gardenletDeployment *appsv1.Deployment
		seed                *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					"bar": "baz",
				},
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
						Type: "provider",
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
		gardenletConfig, err := encoding.EncodeGardenletConfiguration(&gardenletconfigv1alpha1.GardenletConfiguration{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gardenletconfigv1alpha1.SchemeGroupVersion.String(),
				Kind:       "GardenletConfiguration",
			},
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: seed.ObjectMeta,
					Spec:       seed.Spec,
				},
			},
		})
		Expect(err).NotTo(HaveOccurred())

		gardenlet = &seedmanagementv1alpha1.Gardenlet{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "seed-",
				Namespace:    testNamespace.Name,
				Labels:       map[string]string{testID: testRunID},
			},
			Spec: seedmanagementv1alpha1.GardenletSpec{
				Deployment: seedmanagementv1alpha1.GardenletSelfDeployment{
					Helm: seedmanagementv1alpha1.GardenletHelm{OCIRepository: ociRepository},
					GardenletDeployment: seedmanagementv1alpha1.GardenletDeployment{
						ReplicaCount:         ptr.To[int32](1),
						RevisionHistoryLimit: ptr.To[int32](3),
						Image: &seedmanagementv1alpha1.Image{
							PullPolicy: ptr.To(corev1.PullIfNotPresent),
						},
					},
				},
				Config: *gardenletConfig,
			},
		}

		gardenletDeployment = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "gardenlet", Namespace: testNamespace.Name}}
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

			By("Delete and wait for Gardenlet deployment to be gone")
			Eventually(func(g Gomega) {
				g.Expect(testClient.Delete(ctx, gardenletDeployment)).To(Or(Succeed(), BeNotFoundError()))
				g.Expect(mgrClient.Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)).Should(BeNotFoundError())
			}).Should(Succeed())
		})
	})

	verifyGardenletDeployment := func(seedRegistered bool) {
		By("Gardenlet status should reflect successful reconciliation")

		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenlet), gardenlet)).To(Succeed())
			g.Expect(gardenlet.Spec.Deployment.PodLabels).To(HaveKeyWithValue("networking.resources.gardener.cloud/to-virtual-garden-kube-apiserver-tcp-443", "allowed"))
			g.Expect(gardenlet.Status.ObservedGeneration).To(Equal(gardenlet.Generation))
			condition := v1beta1helper.GetCondition(gardenlet.Status.Conditions, seedmanagementv1alpha1.SeedRegistered)
			g.Expect(condition).NotTo(BeNil())
			if seedRegistered {
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconciled))
			} else {
				g.Expect(condition.Status).To(Equal(gardencorev1beta1.ConditionProgressing))
				g.Expect(condition.Reason).To(Equal(gardencorev1beta1.EventReconcileError))
			}
		}).Should(Succeed())

		By("Verify that gardenlet is deployed")
		EventuallyWithOffset(1, func(g Gomega) {
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRole{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:apiserver-sni"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet:managed-istio"}, &rbacv1.ClusterRoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener-system-critical"}, &schedulingv1.PriorityClass{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: testNamespace.Name}, &rbacv1.Role{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardener.cloud:system:gardenlet", Namespace: testNamespace.Name}, &rbacv1.RoleBinding{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet-kubeconfig-bootstrap", Namespace: testNamespace.Name}, &corev1.Secret{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: testNamespace.Name}, &corev1.Service{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: testNamespace.Name}, &corev1.ServiceAccount{})).To(Succeed())
			g.Expect(testClient.Get(ctx, client.ObjectKey{Name: "gardenlet", Namespace: testNamespace.Name}, &appsv1.Deployment{})).To(Succeed())
		}).Should(Succeed())
	}

	When("Seed object does not exist yet", func() {
		It("should deploy gardenlet and update it on request", func() {
			verifyGardenletDeployment(false)

			By("Update some value")
			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Spec.Deployment.RevisionHistoryLimit = ptr.To[int32](1337)
			Expect(testClient.Patch(ctx, gardenlet, patch)).To(Succeed())

			By("Verify that value change was rolled out")
			Eventually(func(g Gomega) *int32 {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)).To(Succeed())
				return gardenletDeployment.Spec.RevisionHistoryLimit
			}).Should(PointTo(Equal(int32(1337))))
		})
	})

	When("Seed object gets created", func() {
		It("should deploy gardenlet and no longer touch it when Seed object got created", func() {
			verifyGardenletDeployment(false)

			By("Create Seed") // gardenlet would do this typically, but it doesn't run in this setup
			seed.Name = gardenlet.Name
			Expect(testClient.Create(ctx, seed)).To(Succeed())

			verifyGardenletDeployment(true)

			Eventually(func() error {
				return mgrClient.Get(ctx, client.ObjectKeyFromObject(seed), seed)
			}).Should(Succeed())

			By("Update some value")
			patch := client.MergeFrom(gardenlet.DeepCopy())
			gardenlet.Spec.Deployment.RevisionHistoryLimit = ptr.To[int32](1337)
			Expect(testClient.Patch(ctx, gardenlet, patch)).To(Succeed())

			By("Verify that value change was not rolled out")
			Eventually(func(g Gomega) *int32 {
				g.Expect(testClient.Get(ctx, client.ObjectKeyFromObject(gardenletDeployment), gardenletDeployment)).To(Succeed())
				return gardenletDeployment.Spec.RevisionHistoryLimit
			}).Should(PointTo(Equal(int32(3))))
		})
	})
})
