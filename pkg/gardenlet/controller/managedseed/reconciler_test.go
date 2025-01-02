// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseed_test

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockgardenletdeployer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	name      = "test"
	namespace = "garden"

	syncPeriod     = 30 * time.Minute
	waitSyncPeriod = 15 * time.Second
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller

		actuator           *mockgardenletdeployer.MockInterface
		gardenClient       *mockclient.MockClient
		gardenStatusWriter *mockclient.MockStatusWriter

		cfg       gardenletconfigv1alpha1.GardenletConfiguration
		fakeClock clock.Clock

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		managedSeed *seedmanagementv1alpha1.ManagedSeed
		status      *seedmanagementv1alpha1.ManagedSeedStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		actuator = mockgardenletdeployer.NewMockInterface(ctrl)
		gardenClient = mockclient.NewMockClient(ctrl)
		gardenStatusWriter = mockclient.NewMockStatusWriter(ctrl)

		gardenClient.EXPECT().Status().Return(gardenStatusWriter).AnyTimes()

		cfg = gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				ManagedSeed: &gardenletconfigv1alpha1.ManagedSeedControllerConfiguration{
					SyncPeriod:     &metav1.Duration{Duration: syncPeriod},
					WaitSyncPeriod: &metav1.Duration{Duration: waitSyncPeriod},
				},
			},
		}
		fakeClock = testclock.NewFakeClock(time.Time{})

		reconciler = &Reconciler{
			GardenAPIReader: gardenClient,
			GardenClient:    gardenClient,
			Config:          cfg,
			Clock:           fakeClock,
			Recorder:        &record.FakeRecorder{},
		}
		Actuator = actuator
		DeferCleanup(func() { Actuator = nil })

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: name,
				},
				Gardenlet: seedmanagementv1alpha1.GardenletConfig{},
			},
		}
		status = &seedmanagementv1alpha1.ManagedSeedStatus{
			ObservedGeneration: 1,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetManagedSeed = func() {
			gardenClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.GetOption) error {
					*ms = *managedSeed
					return nil
				},
			)
		}
		expectGetShoot = func() {
			gardenClient.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, shoot *gardencorev1beta1.Shoot, _ ...client.GetOption) error {
					*shoot = gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Generation: 1,
						},
						Status: gardencorev1beta1.ShootStatus{
							LastOperation: &gardencorev1beta1.LastOperation{
								State: gardencorev1beta1.LastOperationStateSucceeded,
							},
							ObservedGeneration: 1,
						},
					}
					return nil
				},
			)
		}
		expectPatchManagedSeed = func(expect func(*seedmanagementv1alpha1.ManagedSeed)) {
			gardenClient.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch, _ ...client.PatchOption) error {
					expect(ms)
					*managedSeed = *ms
					return nil
				},
			)
		}
		expectPatchManagedSeedStatus = func(expect func(*seedmanagementv1alpha1.ManagedSeed)) {
			gardenStatusWriter.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ client.Patch, _ ...client.PatchOption) error {
					expect(ms)
					*managedSeed = *ms
					return nil
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			BeforeEach(func() {
				managedSeed.Status.Conditions = []gardencorev1beta1.Condition{{
					Type:               seedmanagementv1alpha1.ManagedSeedShootReconciled,
					Status:             gardencorev1beta1.ConditionTrue,
					LastTransitionTime: metav1.Time{Time: fakeClock.Now()},
					LastUpdateTime:     metav1.Time{Time: fakeClock.Now()},
					Reason:             gardencorev1beta1.EventReconciled,
					Message:            `Shoot "/" has been reconciled`,
				}}
			})

			It("should add the finalizer, if not present", func() {
				expectGetManagedSeed()
				expectGetShoot()
				expectPatchManagedSeed(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(ms.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				})
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &runtime.RawExtension{}, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeed creation or update, and update the status (no wait)", func() {
				expectGetManagedSeed()
				expectGetShoot()
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &runtime.RawExtension{}, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})
		})

		Context("delete", func() {
			BeforeEach(func() {
				ts := metav1.Now()
				managedSeed.DeletionTimestamp = &ts
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the ManagedSeed deletion and update the status (no wait)", func() {
				expectGetManagedSeed()
				expectGetShoot()
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &runtime.RawExtension{}, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, false, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeed deletion and update the status (wait)", func() {
				expectGetManagedSeed()
				expectGetShoot()
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &runtime.RawExtension{}, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, true, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: waitSyncPeriod}))
			})

			It("should reconcile the ManagedSeed deletion, remove the finalizer, and not update the status", func() {
				expectGetManagedSeed()
				expectGetShoot()
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &runtime.RawExtension{}, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, false, true, nil)
				expectPatchManagedSeed(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(ms.Finalizers).To(BeEmpty())
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
