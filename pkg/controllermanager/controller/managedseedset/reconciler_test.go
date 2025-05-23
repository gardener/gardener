// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	mockmanagedseedset "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset/mock"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	syncPeriod = 30 * time.Minute
)

var _ = Describe("reconciler", func() {
	var (
		ctrl *gomock.Controller

		actuator *mockmanagedseedset.MockActuator
		c        *mockclient.MockClient
		sw       *mockclient.MockStatusWriter

		cfg controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet
		status         *seedmanagementv1alpha1.ManagedSeedSetStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		actuator = mockmanagedseedset.NewMockActuator(ctrl)
		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)

		c.EXPECT().Status().Return(sw).AnyTimes()

		cfg = controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration{
			SyncPeriod: metav1.Duration{Duration: syncPeriod},
		}

		reconciler = &Reconciler{Client: c, Actuator: actuator, Config: cfg}

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}

		managedSeedSet = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:       name,
				Namespace:  namespace,
				Generation: 1,
			},
		}
		status = &seedmanagementv1alpha1.ManagedSeedSetStatus{
			ObservedGeneration: 1,
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetManagedSeedSet = func() {
			c.EXPECT().Get(gomock.Any(), client.ObjectKey{Namespace: namespace, Name: name}, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, mss *seedmanagementv1alpha1.ManagedSeedSet, _ ...client.GetOption) error {
					*mss = *managedSeedSet
					return nil
				},
			)
		}
		expectPatchManagedSeedSet = func(expect func(*seedmanagementv1alpha1.ManagedSeedSet)) {
			c.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, mss *seedmanagementv1alpha1.ManagedSeedSet, _ client.Patch, _ ...client.PatchOption) error {
					expect(mss)
					*managedSeedSet = *mss
					return nil
				},
			)
		}
		expectPatchManagedSeedSetStatus = func(expect func(*seedmanagementv1alpha1.ManagedSeedSet)) {
			sw.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, mss *seedmanagementv1alpha1.ManagedSeedSet, _ client.Patch, _ ...client.PatchOption) error {
					expect(mss)
					*managedSeedSet = *mss
					return nil
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			It("should add the finalizer, if not present", func() {
				expectGetManagedSeedSet()
				expectPatchManagedSeedSet(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(mss.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				})
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), managedSeedSet).Return(status, false, nil)
				expectPatchManagedSeedSetStatus(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(&mss.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeedSet creation or update, and update the status", func() {
				expectGetManagedSeedSet()
				managedSeedSet.Finalizers = []string{gardencorev1beta1.GardenerName}
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), managedSeedSet).Return(status, false, nil)
				expectPatchManagedSeedSetStatus(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(&mss.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})
		})

		Context("delete", func() {
			BeforeEach(func() {
				ts := metav1.Now()
				managedSeedSet.DeletionTimestamp = &ts
				managedSeedSet.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the ManagedSeedSet deletion and update the status", func() {
				expectGetManagedSeedSet()
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), managedSeedSet).Return(status, false, nil)
				expectPatchManagedSeedSetStatus(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(&mss.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeedSet deletion, remove the finalizer, and not update the status", func() {
				expectGetManagedSeedSet()
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), managedSeedSet).Return(status, true, nil)
				expectPatchManagedSeedSet(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(mss.Finalizers).To(BeEmpty())
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
			})
		})
	})
})
