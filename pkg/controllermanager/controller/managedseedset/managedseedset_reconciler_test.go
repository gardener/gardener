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

package managedseedset_test

import (
	"context"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	mockmanagedseedset "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset/mock"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	syncPeriod = 30 * time.Minute
)

var _ = Describe("Reconciler", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		actuator     *mockmanagedseedset.MockActuator
		c            *mockclient.MockClient
		sw           *mockclient.MockStatusWriter

		cfg *config.ManagedSeedSetControllerConfiguration

		reconciler reconcile.Reconciler

		ctx     context.Context
		request reconcile.Request

		set    *seedmanagementv1alpha1.ManagedSeedSet
		status *seedmanagementv1alpha1.ManagedSeedSetStatus
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		actuator = mockmanagedseedset.NewMockActuator(ctrl)
		c = mockclient.NewMockClient(ctrl)
		sw = mockclient.NewMockStatusWriter(ctrl)

		gardenClient.EXPECT().Client().Return(c).AnyTimes()
		c.EXPECT().Status().Return(sw).AnyTimes()

		cfg = &config.ManagedSeedSetControllerConfiguration{
			SyncPeriod: metav1.Duration{Duration: syncPeriod},
		}

		reconciler = NewReconciler(gardenClient, actuator, cfg, gardenerlogger.NewNopLogger())

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: kutil.Key(namespace, name)}

		set = &seedmanagementv1alpha1.ManagedSeedSet{
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
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, mss *seedmanagementv1alpha1.ManagedSeedSet) error {
					*mss = *set
					return nil
				},
			)
		}
		expectPatchManagedSeedSet = func(expect func(*seedmanagementv1alpha1.ManagedSeedSet)) {
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, mss *seedmanagementv1alpha1.ManagedSeedSet, _ client.Patch, _ ...client.PatchOption) error {
					expect(mss)
					*set = *mss
					return nil
				},
			)
		}
		expectPatchManagedSeedSetStatus = func(expect func(*seedmanagementv1alpha1.ManagedSeedSet)) {
			sw.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeedSet{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, mss *seedmanagementv1alpha1.ManagedSeedSet, _ client.Patch, _ ...client.PatchOption) error {
					expect(mss)
					*set = *mss
					return nil
				},
			)
		}
	)

	Describe("#Reconcile", func() {
		Context("reconcile", func() {
			It("should add the finalizer, reconcile the ManagedSeedSet creation or update, and update the status", func() {
				expectGetManagedSeedSet()
				expectPatchManagedSeedSet(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(mss.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				})
				actuator.EXPECT().Reconcile(ctx, set).Return(status, false, nil)
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
				set.DeletionTimestamp = &ts
				set.Finalizers = []string{gardencorev1beta1.GardenerName}
			})

			It("should reconcile the ManagedSeedSet deletion and update the status", func() {
				expectGetManagedSeedSet()
				actuator.EXPECT().Reconcile(ctx, set).Return(status, false, nil)
				expectPatchManagedSeedSetStatus(func(mss *seedmanagementv1alpha1.ManagedSeedSet) {
					Expect(&mss.Status).To(Equal(status))
				})

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))
			})

			It("should reconcile the ManagedSeedSet deletion, remove the finalizer, and not update the status", func() {
				expectGetManagedSeedSet()
				actuator.EXPECT().Reconcile(ctx, set).Return(status, true, nil)
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
