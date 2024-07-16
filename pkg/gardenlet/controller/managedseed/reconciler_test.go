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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockgardenletdeployer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
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

		actuator           *mockgardenletdeployer.MockActuator
		gardenClient       *mockclient.MockClient
		gardenStatusWriter *mockclient.MockStatusWriter

		cfg config.GardenletConfiguration

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		managedSeed *seedmanagementv1alpha1.ManagedSeed
		status      *seedmanagementv1alpha1.ManagedSeedStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		actuator = mockgardenletdeployer.NewMockActuator(ctrl)
		gardenClient = mockclient.NewMockClient(ctrl)
		gardenStatusWriter = mockclient.NewMockStatusWriter(ctrl)

		gardenClient.EXPECT().Status().Return(gardenStatusWriter).AnyTimes()

		cfg = config.GardenletConfiguration{
			Controllers: &config.GardenletControllerConfiguration{
				ManagedSeed: &config.ManagedSeedControllerConfiguration{
					SyncPeriod:     &metav1.Duration{Duration: syncPeriod},
					WaitSyncPeriod: &metav1.Duration{Duration: waitSyncPeriod},
				},
			},
		}

		reconciler = &Reconciler{GardenClient: gardenClient, Config: cfg}
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
			It("should add the finalizer, if not present", func() {
				expectGetManagedSeed()
				expectPatchManagedSeed(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(ms.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				})
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeed creation or update, and update the status (no wait)", func() {
				expectGetManagedSeed()
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeed creation or update, and update the status (wait)", func() {
				expectGetManagedSeed()
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, true, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: waitSyncPeriod}))
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
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, false, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeed deletion and update the status (wait)", func() {
				expectGetManagedSeed()
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, true, false, nil)
				expectPatchManagedSeedStatus(func(ms *seedmanagementv1alpha1.ManagedSeed) {
					Expect(&ms.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: waitSyncPeriod}))
			})

			It("should reconcile the ManagedSeed deletion, remove the finalizer, and not update the status", func() {
				expectGetManagedSeed()
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed).Return(status, false, true, nil)
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
