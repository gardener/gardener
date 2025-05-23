// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package shoot_test

import (
	"context"
	"errors"

	"github.com/hashicorp/go-multierror"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/gardener/gardener/extensions/pkg/controller"
	extensionswebhook "github.com/gardener/gardener/extensions/pkg/webhook"
	. "github.com/gardener/gardener/extensions/pkg/webhook/shoot"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Webhook", func() {
	var (
		ctx        = context.Background()
		fakeClient client.Client
		consistOf  func(...client.Object) types.GomegaMatcher

		shootWebhookConfigs extensionswebhook.Configs

		extensionName       = "provider-test"
		managedResourceName = "extension-provider-test-shoot-webhooks"
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)

		shootWebhookConfigs = extensionswebhook.Configs{
			MutatingWebhookConfig: &admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: extensionName,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{{
					Name: "some-webhook",
				}},
			},
		}
	})

	Describe("#ReconcileWebhookConfig", func() {
		var (
			cluster   *controller.Cluster
			namespace = "extension-foo-bar"
		)

		BeforeEach(func() {
			cluster = &controller.Cluster{Shoot: &gardencorev1beta1.Shoot{}}
		})

		It("should reconcile the shoot webhook config", func() {
			Expect(ReconcileWebhookConfig(ctx, fakeClient, namespace, managedResourceName, shootWebhookConfigs, cluster, true)).To(Succeed())
			expectWebhookConfigReconciliation(ctx, fakeClient, namespace, managedResourceName, shootWebhookConfigs.MutatingWebhookConfig, consistOf)
		})
	})

	Describe("#ReconcileWebhooksForAllNamespaces", func() {
		var (
			extensionType          = "test"
			shootNamespaceSelector = map[string]string{"networking.shoot.gardener.cloud/provider": extensionType}

			namespace1 *corev1.Namespace
			namespace2 *corev1.Namespace
			namespace3 *corev1.Namespace
			namespace4 *corev1.Namespace
			namespace5 *corev1.Namespace

			cluster3 *extensionsv1alpha1.Cluster
			cluster4 *extensionsv1alpha1.Cluster
			cluster5 *extensionsv1alpha1.Cluster
		)

		BeforeEach(func() {
			namespace1 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace1",
				},
			}
			namespace2 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace2",
					Labels: map[string]string{
						"gardener.cloud/role":                      "shoot",
						"networking.shoot.gardener.cloud/provider": "foo",
					},
				},
			}
			namespace3 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace3",
					Labels: map[string]string{
						"gardener.cloud/role":                      "shoot",
						"networking.shoot.gardener.cloud/provider": extensionType,
					},
				},
			}
			namespace4 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace4",
					Labels: map[string]string{
						"gardener.cloud/role":                      "shoot",
						"networking.shoot.gardener.cloud/provider": extensionType,
					},
				},
			}
			namespace5 = &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: "namespace5",
					Labels: map[string]string{
						"gardener.cloud/role":                      "shoot",
						"networking.shoot.gardener.cloud/provider": extensionType,
					},
				},
			}

			Expect(fakeClient.Create(ctx, namespace1)).To(Succeed())
			Expect(fakeClient.Create(ctx, namespace2)).To(Succeed())
			Expect(fakeClient.Create(ctx, namespace3)).To(Succeed())
			Expect(fakeClient.Create(ctx, namespace4)).To(Succeed())
			Expect(fakeClient.Create(ctx, namespace5)).To(Succeed())

			cluster3 = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: namespace3.Name},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{},
					},
				},
			}
			cluster4 = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: namespace4.Name},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{},
					},
				},
			}
			cluster5 = &extensionsv1alpha1.Cluster{
				ObjectMeta: metav1.ObjectMeta{Name: namespace5.Name},
				Spec: extensionsv1alpha1.ClusterSpec{
					Shoot: runtime.RawExtension{
						Object: &gardencorev1beta1.Shoot{},
					},
				},
			}

			Expect(fakeClient.Create(ctx, cluster3)).To(Succeed())
			Expect(fakeClient.Create(ctx, cluster4)).To(Succeed())
			Expect(fakeClient.Create(ctx, cluster5)).To(Succeed())
		})

		It("should reconcile the webhook config for namespace3 and namespace4", func() {
			Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace3.Name, Name: managedResourceName}})).To(Succeed())
			Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace4.Name, Name: managedResourceName}})).To(Succeed())

			Expect(ReconcileWebhooksForAllNamespaces(ctx, fakeClient, managedResourceName, shootNamespaceSelector, shootWebhookConfigs)).To(Succeed())

			expectNoWebhookConfigReconciliation(ctx, fakeClient, namespace1.Name, managedResourceName)
			expectNoWebhookConfigReconciliation(ctx, fakeClient, namespace2.Name, managedResourceName)
			expectWebhookConfigReconciliation(ctx, fakeClient, namespace3.Name, managedResourceName, shootWebhookConfigs.MutatingWebhookConfig, consistOf)
			expectWebhookConfigReconciliation(ctx, fakeClient, namespace4.Name, managedResourceName, shootWebhookConfigs.MutatingWebhookConfig, consistOf)
			expectNoWebhookConfigReconciliation(ctx, fakeClient, namespace5.Name, managedResourceName)
		})

		It("should return an error because cluster for namespace3 is missing", func() {
			Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace3.Name, Name: managedResourceName}})).To(Succeed())
			Expect(fakeClient.Delete(ctx, cluster3)).To(Succeed())

			err := ReconcileWebhooksForAllNamespaces(ctx, fakeClient, managedResourceName, shootNamespaceSelector, shootWebhookConfigs)

			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(apierrors.NewNotFound(schema.GroupResource{Group: extensionsv1alpha1.SchemeGroupVersion.Group, Resource: "clusters"}, namespace3.Name))))
		})

		It("should return an error because cluster for namespace4 is does not contain shoot", func() {
			Expect(fakeClient.Create(ctx, &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Namespace: namespace4.Name, Name: managedResourceName}})).To(Succeed())

			patch := client.MergeFrom(cluster4.DeepCopy())
			cluster4.Spec.Shoot = runtime.RawExtension{}
			Expect(fakeClient.Patch(ctx, cluster4, patch)).To(Succeed())

			err := ReconcileWebhooksForAllNamespaces(ctx, fakeClient, managedResourceName, shootNamespaceSelector, shootWebhookConfigs)

			Expect(err).To(BeAssignableToTypeOf(&multierror.Error{}))
			Expect(err.(*multierror.Error).Errors).To(ConsistOf(Equal(errors.New("no shoot found in cluster resource"))))
		})
	})
})

func expectWebhookConfigReconciliation(ctx context.Context, fakeClient client.Client, namespace, managedResourceName string, shootWebhook *admissionregistrationv1.MutatingWebhookConfiguration, consistOf func(...client.Object) types.GomegaMatcher) {
	managedResource := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
	ExpectWithOffset(1, managedResource.Spec.SecretRefs).To(HaveLen(1))

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResource.Spec.SecretRefs[0].Name, Namespace: namespace}}
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
	ExpectWithOffset(1, secret.Type).To(Equal(corev1.SecretTypeOpaque))
	Expect(managedResource).To(consistOf(shootWebhook))
}

func expectNoWebhookConfigReconciliation(ctx context.Context, fakeClient client.Client, namespace, managedResourceName string) {
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: managedResourceName}, &corev1.Secret{})).To(BeNotFoundError())
	ExpectWithOffset(1, fakeClient.Get(ctx, client.ObjectKey{Namespace: namespace, Name: managedResourceName}, &resourcesv1alpha1.ManagedResource{})).To(BeNotFoundError())
}
