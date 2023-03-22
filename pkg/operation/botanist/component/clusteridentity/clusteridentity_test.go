// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package clusteridentity_test

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/clusteridentity"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("ClusterIdentity", func() {
	var (
		ctrl            *gomock.Controller
		c               *mockclient.MockClient
		clusterIdentity Interface

		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		identity  = "hugo"
		origin    = "shoot"
		namespace = "shoot--foo--bar"

		configMapYAML = `apiVersion: v1
data:
  cluster-identity: ` + identity + `
  origin: ` + origin + `
immutable: true
kind: ConfigMap
metadata:
  creationTimestamp: null
  name: cluster-identity
  namespace: kube-system
`

		managedResourceName       = "shoot-core-cluster-identity"
		managedResourceSecretName = "managedresource-shoot-core-cluster-identity"

		managedResourceSecret *corev1.Secret
		managedResource       *resourcesv1alpha1.ManagedResource
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		clusterIdentity = NewForShoot(c, namespace, identity)

		managedResourceSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceSecretName,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{
				"configmap__kube-system__cluster-identity.yaml": []byte(configMapYAML),
			},
		}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
				Labels:    map[string]string{"origin": "gardener"},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				SecretRefs: []corev1.LocalObjectReference{
					{Name: managedResourceSecretName},
				},
				InjectLabels: map[string]string{"shoot.gardener.cloud/no-cleanup": "true"},
				KeepObjects:  pointer.Bool(false),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy", func() {
		It("should fail because the managed resource secret cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Return(fakeErr),
			)

			Expect(clusterIdentity.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource cannot be updated", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr),
			)

			Expect(clusterIdentity.Deploy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully deploy all resources", func() {
			gomock.InOrder(
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceSecretName), gomock.AssignableToTypeOf(&corev1.Secret{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&corev1.Secret{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResourceSecret))
				}),
				c.EXPECT().Get(ctx, kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})),
				c.EXPECT().Update(ctx, gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Do(func(ctx context.Context, obj client.Object, opts ...client.UpdateOption) {
					Expect(obj).To(DeepEqual(managedResource))
				}),
			)

			Expect(clusterIdentity.Deploy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		managedResourceToDelete := &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}

		It("should fail because the managed resource cannot be deleted", func() {
			c.EXPECT().Delete(ctx, managedResourceToDelete).Return(fakeErr)

			Expect(clusterIdentity.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource secret cannot be deleted", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResourceToDelete),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}).Return(fakeErr),
			)

			Expect(clusterIdentity.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all the resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResourceToDelete),
				c.EXPECT().Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}),
			)

			Expect(clusterIdentity.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should fail because it cannot be checked if the managed resource became healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(clusterIdentity.Wait(ctx)).To(MatchError(fakeErr))
		})

		It("should fail because the managed resource doesn't become healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
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
					}).DeepCopyInto(obj.(*resourcesv1alpha1.ManagedResource))
					return nil
				},
			).AnyTimes()

			Expect(clusterIdentity.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should successfully wait for the managed resource to become healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).DoAndReturn(
				func(ctx context.Context, _ client.ObjectKey, obj client.Object, _ ...client.GetOption) error {
					(&resourcesv1alpha1.ManagedResource{
						ObjectMeta: metav1.ObjectMeta{
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
					}).DeepCopyInto(obj.(*resourcesv1alpha1.ManagedResource))
					return nil
				},
			)

			Expect(clusterIdentity.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		timeNowFunc := func() time.Time { return time.Time{} }

		It("should fail when the wait for the managed resource deletion fails", func() {
			oldTimeNow := gardenerutils.TimeNow
			defer func() { gardenerutils.TimeNow = oldTimeNow }()
			gardenerutils.TimeNow = timeNowFunc

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(clusterIdentity.WaitCleanup(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the wait for the managed resource deletion times out", func() {
			oldTimeNow := gardenerutils.TimeNow
			defer func() { gardenerutils.TimeNow = oldTimeNow }()
			gardenerutils.TimeNow = timeNowFunc

			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()

			Expect(clusterIdentity.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		It("should successfully delete all resources", func() {
			oldTimeNow := gardenerutils.TimeNow
			defer func() { gardenerutils.TimeNow = oldTimeNow }()
			gardenerutils.TimeNow = timeNowFunc

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(clusterIdentity.WaitCleanup(ctx)).To(Succeed())
		})
	})

	Describe("#IsClusterIdentityEmptyOrFromOrigin", func() {
		var (
			seedClient client.Client

			configMapSeed    *corev1.ConfigMap
			configMapNonSeed *corev1.ConfigMap
		)

		BeforeEach(func() {
			seedClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.SeedScheme).Build()

			configMapSeed = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "cluster-identity",
					Namespace: metav1.NamespaceSystem,
				},
				Immutable: pointer.Bool(true),
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
				Immutable: pointer.Bool(true),
				Data: map[string]string{
					"cluster-identity": "foo",
					"origin":           "bar",
				},
			}
		})

		It("should return true if there is no cluster-identity config map", func() {
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, "seed")).To(BeTrue())
		})

		It("should return false if there is a cluster-identity config map with an origin not equal to seed", func() {
			Expect(seedClient.Create(ctx, configMapNonSeed)).To(Succeed())
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, "seed")).To(BeFalse())
		})

		It("should return true if there is a cluster-identity config map with an origin equal to seed", func() {
			Expect(seedClient.Create(ctx, configMapSeed)).To(Succeed())
			Expect(IsClusterIdentityEmptyOrFromOrigin(ctx, seedClient, "seed")).To(BeTrue())
		})
	})
})
