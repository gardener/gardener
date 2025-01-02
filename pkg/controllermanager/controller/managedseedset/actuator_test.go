// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	mockmanagedseedset "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset/mock"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/test"
	mockrecord "github.com/gardener/gardener/third_party/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	name      = "test"
	namespace = "garden"

	maxShootRetries int32 = 3
)

var _ = Describe("Actuator", func() {
	var (
		ctrl *gomock.Controller

		gc       *mockclient.MockClient
		rg       *mockmanagedseedset.MockReplicaGetter
		rf       *mockmanagedseedset.MockReplicaFactory
		r0       *mockmanagedseedset.MockReplica
		recorder *mockrecord.MockEventRecorder

		cfg *controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration

		actuator Actuator

		ctx context.Context
		log logr.Logger

		before  = metav1.Now()
		now     = metav1.Now()
		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gc = mockclient.NewMockClient(ctrl)
		rg = mockmanagedseedset.NewMockReplicaGetter(ctrl)
		rf = mockmanagedseedset.NewMockReplicaFactory(ctrl)
		r0 = mockmanagedseedset.NewMockReplica(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)

		v := int(maxShootRetries)
		cfg = &controllermanagerconfigv1alpha1.ManagedSeedSetControllerConfiguration{
			MaxShootRetries: &v,
		}

		actuator = NewActuator(gc, rg, rf, cfg, recorder)

		ctx = context.TODO()
		log = logr.Discard()

		cleanup = test.WithVar(&Now, func() metav1.Time { return now })
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	var (
		managedSeedSet = func(
			replicas, nextReplicaNumber int32,
			replicaName string,
			reason seedmanagementv1alpha1.PendingReplicaReason,
			retries *int32,
		) *seedmanagementv1alpha1.ManagedSeedSet {
			var pendingReplica *seedmanagementv1alpha1.PendingReplica
			if replicaName != "" {
				pendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:    replicaName,
					Reason:  reason,
					Since:   before,
					Retries: retries,
				}
			}
			return &seedmanagementv1alpha1.ManagedSeedSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:       name,
					Namespace:  namespace,
					Generation: 1,
				},
				Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
					Replicas: ptr.To(replicas),
				},
				Status: seedmanagementv1alpha1.ManagedSeedSetStatus{
					Replicas:          1,
					NextReplicaNumber: nextReplicaNumber,
					PendingReplica:    pendingReplica,
				},
			}
		}
		status = func(
			replicas, readyReplicas, nextReplicaNumber int32,
			replicaName string,
			reason seedmanagementv1alpha1.PendingReplicaReason,
			since metav1.Time,
			retries *int32,
		) *seedmanagementv1alpha1.ManagedSeedSetStatus {
			var pendingReplica *seedmanagementv1alpha1.PendingReplica
			if replicaName != "" {
				pendingReplica = &seedmanagementv1alpha1.PendingReplica{
					Name:    replicaName,
					Reason:  reason,
					Since:   since,
					Retries: retries,
				}
			}
			return &seedmanagementv1alpha1.ManagedSeedSetStatus{
				ObservedGeneration: 1,
				Replicas:           replicas,
				ReadyReplicas:      readyReplicas,
				NextReplicaNumber:  nextReplicaNumber,
				PendingReplica:     pendingReplica,
			}
		}

		expectReplica = func(r *mockmanagedseedset.MockReplica, ordinal int32, status ReplicaStatus, seedReady bool, shootStatus gardenerutils.ShootStatus, deletable bool) {
			r.EXPECT().GetName().Return(getReplicaName(ordinal)).AnyTimes()
			r.EXPECT().GetFullName().Return(getReplicaFullName(ordinal)).AnyTimes()
			r.EXPECT().GetObjectKey().Return(getReplicaObjectKey(ordinal)).AnyTimes()
			r.EXPECT().GetOrdinal().Return(ordinal).AnyTimes()
			r.EXPECT().GetStatus().Return(status).AnyTimes()
			r.EXPECT().IsSeedReady().Return(seedReady).AnyTimes()
			r.EXPECT().GetShootHealthStatus().Return(shootStatus).AnyTimes()
			r.EXPECT().IsDeletable().Return(deletable).AnyTimes()
		}
	)

	Context("not scaling in or out", func() {
		DescribeTable("#Reconcile",
			func(managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, reason, fmt string, args ...any) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, managedSeedSet).Return([]Replica{r0}, nil)

				if fmt != "" {
					recorder.EXPECT().Eventf(managedSeedSet, corev1.EventTypeNormal, reason, fmt, args)
				}

				s, rf, err := actuator.Reconcile(ctx, log, managedSeedSet)
				Expect(err).ToNot(HaveOccurred())
				Expect(s).To(Equal(status))
				Expect(rf).To(Equal(s.ReadyReplicas == s.Replicas))
			},

			Entry("should retry the shoot and return correct status if a replica has status ShootReconcileFailed and max retries not yet reached",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, now, ptr.To[int32](1)),
				EventRetryingShootReconciliation, "Retrying Shoot %s reconciliation", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, ptr.To(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcileFailedReason, now, ptr.To(maxShootRetries)),
				EventNotRetryingShootReconciliation, "Not retrying Shoot %s reconciliation since max retries have been reached", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, ptr.To[int32](1)),
				EventRetryingShootDeletion, "Retrying Shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, ptr.To(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, ptr.To(maxShootRetries)),
				EventNotRetryingShootDeletion, "Not retrying Shoot %s deletion since max retries have been reached", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconciling",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, before, nil),
				EventWaitingForShootReconciled, "Waiting for Shoot %s to be reconciled", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil),
				EventWaitingForShootDeleted, "Waiting for Shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should create the managed seed and return correct status if a replica has status ShootReconciled",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().CreateManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, now, nil),
				EventCreatingManagedSeed, "Creating ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedPreparing",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, before, nil),
				EventWaitingForManagedSeedRegistered, "Waiting for ManagedSeed %s to be registered", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil),
				EventWaitingForManagedSeedDeleted, "Waiting for ManagedSeed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica seed is not ready",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, now, nil),
				EventWaitingForSeedReady, "Waiting for Seed %s to be ready", getReplicaName(0),
			),
			Entry("should return correct status if a replica shoot is not healthy",
				managedSeedSet(1, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootNotHealthyReason, now, nil),
				EventWaitingForShootHealthy, "Waiting for Shoot %s to be healthy", getReplicaFullName(0),
			),
			Entry("should return correct status if all replicas are ready",
				managedSeedSet(1, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 1, 1, "", "", now, nil),
				"", "",
			),
		)
	})

	Context("scaling out", func() {
		DescribeTable("#Reconcile",
			func(managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, reason, fmt string, args ...any) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, managedSeedSet).Return([]Replica{r0}, nil)
				recorder.EXPECT().Eventf(managedSeedSet, corev1.EventTypeNormal, reason, fmt, args)

				s, rf, err := actuator.Reconcile(ctx, log, managedSeedSet)
				Expect(err).ToNot(HaveOccurred())
				Expect(s).To(Equal(status))
				Expect(rf).To(BeFalse())
			},

			Entry("should retry the shoot and return correct status if a replica has status ShootReconcileFailed and max retries not yet reached",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, now, ptr.To[int32](1)),
				EventRetryingShootReconciliation, "Retrying Shoot %s reconciliation", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, ptr.To(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcileFailedReason, now, ptr.To(maxShootRetries)),
				EventNotRetryingShootReconciliation, "Not retrying Shoot %s reconciliation since max retries have been reached", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, ptr.To[int32](1)),
				EventRetryingShootDeletion, "Retrying Shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, ptr.To(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, ptr.To(maxShootRetries)),
				EventNotRetryingShootDeletion, "Not retrying Shoot %s deletion since max retries have been reached", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconciling",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, before, nil),
				EventWaitingForShootReconciled, "Waiting for Shoot %s to be reconciled", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil),
				EventWaitingForShootDeleted, "Waiting for Shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should create the managed seed and return correct status if a replica has status ShootReconciled",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().CreateManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, now, nil),
				EventCreatingManagedSeed, "Creating ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedPreparing",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, before, nil),
				EventWaitingForManagedSeedRegistered, "Waiting for ManagedSeed %s to be registered", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil),
				EventWaitingForManagedSeedDeleted, "Waiting for ManagedSeed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica seed is not ready",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, now, nil),
				EventWaitingForSeedReady, "Waiting for Seed %s to be ready", getReplicaName(0),
			),
			Entry("should return correct status if a replica shoot is not healthy",
				managedSeedSet(2, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootNotHealthyReason, now, nil),
				EventWaitingForShootHealthy, "Waiting for Shoot %s to be healthy", getReplicaFullName(0),
			),
			Entry("should create the shoot of a new replica and return correct status if all replicas are ready",
				managedSeedSet(2, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusHealthy, true)
					r1 := mockmanagedseedset.NewMockReplica(ctrl)
					rf.EXPECT().NewReplica(managedSeedSet(2, 1, "", "", nil), nil, nil, nil, false).Return(r1)
					r1.EXPECT().CreateShoot(ctx, gc, int32(1)).Return(nil)
					r1.EXPECT().GetName().Return(getReplicaName(1))
				},
				status(2, 1, 2, getReplicaName(1), seedmanagementv1alpha1.ShootReconcilingReason, now, nil),
				EventCreatingShoot, "Creating Shoot %s", getReplicaFullName(1),
			),
			Entry("should create the shoot of a new replica and return correct status if all replicas are ready and nextReplicaNumber is invalid",
				managedSeedSet(2, 0, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusHealthy, true)
					r1 := mockmanagedseedset.NewMockReplica(ctrl)
					rf.EXPECT().NewReplica(managedSeedSet(2, 0, "", "", nil), nil, nil, nil, false).Return(r1)
					r1.EXPECT().CreateShoot(ctx, gc, int32(1)).Return(nil)
					r1.EXPECT().GetName().Return(getReplicaName(1))
				},
				status(2, 1, 2, getReplicaName(1), seedmanagementv1alpha1.ShootReconcilingReason, now, nil),
				EventCreatingShoot, "Creating Shoot %s", getReplicaFullName(1),
			),
		)
	})

	Context("scaling in", func() {
		DescribeTable("#Reconcile",
			func(managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, success bool, reason, fmt string, args ...any) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, managedSeedSet).Return([]Replica{r0}, nil)
				if success {
					recorder.EXPECT().Eventf(managedSeedSet, corev1.EventTypeNormal, reason, fmt, args)
				} else {
					recorder.EXPECT().Eventf(managedSeedSet, corev1.EventTypeWarning, reason, fmt, args)
				}

				s, rf, err := actuator.Reconcile(ctx, log, managedSeedSet)
				if success {
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
				Expect(s).To(Equal(status))
				Expect(rf).To(BeFalse())
			},

			Entry("should delete the shoot and return correct status if a replica has status ShootReconcileFailed",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting Shoot %s", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, ptr.To[int32](1)), true,
				EventRetryingShootDeletion, "Retrying Shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, ptr.To(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, gardenerutils.ShootStatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, ptr.To(maxShootRetries)), true,
				EventNotRetryingShootDeletion, "Not retrying Shoot %s deletion since max retries have been reached", getReplicaFullName(0),
			),
			Entry("should delete the shoot and return correct status if a replica has status ShootReconciling",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting Shoot %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil), true,
				EventWaitingForShootDeleted, "Waiting for Shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should delete the shoot and return correct status if a replica has status ShootReconciled",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting Shoot %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica has status ManagedSeedPreparing",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, gardenerutils.ShootStatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil), true,
				EventWaitingForManagedSeedDeleted, "Waiting for ManagedSeed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica seed is not ready",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica shoot is not healthy",
				managedSeedSet(0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusUnhealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed of a deletable replica and return correct status if all replicas are ready",
				managedSeedSet(0, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting ManagedSeed %s", getReplicaFullName(0),
			),
			Entry("should fail if all replicas are ready and there are no deletable replicas",
				managedSeedSet(0, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, gardenerutils.ShootStatusHealthy, false)
				},
				status(1, 1, 1, "", "", now, nil), false,
				gardencorev1beta1.EventReconcileError, "no deletable replicas found",
			),
		)
	})
})

func getReplicaName(ordinal int32) string {
	return fmt.Sprintf("%s-%d", name, ordinal)
}

func getReplicaFullName(ordinal int32) string {
	return fmt.Sprintf("%s/%s", namespace, getReplicaName(ordinal))
}

func getReplicaObjectKey(ordinal int32) client.ObjectKey {
	return client.ObjectKey{Namespace: namespace, Name: getReplicaName(ordinal)}
}
