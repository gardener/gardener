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

package networkpolicies_test

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
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	"github.com/gardener/gardener/pkg/operation/botanist/component"
	. "github.com/gardener/gardener/pkg/operation/botanist/component/networkpolicies"
	kubernetesutils "github.com/gardener/gardener/pkg/utils/kubernetes"
)

var _ = Describe("Bootstrap", func() {
	var (
		ctx       = context.TODO()
		fakeErr   = fmt.Errorf("fake error")
		namespace = "garden"

		ctrl     *gomock.Controller
		c        *mockclient.MockClient
		deployer component.DeployWaiter

		managedResourceName       = "global-network-policies"
		managedResourceSecretName = "managedresource-" + managedResourceName

		secret          = &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: managedResourceSecretName}}
		managedResource = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name:      managedResourceName,
				Namespace: namespace,
			},
		}
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		deployer = NewBootstrapper(c, namespace)
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Deploy, #Destroy", func() {
		It("should fail when the managed resource deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the secret deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(deployer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Destroy", func() {
		It("should fail when the managed resource deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the secret deletion fails", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret).Return(fakeErr),
			)

			Expect(deployer.Destroy(ctx)).To(MatchError(fakeErr))
		})

		It("should successfully delete all resources", func() {
			gomock.InOrder(
				c.EXPECT().Delete(ctx, managedResource),
				c.EXPECT().Delete(ctx, secret),
			)

			Expect(deployer.Destroy(ctx)).To(Succeed())
		})
	})

	Describe("#Wait", func() {
		It("should fail because it cannot be checked if the managed resource became healthy", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(deployer.Wait(ctx)).To(MatchError(fakeErr))
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

			Expect(deployer.Wait(ctx)).To(MatchError(ContainSubstring("is not healthy")))
		})

		It("should successfully wait for all resources to be ready", func() {
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

			Expect(deployer.Wait(ctx)).To(Succeed())
		})
	})

	Describe("#WaitCleanup", func() {
		It("should fail when the wait for the managed resource deletion fails", func() {
			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(fakeErr)

			Expect(deployer.WaitCleanup(ctx)).To(MatchError(fakeErr))
		})

		It("should fail when the wait for the managed resource deletion times out", func() {
			oldTimeout := TimeoutWaitForManagedResource
			defer func() { TimeoutWaitForManagedResource = oldTimeout }()
			TimeoutWaitForManagedResource = time.Millisecond

			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).AnyTimes()

			Expect(deployer.WaitCleanup(ctx)).To(MatchError(ContainSubstring("still exists")))
		})

		It("should successfully wait for all resources to be cleaned up", func() {
			c.EXPECT().Get(gomock.Any(), kubernetesutils.Key(namespace, managedResourceName), gomock.AssignableToTypeOf(&resourcesv1alpha1.ManagedResource{})).Return(apierrors.NewNotFound(schema.GroupResource{}, ""))

			Expect(deployer.WaitCleanup(ctx)).To(Succeed())
		})
	})
})
