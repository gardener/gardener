// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package clusteridentity_test

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

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/component/clusteridentity"
	"github.com/gardener/gardener/pkg/resourcemanager/controller/garbagecollector/references"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ClusterIdentity", func() {
	var (
		c               client.Client
		clusterIdentity Interface
		consistOf       func(...client.Object) types.GomegaMatcher

		ctx       = context.Background()
		identity  = "hugo"
		origin    = "shoot"
		namespace = "shoot--foo--bar"

		configMap = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "cluster-identity",
				Namespace: "kube-system",
			},
			Data: map[string]string{
				"cluster-identity": identity,
				"origin":           origin,
			},
			Immutable: ptr.To(true),
		}

		managedResourceName       = "shoot-core-cluster-identity"
		managedResourceSecretName = "managedresource-shoot-core-cluster-identity"

		managedResource *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		c = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()
		clusterIdentity = NewForShoot(c, namespace, identity)
		consistOf = NewManagedResourceConsistOfObjectsMatcher(c)

		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:            managedResourceName,
				Namespace:       namespace,
				Labels:          map[string]string{"origin": "gardener"},
				ResourceVersion: "1",
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs:   []corev1.LocalObjectReference{},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  ptr.To(false),
			},
		}
	})

	Describe("#Deploy", func() {
		It("should successfully deploy all resources", func() {
			Expect(clusterIdentity.Deploy(ctx)).To(Succeed())

			actualMr := &resourcesv1alpha1.ManagedResource{}
			Expect(c.Get(ctx, client.ObjectKeyFromObject(managedResource), actualMr)).To(Succeed())
			managedResource.Spec.SecretRefs = []corev1.LocalObjectReference{{Name: actualMr.Spec.SecretRefs[0].Name}}

			utilruntime.Must(references.InjectAnnotations(managedResource))
			Expect(actualMr).To(DeepEqual(managedResource))
			Expect(actualMr).To(consistOf(configMap))
		})
	})

	Describe("#Destroy", func() {
		It("should successfully delete all the resources", func() {
			mrSecret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: managedResourceSecretName, Namespace: namespace}}
			mr := &resourcesv1alpha1.ManagedResource{ObjectMeta: metav1.ObjectMeta{Name: managedResourceName, Namespace: namespace}}
			Expect(c.Create(ctx, mrSecret)).To(Succeed())
			Expect(c.Create(ctx, mr)).To(Succeed())

			Expect(clusterIdentity.Destroy(ctx)).To(Succeed())

			Expect(c.Get(ctx, client.ObjectKeyFromObject(mrSecret), mrSecret)).To(BeNotFoundError())
			Expect(c.Get(ctx, client.ObjectKeyFromObject(mr), mr)).To(BeNotFoundError())
		})
	})

	Context("waiting functions", func() {
		var (
			fakeOps   *retryfake.Ops
			resetVars func()
		)

		BeforeEach(func() {
			fakeOps = &retryfake.Ops{MaxAttempts: 1}
			resetVars = test.WithVars(
				&retry.Until, fakeOps.Until,
				&retry.UntilTimeout, fakeOps.UntilTimeout,
			)
		})

		AfterEach(func() {
			resetVars()
		})

		Describe("#Wait", func() {
			It("should fail because reading the ManagedResource fails", func() {
				Expect(clusterIdentity.Wait(ctx)).To(MatchError(ContainSubstring("not found")))
			})

			It("should fail because the ManagedResource doesn't become healthy", func() {
				fakeOps.MaxAttempts = 2

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

				Expect(clusterIdentity.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
			})

			It("should successfully wait for the managed resource to become healthy", func() {
				fakeOps.MaxAttempts = 2

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
						},
					},
				})).To(Succeed())

				Expect(clusterIdentity.Wait(ctx)).To(Succeed())
			})
		})

		Describe("#WaitCleanup", func() {
			It("should fail when the wait for the managed resource deletion times out", func() {
				fakeOps.MaxAttempts = 2

				Expect(c.Create(ctx, &resourcesv1alpha1.ManagedResource{
					ObjectMeta: metav1.ObjectMeta{
						Name:      managedResourceName,
						Namespace: namespace,
					},
				})).To(Succeed())

				Expect(clusterIdentity.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
			})

			It("should not return an error when it's already removed", func() {
				Expect(clusterIdentity.WaitCleanup(ctx)).To(Succeed())
			})
		})
	})

	Describe("#IsClusterIdentityEmptyOrFromOrigin", func() {
		var (
			configMapSeed    *corev1.ConfigMap
			configMapNonSeed *corev1.ConfigMap
		)

		BeforeEach(func() {
			configMapSeed = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-identity",
					Namespace: metav1.NamespaceSystem,
				},
				Immutable: ptr.To(true),
				Data: map[string]string{
					"cluster-identity": "foo",
					"origin":           "seed",
				},
			}
			configMapNonSeed = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-identity",
					Namespace: metav1.NamespaceSystem,
				},
				Immutable: ptr.To(true),
				Data: map[string]string{
					"cluster-identity": "foo",
					"origin":           "bar",
				},
			}
		})

		It("should return true if there is no cluster-identity config map", func() {
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, c, "seed")).To(BeTrue())
		})

		It("should return false if there is a cluster-identity config map with an origin not equal to seed", func() {
			Expect(c.Create(ctx, configMapNonSeed)).To(Succeed())
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, c, "seed")).To(BeFalse())
		})

		It("should return true if there is a cluster-identity config map with an origin equal to seed", func() {
			Expect(c.Create(ctx, configMapSeed)).To(Succeed())
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, c, "seed")).To(BeTrue())
		})
	})
})
