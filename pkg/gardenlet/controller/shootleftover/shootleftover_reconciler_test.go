// Copyright (c) 2021 SAP SE or an SAP affiliate company.All rights reserved.This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
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

package shootleftover_test

import (
	"context"
	"errors"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shootleftover"
	mocshootleftover "github.com/gardener/gardener/pkg/gardenlet/controller/shootleftover/mock"
	mockrecord "github.com/gardener/gardener/pkg/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	name        = "test"
	namespace   = "garden-foo"
	seedName    = "foo"
	shootName   = "bar"
	technicalID = "shoot--foo--bar"
	uid         = "abcdefgh"
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		actuator     *mocshootleftover.MockActuator
		c            *mockclient.MockClient
		sw           *mockclient.MockStatusWriter
		recorder     *mockrecord.MockEventRecorder

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		shootLeftover *gardencorev1alpha1.ShootLeftover
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		actuator = mocshootleftover.NewMockActuator(ctrl)
		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)

		gardenClient.EXPECT().Client().Return(c).AnyTimes()
		c.EXPECT().Status().Return(sw).AnyTimes()

		reconciler = NewReconciler(gardenClient, actuator, recorder)

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: kutil.Key(namespace, name)}

		shootLeftover = &gardencorev1alpha1.ShootLeftover{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: gardencorev1alpha1.ShootLeftoverSpec{
				SeedName:    seedName,
				ShootName:   shootName,
				TechnicalID: pointer.String(technicalID),
				UID:         func(v types.UID) *types.UID { return &v }(uid),
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGet = func() *gomock.Call {
			return c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1alpha1.ShootLeftover{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, slo *gardencorev1alpha1.ShootLeftover) error {
					*slo = *shootLeftover
					return nil
				},
			)
		}
		expectPatch = func(expect func(*gardencorev1alpha1.ShootLeftover)) *gomock.Call {
			return c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1alpha1.ShootLeftover{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, slo *gardencorev1alpha1.ShootLeftover, _ client.Patch, _ ...client.PatchOption) error {
					expect(slo)
					*shootLeftover = *slo
					return nil
				},
			)
		}
		expectPatchStatus = func(expect func(*gardencorev1alpha1.ShootLeftover)) *gomock.Call {
			return sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1alpha1.ShootLeftover{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, slo *gardencorev1alpha1.ShootLeftover, _ client.Patch, _ ...client.PatchOption) error {
					expect(slo)
					*shootLeftover = *slo
					return nil
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			var (
				expectAddFinalizer = func() *gomock.Call {
					return expectPatch(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
					})
				}
				expectPatchStatusProcessing = func() *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(gardencorev1alpha1.ConditionProgressing),
								"Reason":  Equal(gardencorev1alpha1.EventReconciling),
								"Message": Equal("Checking leftover resources"),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeCreate),
							"State":       Equal(gardencorev1beta1.LastOperationStateProcessing),
							"Progress":    Equal(int32(0)),
							"Description": Equal("Reconciliation of ShootLeftover initialized."),
						})))
					})
				}
				expectPatchStatusSucceeded = func(status gardencorev1alpha1.ConditionStatus, message string) *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(status),
								"Reason":  Equal(gardencorev1alpha1.EventReconciled),
								"Message": Equal(message),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeCreate),
							"State":       Equal(gardencorev1beta1.LastOperationStateSucceeded),
							"Progress":    Equal(int32(100)),
							"Description": Equal("Reconciliation of ShootLeftover succeeded."),
						})))
					})
				}
				expectPatchStatusError = func(status gardencorev1alpha1.ConditionStatus, message string) *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(status),
								"Reason":  Equal(gardencorev1alpha1.EventReconcileError),
								"Message": Equal(message),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeCreate),
							"State":       Equal(gardencorev1beta1.LastOperationStateError),
							"Description": Equal(message + " Operation will be retried."),
						})))
					})
				}
			)

			It("should add the finalizer, reconcile the creation or update, and update the status (resources exist)", func() {
				gomock.InOrder(
					expectGet(),
					expectAddFinalizer(),
					expectPatchStatusProcessing(),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling ShootLeftover"),
					actuator.EXPECT().Reconcile(ctx, shootLeftover).Return(true, nil),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, "ShootLeftover reconciled"),
					expectPatchStatusSucceeded(gardencorev1alpha1.ConditionTrue, "Some leftover resources still exist"),
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should add the finalizer, reconcile the creation or update, and update the status (resources don't exist)", func() {
				gomock.InOrder(
					expectGet(),
					expectAddFinalizer(),
					expectPatchStatusProcessing(),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling ShootLeftover"),
					actuator.EXPECT().Reconcile(ctx, shootLeftover).Return(false, nil),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventReconciled, "ShootLeftover reconciled"),
					expectPatchStatusSucceeded(gardencorev1alpha1.ConditionFalse, "No leftover resources exist"),
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should add the finalizer, reconcile the creation or update, and update the status (reconcile failed)", func() {
				err := errors.New("test")
				gomock.InOrder(
					expectGet(),
					expectAddFinalizer(),
					expectPatchStatusProcessing(),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventReconciling, "Reconciling ShootLeftover"),
					actuator.EXPECT().Reconcile(ctx, shootLeftover).Return(false, err),
					recorder.EXPECT().Eventf(shootLeftover, corev1.EventTypeWarning, gardencorev1beta1.EventReconcileError, "Could not reconcile ShootLeftover: %v", err),
					expectPatchStatusError(gardencorev1alpha1.ConditionUnknown, "Test"),
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})

		Context("delete", func() {
			var (
				expectRemoveFinalizer = func() *gomock.Call {
					return expectPatch(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Finalizers).To(Equal([]string{}))
					})
				}
				expectPatchStatusProcessing = func() *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(gardencorev1alpha1.ConditionProgressing),
								"Reason":  Equal(gardencorev1alpha1.EventDeleting),
								"Message": Equal("Deleting leftover resources"),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeDelete),
							"State":       Equal(gardencorev1beta1.LastOperationStateProcessing),
							"Progress":    Equal(int32(0)),
							"Description": Equal("Deletion of ShootLeftover initialized."),
						})))
					})
				}
				expectPatchStatusSucceeded = func(status gardencorev1alpha1.ConditionStatus, message string) *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(status),
								"Reason":  Equal(gardencorev1alpha1.EventDeleted),
								"Message": Equal(message),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeDelete),
							"State":       Equal(gardencorev1beta1.LastOperationStateSucceeded),
							"Progress":    Equal(int32(100)),
							"Description": Equal("Deletion of ShootLeftover succeeded."),
						})))
					})
				}
				expectPatchStatusError = func(status gardencorev1alpha1.ConditionStatus, message string) *gomock.Call {
					return expectPatchStatus(func(slo *gardencorev1alpha1.ShootLeftover) {
						Expect(slo.Status.Conditions).To(ConsistOf(
							MatchFields(IgnoreExtras, Fields{
								"Type":    Equal(gardencorev1alpha1.ShootLeftoverResourcesExist),
								"Status":  Equal(status),
								"Reason":  Equal(gardencorev1alpha1.EventDeleteError),
								"Message": Equal(message),
							}),
						))
						Expect(slo.Status.LastOperation).To(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":        Equal(gardencorev1beta1.LastOperationTypeDelete),
							"State":       Equal(gardencorev1beta1.LastOperationStateError),
							"Description": Equal(message + " Operation will be retried."),
						})))
					})
				}
			)

			BeforeEach(func() {
				ts := metav1.Now()
				shootLeftover.DeletionTimestamp = &ts
				shootLeftover.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the deletion, update the status, and remove the finalizer", func() {
				gomock.InOrder(
					expectGet(),
					expectPatchStatusProcessing(),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting ShootLeftover"),
					actuator.EXPECT().Delete(ctx, shootLeftover).Return(false, nil),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventDeleted, "ShootLeftover deleted"),
					expectPatchStatusSucceeded(gardencorev1alpha1.ConditionFalse, "No leftover resources exist"),
					expectRemoveFinalizer(),
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})

			It("should reconcile the deletion and update the status (delete failed)", func() {
				err := errors.New("test")
				gomock.InOrder(
					expectGet(),
					expectPatchStatusProcessing(),
					recorder.EXPECT().Event(shootLeftover, corev1.EventTypeNormal, gardencorev1beta1.EventDeleting, "Deleting ShootLeftover"),
					actuator.EXPECT().Delete(ctx, shootLeftover).Return(true, err),
					recorder.EXPECT().Eventf(shootLeftover, corev1.EventTypeWarning, gardencorev1beta1.EventDeleteError, "Could not delete ShootLeftover: %v", err),
					expectPatchStatusError(gardencorev1alpha1.ConditionTrue, "Test"),
				)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).To(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
