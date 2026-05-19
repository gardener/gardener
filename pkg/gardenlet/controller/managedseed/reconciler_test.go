// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
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
	"k8s.io/client-go/tools/events"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement/encoding"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	mockgardenletdeployer "github.com/gardener/gardener/pkg/controller/gardenletdeployer/mock"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/managedseed"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
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

		actuator   *mockgardenletdeployer.MockInterface
		fakeClient client.Client

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
		fakeClient = fakeclient.NewClientBuilder().
			WithScheme(kubernetes.GardenScheme).
			WithStatusSubresource(&seedmanagementv1alpha1.ManagedSeed{}).
			Build()

		cfg = gardenletconfigv1alpha1.GardenletConfiguration{
			Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
				ManagedSeed: &gardenletconfigv1alpha1.ManagedSeedControllerConfiguration{
					SyncPeriod:     &metav1.Duration{Duration: syncPeriod},
					WaitSyncPeriod: &metav1.Duration{Duration: waitSyncPeriod},
				},
			},
		}
		fakeClock = testclock.NewFakeClock(time.Time{})

		gardenClusterAddress := "foobar"
		reconciler = &Reconciler{
			GardenAPIReader: fakeClient,
			GardenClient:    fakeClient,
			Config:          cfg,
			Clock:           fakeClock,
			Recorder:        &events.FakeRecorder{},
		}
		Actuator = actuator
		DeferCleanup(func() { Actuator = nil })

		ctx = context.TODO()
		request = reconcile.Request{NamespacedName: client.ObjectKey{Namespace: namespace, Name: name}}

		gardenletConfig := &gardenletconfigv1alpha1.GardenletConfiguration{
			GardenClientConnection: &gardenletconfigv1alpha1.GardenClientConnection{
				GardenClusterAddress: new(gardenClusterAddress),
			},
		}
		gardenletConfigRaw, err := encoding.EncodeGardenletConfiguration(gardenletConfig)
		Expect(err).ToNot(HaveOccurred())

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
				Gardenlet: seedmanagementv1alpha1.GardenletConfig{
					Config: *gardenletConfigRaw,
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

				Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

				// Create a Shoot that the reconciler reads
				Expect(fakeClient.Create(ctx, &gardencorev1beta1.Shoot{
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
				})).To(Succeed())
			})

			It("should add the finalizer, if not present", func() {
				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.AssignableToTypeOf([]gardencorev1beta1.Condition{}), managedSeed.Spec.Gardenlet.Deployment, gomock.Any(), seedmanagementv1alpha1.BootstrapNone, false).Return(nil, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				Expect(managedSeed.Finalizers).To(Equal([]string{gardencorev1beta1.GardenerName}))
				Expect(&managedSeed.Status).To(Equal(status))
			})

			It("should reconcile the ManagedSeed creation or update, and update the status (no wait)", func() {
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
				Expect(fakeClient.Update(ctx, managedSeed)).To(Succeed())

				actuator.EXPECT().Reconcile(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.AssignableToTypeOf([]gardencorev1beta1.Condition{}), managedSeed.Spec.Gardenlet.Deployment, gomock.Any(), seedmanagementv1alpha1.BootstrapNone, false).Return(nil, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				Expect(&managedSeed.Status).To(Equal(status))
			})
		})

		Context("delete", func() {
			BeforeEach(func() {
				managedSeed.Finalizers = []string{gardencorev1beta1.GardenerName}
				Expect(fakeClient.Create(ctx, managedSeed)).To(Succeed())

				// Create a Shoot
				Expect(fakeClient.Create(ctx, &gardencorev1beta1.Shoot{
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
				})).To(Succeed())

				// Delete the managed seed (sets DeletionTimestamp)
				Expect(fakeClient.Delete(ctx, managedSeed)).To(Succeed())
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
			})

			It("should reconcile the ManagedSeed deletion and update the status (no wait)", func() {
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &managedSeed.Spec.Gardenlet.Config, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, false, false, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: syncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				Expect(&managedSeed.Status).To(Equal(status))
			})

			It("should reconcile the ManagedSeed deletion and update the status (wait)", func() {
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{}), gomock.Any(), gomock.Any(), gomock.Any(), seedmanagementv1alpha1.BootstrapNone, false).Return(nil, true, false, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{RequeueAfter: waitSyncPeriod}))

				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(Succeed())
				Expect(&managedSeed.Status).To(Equal(status))
			})

			It("should reconcile the ManagedSeed deletion, remove the finalizer, and not update the status", func() {
				actuator.EXPECT().Delete(gomock.Any(), gomock.AssignableToTypeOf(logr.Logger{}), managedSeed, managedSeed.Status.Conditions, managedSeed.Spec.Gardenlet.Deployment, &managedSeed.Spec.Gardenlet.Config, seedmanagementv1alpha1.BootstrapNone, false).Return(nil, false, true, nil)

				result, err := reconciler.Reconcile(ctx, request)
				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))

				// ManagedSeed should be fully deleted (no finalizer, so it gets cleaned up)
				Expect(fakeClient.Get(ctx, client.ObjectKeyFromObject(managedSeed), managedSeed)).To(BeNotFoundError())
			})
		})
	})
})
