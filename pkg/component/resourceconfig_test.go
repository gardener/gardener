// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package component_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils"
	"github.com/gardener/gardener/pkg/utils/managedresources"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ResourceConfig", func() {
	var (
		obj1 *corev1.ConfigMap
		obj2 *corev1.Secret
		obj3 *corev1.Service

		rc1 ResourceConfig
		rc2 ResourceConfig
		rc3 ResourceConfig

		resourceConfigs1 ResourceConfigs
		resourceConfigs2 ResourceConfigs
	)

	BeforeEach(func() {
		obj1 = &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "configmap"}}
		obj2 = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "secret"}}
		obj3 = &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "service"}}

		rc1 = ResourceConfig{Obj: obj1, Class: Runtime}
		rc2 = ResourceConfig{Obj: obj2, Class: Application}
		rc3 = ResourceConfig{Obj: obj3, Class: Application}

		resourceConfigs1 = ResourceConfigs{rc1, rc2}
		resourceConfigs2 = ResourceConfigs{rc3}
	})

	Describe("#AllRuntimeObjects", func() {
		It("should return all runtime objects", func() {
			Expect(AllRuntimeObjects(resourceConfigs1, resourceConfigs2)).To(ConsistOf(obj1))
		})
	})

	Describe("#AllApplicationObjects", func() {
		It("should return all runtime objects", func() {
			Expect(AllApplicationObjects(resourceConfigs1, resourceConfigs2)).To(ConsistOf(obj2, obj3))
		})
	})

	Describe("#MergeResourceConfigs", func() {
		It("should return the expected resource configs", func() {
			Expect(MergeResourceConfigs(resourceConfigs1, resourceConfigs2)).To(ConsistOf(rc1, rc2, rc3))
		})
	})

	Context("Deployment/Destruction", func() {
		var (
			ctx                   = context.Background()
			namespace             = "some-namespace"
			managedResourceName   = "managed-resource-name"
			managedResourceLabels = map[string]string{"foo": "bar"}

			clusterType  ClusterType
			fakeClient   client.Client
			registry     *managedresources.Registry
			allResources ResourceConfigs

			consistOf             func(...client.Object) types.GomegaMatcher
			managedResource       *resourcesv1alpha1.ManagedResource
			managedResourceSecret *corev1.Secret
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
			allResources = MergeResourceConfigs(resourceConfigs1, resourceConfigs2)

			consistOf = NewManagedResourceConsistOfObjectsMatcher(fakeClient)
			managedResource = &resourcesv1alpha1.ManagedResource{
				ObjectMeta: metav1.ObjectMeta{
					Name:      managedResourceName,
					Namespace: namespace,
				},
			}
			managedResourceSecret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "managedresource-" + managedResource.Name,
					Namespace: namespace,
				},
			}
		})

		Context("cluster type seed", func() {
			BeforeEach(func() {
				clusterType = ClusterTypeSeed
				registry = managedresources.NewRegistry(kubernetes.SeedScheme, kubernetes.SeedCodec, kubernetes.SeedSerializer)
			})

			Describe("#Deploy", func() {
				It("should deploy the expected resources", func() {
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

					Expect(DeployResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, managedResourceLabels, registry, allResources)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					expectedMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceName,
							Namespace:       namespace,
							Labels:          utils.MergeStringMaps(map[string]string{"gardener.cloud/role": "seed-system-component"}, managedResourceLabels),
							ResourceVersion: "1",
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							Class: ptr.To("seed"),
							SecretRefs: []corev1.LocalObjectReference{{
								Name: managedResource.Spec.SecretRefs[0].Name,
							}},
							KeepObjects: ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedMr))
					Expect(managedResource).To(DeepEqual(expectedMr))
					Expect(managedResource).To(consistOf(obj1, obj2, obj3))
				})
			})

			Describe("#Destroy", func() {
				It("should destroy the expected resources", func() {
					Expect(DeployResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, managedResourceLabels, registry, allResources)).To(Succeed())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())

					Expect(DestroyResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, allResources)).To(Succeed())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
				})
			})
		})

		Context("cluster type shoot", func() {
			BeforeEach(func() {
				clusterType = ClusterTypeShoot
				registry = managedresources.NewRegistry(kubernetes.ShootScheme, kubernetes.ShootCodec, kubernetes.ShootSerializer)
			})

			Describe("#Deploy", func() {
				It("should deploy the expected resources", func() {
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())

					Expect(DeployResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, managedResourceLabels, registry, allResources)).To(Succeed())

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					expectedMr := &resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
							Name:            managedResourceName,
							Namespace:       namespace,
							ResourceVersion: "1",
							Labels:          map[string]string{"origin": "gardener"},
						},
						Spec: resourcesv1alpha1.ManagedResourceSpec{
							InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
							SecretRefs: []corev1.LocalObjectReference{{
								Name: managedResource.Spec.SecretRefs[0].Name,
							}},
							KeepObjects: ptr.To(false),
						},
					}
					utilruntime.Must(references.InjectAnnotations(expectedMr))
					Expect(managedResource).To(DeepEqual(expectedMr))
					Expect(managedResource).To(consistOf(obj2, obj3))

					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj1), obj1)).To(Succeed())
				})
			})

			Describe("#Destroy", func() {
				It("should destroy the expected resources", func() {
					Expect(DeployResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, managedResourceLabels, registry, allResources)).To(Succeed())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(Succeed())
					managedResourceSecret.Name = managedResource.Spec.SecretRefs[0].Name
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(Succeed())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj1), obj1)).To(Succeed())

					Expect(DestroyResourceConfigs(ctx, fakeClient, namespace, clusterType, managedResourceName, allResources)).To(Succeed())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResource), managedResource)).To(BeNotFoundError())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedResourceSecret), managedResourceSecret)).To(BeNotFoundError())
					Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(obj1), obj1)).To(BeNotFoundError())
				})
			})
		})
	})
})
