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
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/controllermanager/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	mockmanagedseedset "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset/mock"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

const (
	syncPeriod = 30 * time.Minute
)

var _ = Describe("reconciler", func() {
	var (
		ctrl *gomock.Controller

		actuator   *mockmanagedseedset.MockActuator
		fakeClient client.Client

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
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&seedmanagementv1alpha1.ManagedSeedSet{}).Build()

		cfg = controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration{
			SyncPeriod: metav1.Duration{Duration: syncPeriod},
		}

		reconciler = &Reconciler{Client: fakeClient, Actuator: actuator, Config: cfg}

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

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			It("should add the finalizer, if not present", func() {
				Expect(fakeClient.Create(ctx, managedSeedSet.DeepCopy())).To(Succeed())

				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any()).Return(status, false, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				Expect(managedSeedSet.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				Expect(managedSeedSet.Status.ObservedGeneration).To(Equal(int64(1)))
			})

			It("should reconcile the ManagedSeedSet creation or update, and update the status", func() {
				managedSeedSet.Finalizers = []string{gardencorev1beta1.GardenerName}
				Expect(fakeClient.Create(ctx, managedSeedSet.DeepCopy())).To(Succeed())

				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any()).Return(status, false, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				Expect(managedSeedSet.Status.ObservedGeneration).To(Equal(int64(1)))
			})
		})

		Context("delete", func() {
			BeforeEach(func() {
				ts := metav1.Now()
				managedSeedSet.DeletionTimestamp = &ts
				managedSeedSet.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the ManagedSeedSet deletion and update the status", func() {
				Expect(fakeClient.Create(ctx, managedSeedSet)).To(Succeed())
				Expect(fakeClient.Delete(ctx, managedSeedSet)).To(Succeed())

				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any()).Return(status, false, nil)

				// Verify object still exists
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), &seedmanagementv1alpha1.ManagedSeedSet{})).To(Succeed())

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), managedSeedSet)).To(Succeed())
				Expect(managedSeedSet.Status.ObservedGeneration).To(Equal(int64(1)))
			})

			It("should reconcile the ManagedSeedSet deletion, remove the finalizer, and not update the status", func() {
				Expect(fakeClient.Create(ctx, managedSeedSet)).To(Succeed())
				Expect(fakeClient.Delete(ctx, managedSeedSet)).To(Succeed())

				// Verify object still exists
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), &seedmanagementv1alpha1.ManagedSeedSet{})).To(Succeed())

				actuator.EXPECT().Reconcile(gomock.Any(), gomock.Any(), gomock.Any()).Return(status, true, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeedSet), &seedmanagementv1alpha1.ManagedSeedSet{})).To(BeNotFoundError())
			})
		})
	})
})
