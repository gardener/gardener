// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package managedseed

import (
	"context"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockrecord "github.com/gardener/gardener/pkg/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mockkubernetes "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	mockmanagedseed "github.com/gardener/gardener/pkg/mock/gardener/gardenlet/controller/managedseed"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name      = "test"
	namespace = "garden"
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		actuator     *mockmanagedseed.MockActuator
		recorder     *mockrecord.MockEventRecorder
		c            *mockclient.MockClient
		sw           *mockclient.MockStatusWriter

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		managedSeed *seedmanagementv1alpha1.ManagedSeed
		shoot       *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		actuator = mockmanagedseed.NewMockActuator(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)
		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)

		gardenClient.EXPECT().Client().Return(c).AnyTimes()
		gardenClient.EXPECT().DirectClient().Return(c).AnyTimes()
		c.EXPECT().Status().Return(sw).AnyTimes()

		reconciler = newReconciler(gardenClient, actuator, recorder, gardenerlogger.NewNopLogger())

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: kutil.Key(namespace, name)}

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: seedmanagementv1alpha1.Shoot{
					Name: name,
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Status: gardencorev1beta1.ShootStatus{
				LastOperation: &gardencorev1beta1.LastOperation{
					State: gardencorev1beta1.LastOperationStateSucceeded,
				},
				ObservedGeneration: 1,
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			It("should reconcile the ManagedSeed creation or update", func() {
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
						*ms = *managedSeed
						return nil
					},
				)
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch) error {
						Expect(ms.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
						*managedSeed = *ms
						return nil
					},
				)
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Shoot) error {
						*s = *shoot
						return nil
					},
				)
				actuator.EXPECT().Reconcile(ctx, managedSeed, shoot).Return(nil)
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
						*ms = *managedSeed
						return nil
					},
				)
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch) error {
						Expect(ms.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(seedmanagementv1alpha1.ManagedSeedShootExists),
								"Status": Equal(gardencorev1beta1.ConditionTrue),
								"Reason": Equal("ShootFound"),
							}),
							MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(seedmanagementv1alpha1.ManagedSeedShootReconciled),
								"Status": Equal(gardencorev1beta1.ConditionTrue),
								"Reason": Equal("ShootReconciled"),
							}),
							MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
								"Status": Equal(gardencorev1beta1.ConditionTrue),
								"Reason": Equal("SeedRegistered"),
							}),
						))
						Expect(ms.Status.ObservedGeneration).To(Equal(int64(1)))
						*managedSeed = *ms
						return nil
					},
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("delete", func() {
			BeforeEach(func() {
				ts := metav1.Now()
				managedSeed.DeletionTimestamp = &ts
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the ManagedSeed deletion", func() {
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
						*ms = *managedSeed
						return nil
					},
				)
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Shoot) error {
						*s = *shoot
						return nil
					},
				)
				actuator.EXPECT().Delete(ctx, managedSeed, shoot).Return(nil)
				c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch) error {
						Expect(ms.Finalizers).To(BeEmpty())
						*managedSeed = *ms
						return nil
					},
				)
				c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
					func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
						*ms = *managedSeed
						return nil
					},
				)
				sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
					func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch) error {
						Expect(ms.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(seedmanagementv1alpha1.ManagedSeedShootExists),
								"Status": Equal(gardencorev1beta1.ConditionTrue),
								"Reason": Equal("ShootFound"),
							}),
							MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(seedmanagementv1alpha1.ManagedSeedSeedRegistered),
								"Status": Equal(gardencorev1beta1.ConditionFalse),
								"Reason": Equal("SeedUnregistered"),
							}),
						))
						Expect(ms.Status.ObservedGeneration).To(Equal(int64(1)))
						*managedSeed = *ms
						return nil
					},
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
