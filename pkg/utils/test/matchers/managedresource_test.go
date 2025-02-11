// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package matchers_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ManagedResource Object Matcher", func() {
	var (
		ctx        context.Context
		scheme     *runtime.Scheme
		fakeClient client.Client

		resourceName, resourceNamespace string
		configMap                       *corev1.ConfigMap
		secret                          *corev1.Secret
		deployment                      *appsv1.Deployment

		managedResource                                *resourcesv1alpha1.ManagedResource
		managedResourceSecret1, managedResourceSecret2 *corev1.Secret
	)

	BeforeEach(func() {
		ctx = context.Background()
		scheme = runtime.NewScheme()
		schemeBuilder := runtime.NewSchemeBuilder(kubernetesscheme.AddToScheme, resourcesv1alpha1.AddToScheme)
		Expect(schemeBuilder.AddToScheme(scheme)).To(Succeed())

		fakeClient = fakeclient.NewClientBuilder().WithScheme(scheme).Build()

		resourceName = "test"
		resourceNamespace = "default"

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
			Data: map[string]string{
				"key": "value",
			},
		}
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
			Data: map[string][]byte{
				"key": []byte("value"),
			},
		}
		deployment = &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: ptr.To[int32](2),
				Paused:   true,
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{Name: "gardener"},
						},
					},
				},
			},
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resourceName,
				Namespace: resourceNamespace,
			},
		}
		managedResourceSecret1 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret1",
				Namespace: resourceNamespace,
			},
		}
		managedResourceSecret2 = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secret2",
				Namespace: resourceNamespace,
			},
		}
	})

	setupManagedResource := func() {
		configMapYAML, err := kubernetesutils.Serialize(configMap, fakeClient.Scheme())
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		deploymentYAML, err := kubernetesutils.Serialize(deployment, fakeClient.Scheme())
		ExpectWithOffset(1, err).NotTo(HaveOccurred())
		secretYAML, err := kubernetesutils.Serialize(secret, fakeClient.Scheme())
		ExpectWithOffset(1, err).NotTo(HaveOccurred())

		managedResourceSecret1.Data = map[string][]byte{
			fmt.Sprintf("configmap__%s__%s.yaml", configMap.Namespace, configMap.Name): []byte(configMapYAML),
		}

		managedResourceSecret2.Data = map[string][]byte{
			fmt.Sprintf("deployment__%s__%s.yaml", deployment.Namespace, deployment.Name): []byte(deploymentYAML),
			fmt.Sprintf("secret__%s__%s.yaml", secret.Namespace, secret.Name):             []byte(secretYAML),
		}

		managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{
			{Name: managedResourceSecret1.Name},
			{Name: managedResourceSecret2.Name},
		}

		ExpectWithOffset(1, fakeClient.Create(ctx, managedResource)).To(Succeed())
		ExpectWithOffset(1, fakeClient.Create(ctx, managedResourceSecret1)).To(Succeed())
		ExpectWithOffset(1, fakeClient.Create(ctx, managedResourceSecret2)).To(Succeed())
	}

	commonTests := func(matcherFn func(client.Client) func(...client.Object) types.GomegaMatcher) {
		var matcher func(...client.Object) types.GomegaMatcher

		BeforeEach(func() {
			matcher = matcherFn(fakeClient)
		})

		Context("without managed resource", func() {
			It("should not find an object", func() {
				ExpectWithOffset(1, nil).NotTo(matcher(&corev1.Secret{}))
			})
		})

		Context("with managed resource", func() {
			Context("with secrets references", func() {
				BeforeEach(func() {
					setupManagedResource()
				})

				It("should only find contained resources", func() {
					ExpectWithOffset(1, managedResource).To(matcher(configMap, deployment, secret))

					deploymentModified := deployment.DeepCopy()
					deploymentModified.Spec.MinReadySeconds += 1
					ExpectWithOffset(1, managedResource).NotTo(matcher(deploymentModified))
				})
			})

			Context("without secret references", func() {
				BeforeEach(func() {
					ExpectWithOffset(1, fakeClient.Create(ctx, managedResource)).To(Succeed())
				})

				It("should not find an object", func() {
					ExpectWithOffset(1, managedResource).NotTo(matcher(&corev1.Secret{}))
				})
			})
		})
	}

	Describe("ContainsObject functionality", func() {
		var containObjects func(...client.Object) types.GomegaMatcher

		BeforeEach(func() {
			containObjects = NewManagedResourceContainsObjectsMatcher(fakeClient)
		})

		commonTests(NewManagedResourceContainsObjectsMatcher)

		It("should succeed on finding partial objects", func() {
			setupManagedResource()

			Expect(managedResource).To(containObjects(secret, configMap))
		})
	})

	Describe("ConsistOf functionality", func() {
		var containObjects func(...client.Object) types.GomegaMatcher

		BeforeEach(func() {
			containObjects = NewManagedResourceConsistOfObjectsMatcher(fakeClient)
		})

		commonTests(NewManagedResourceConsistOfObjectsMatcher)

		It("should fail because extra elements are found", func() {
			setupManagedResource()

			Expect(managedResource).NotTo(containObjects(secret, configMap))
		})
	})
})
