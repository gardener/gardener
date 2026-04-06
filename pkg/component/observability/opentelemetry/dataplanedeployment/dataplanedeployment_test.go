// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package dataplanedeployment_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/component"
	. "github.com/gardener/gardener/pkg/component/observability/opentelemetry/dataplanedeployment"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var _ = Describe("DataplaneDeployment", func() {
	var (
		ctx = context.Background()

		managedResourceName = "shoot-core-otel-collector-dataplane"
		namespace           = "some-namespace"
		image               = "some-otel-image:some-tag"

		c         client.Client
		config    Values
		component component.DeployWaiter

		managedResource       *resourcesv1alpha1.ManagedResource
		managedResourceSecret *corev1.Secret
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		config = Values{
			Image:    image,
			Replicas: 1,
		}

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "managedresource-" + managedResource.Name,
				Namespace: managedResource.Namespace,
			},
		}
	})

	Describe("#Deploy", func() {
		JustBeforeEach(func() {
			component = New(c, namespace, config)
			// why do we initiall set managedResource but overwrite it here?
			// we haven't added anything to the fake client
			// what is the state of the managedResource object afterwards
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

			Expect(component.Deploy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
			expectedMr := &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:            managedResourceName,
					Namespace:       namespace,
					ResourceVersion: "1",
					Labels:          map[string]string{"origin": "gardener"},
				},
				Spec: resourcesv1alpha1.ManagedResourceSpec{
					InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
					KeepObjects:  ptr.To(false),
					SecretRefs: []corev1.LocalObjectReference{{
						Name: managedResource.Spec.SecretRefs[0].Name,
					}},
				},
			}
			utilruntime.Must(references.InjectAnnotations(expectedMr))
			Expect(managedResource).To(DeepEqual(expectedMr))

			// EXPLAIN: is this really the pattern used throughout other tests? To reuse the same object used for
			// generating the object key for the GET request?
			managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
			Expect(managedResourceSecret.Type).To(Equal(corev1.SecretTypeOpaque))
			Expect(managedResourceSecret.Immutable).To(Equal(ptr.To(true)))
			Expect(managedResourceSecret.Labels["resources.gardener.cloud/garbage-collectable-reference"]).To(Equal("true"))
		})

		It("should successfully deploy all resources", func() {
			manifests, err := test.ExtractManifestsFromManagedResourceData(managedResourceSecret.Data)
			Expect(err).NotTo(HaveOccurred())
			Expect(manifests).To(ContainElement(ContainSubstring("kind: ServiceAccount")))
			Expect(manifests).To(ContainElement(ContainSubstring("kind: ClusterRole\n")))
			Expect(manifests).To(ContainElement(ContainSubstring("kind: ClusterRoleBinding")))
			Expect(manifests).To(ContainElement(ContainSubstring("kind: Service\n")))
			Expect(manifests).To(ContainElement(ContainSubstring("kind: ConfigMap")))
			Expect(manifests).To(ContainElement(ContainSubstring("receivers:")))
			Expect(manifests).To(ContainElement(ContainSubstring("prometheus:")))
			Expect(manifests).To(ContainElement(ContainSubstring("kind: Deployment")))
			Expect(manifests).To(ContainElement(ContainSubstring(image)))
			Expect(manifests).To(ContainElement(ContainSubstring("gardener-shoot-system-700")))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully destroy all resources", func() {
			component = New(c, namespace, config)
			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(c.Create(ctx, managedResourceSecret)).To(Succeed())

			Expect(component.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
		})
	})

	Describe("#Wait", func() {
		It("should fail because reading the ManagedResource fails", func() {
			component = New(c, namespace, config)

			Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
		})

		It("should fail because the ManagedResource is unhealthy", func() {
			component = New(c, namespace, config)

			Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       managedResourceName,
					Namespace:  namespace,
					Generation: 1,
				},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionFalse,
						},
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionFalse,
						},
					},
				},
			})).To(Succeed())

			Expect(component.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should succeed because the ManagedResource is healthy and progressing", func() {
			component = New(c, namespace, config)

			Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:       managedResourceName,
					Namespace:  namespace,
					Generation: 1,
				},
				Status: resourcesv1alpha1.ManagedResourceStatus{
					ObservedGeneration: 1,
					Conditions: []gardencorev1beta1.Condition{
						{
							Type:   resourcesv1alpha1.ResourcesApplied,
							Status: gardencorev1beta1.ConditionTrue,
						},
						{
							Type:   resourcesv1alpha1.ResourcesHealthy,
							Status: gardencorev1beta1.ConditionTrue,
						},
						{
							Type:   resourcesv1alpha1.ResourcesProgressing,
							Status: gardencorev1beta1.ConditionTrue,
						},
					},
				},
			})).To(Succeed())

			Expect(component.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should fail when the wait for the managed resource deletion fails", func() {
			component = New(c, namespace, config)

			Expect(c.Create(ctx, managedResource)).To(Succeed())
			Expect(component.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		// WaitCleanup should return success if the resources didn't exist in the first place.
		It("should not return an error when it is already removed", func() {
			component = New(c, namespace, config)

			Expect(component.WaitCleanup(ctx)).To(Succeed())
		})
	})
})

func getOtelCollectorDataplaneServiceAccount() *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		AutomountServiceAccountToken: pointer.Bool(true),
	}
}

func getOtelCollectorDataplaneClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector-dataplane-deployment",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes",
					"services",
					"endpoints",
					"pods",
				},
				Verbs: []string{"get", "list", "watch"},
			},
			{
				APIGroups: []string{""},
				Resources: []string{
					"nodes/metrics",
				},
				Verbs: []string{"get"},
			},
		},
	}
}

func getOtelCollectorDataplaneClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "otel-collector-dataplane-deployment",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "otel-collector-dataplane-deployment",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      "ServiceAccount",
				Name:      "otel-collector-dataplane-deployment",
				Namespace: "kube-system",
			},
		},
	}
}

func getOtelCollectorDataplaneService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otel-collector-dataplane-deployment",
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"component": "otel-collector-dataplane-deployment",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "metrics",
					Port:     8080,
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}
