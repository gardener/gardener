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
	"fmt"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	"github.com/gardener/gardener/pkg/controllermanager/apis/config"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	mockmanagedseedset "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset/mock"
	gardenerlogger "github.com/gardener/gardener/pkg/logger"
	mockrecord "github.com/gardener/gardener/pkg/mock/client-go/tools/record"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	operationshoot "github.com/gardener/gardener/pkg/operation/shoot"
	"github.com/gardener/gardener/pkg/utils/test"
)

const (
	name      = "test"
	namespace = "garden"

	maxShootRetries = 3
)

var _ = Describe("Actuator", func() {
	var (
		ctrl *gomock.Controller

		gardenClient *mockkubernetes.MockInterface
		gc           *mockclient.MockClient
		rg           *mockmanagedseedset.MockReplicaGetter
		rf           *mockmanagedseedset.MockReplicaFactory
		r0           *mockmanagedseedset.MockReplica
		recorder     *mockrecord.MockEventRecorder

		cfg *config.ManagedSeedSetControllerConfiguration

		actuator Actuator

		ctx context.Context

		before  = metav1.Now()
		now     = metav1.Now()
		cleanup func()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())

		gardenClient = mockkubernetes.NewMockInterface(ctrl)
		gc = mockclient.NewMockClient(ctrl)
		rg = mockmanagedseedset.NewMockReplicaGetter(ctrl)
		rf = mockmanagedseedset.NewMockReplicaFactory(ctrl)
		r0 = mockmanagedseedset.NewMockReplica(ctrl)
		recorder = mockrecord.NewMockEventRecorder(ctrl)

		gardenClient.EXPECT().Client().Return(gc).AnyTimes()

		v := maxShootRetries
		cfg = &config.ManagedSeedSetControllerConfiguration{
			MaxShootRetries: &v,
		}

		actuator = NewActuator(gardenClient, rg, rf, cfg, recorder, gardenerlogger.NewNopLogger())

		ctx = context.TODO()

		cleanup = test.WithVar(&Now, func() metav1.Time { return now })
	})

	AfterEach(func() {
		cleanup()
		ctrl.Finish()
	})

	var (
		set = func(
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
					Replicas: pointer.Int32(replicas),
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

		expectReplica = func(r *mockmanagedseedset.MockReplica, ordinal int, status ReplicaStatus, seedReady bool, shs operationshoot.Status, deletable bool) {
			r.EXPECT().GetName().Return(getReplicaName(ordinal)).AnyTimes()
			r.EXPECT().GetFullName().Return(getReplicaFullName(ordinal)).AnyTimes()
			r.EXPECT().GetOrdinal().Return(ordinal).AnyTimes()
			r.EXPECT().GetStatus().Return(status).AnyTimes()
			r.EXPECT().IsSeedReady().Return(seedReady).AnyTimes()
			r.EXPECT().GetShootHealthStatus().Return(shs).AnyTimes()
			r.EXPECT().IsDeletable().Return(deletable).AnyTimes()
		}
	)

	Context("not scaling in or out", func() {
		DescribeTable("#Reconcile",
			func(set *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, reason, fmt string, args ...interface{}) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, set).Return([]Replica{r0}, nil)
				if fmt != "" {
					recorder.EXPECT().Eventf(set, corev1.EventTypeNormal, reason, fmt, args)
				}

				s, rf, err := actuator.Reconcile(ctx, set)
				Expect(err).ToNot(HaveOccurred())
				Expect(s).To(Equal(status))
				Expect(rf).To(Equal(s.ReadyReplicas == s.Replicas))
			},

			Entry("should retry the shoot and return correct status if a replica has status ShootReconcileFailed and max retries not yet reached",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, now, pointer.Int32(1)),
				EventRetryingShootReconciliation, "Retrying shoot %s reconciliation", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, pointer.Int32(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcileFailedReason, now, pointer.Int32(maxShootRetries)),
				EventNotRetryingShootReconciliation, "Not retrying shoot %s reconciliation since max retries has been reached", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, pointer.Int32(1)),
				EventRetryingShootDeletion, "Retrying shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, pointer.Int32(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, pointer.Int32(maxShootRetries)),
				EventNotRetryingShootDeletion, "Not retrying shoot %s deletion since max retries has been reached", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconciling",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, before, nil),
				EventWaitingForShootReconciled, "Waiting for shoot %s to be reconciled", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil),
				EventWaitingForShootDeleted, "Waiting for shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should create the managed seed and return correct status if a replica has status ShootReconciled",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().CreateManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, now, nil),
				EventCreatingManagedSeed, "Creating managed seed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedPreparing",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, before, nil),
				EventWaitingForManagedSeedRegistered, "Waiting for managed seed %s to be registered", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil),
				EventWaitingForManagedSeedDeleted, "Waiting for managed seed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica seed is not ready",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, now, nil),
				EventWaitingForSeedReady, "Waiting for seed %s to be ready", getReplicaName(0),
			),
			Entry("should return correct status if a replica shoot is not healthy",
				set(1, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootNotHealthyReason, now, nil),
				EventWaitingForShootHealthy, "Waiting for shoot %s to be healthy", getReplicaFullName(0),
			),
			Entry("should return correct status if all replicas are ready",
				set(1, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusHealthy, true)
				},
				status(1, 1, 1, "", "", now, nil),
				"", "",
			),
		)
	})

	Context("scaling out", func() {
		DescribeTable("#Reconcile",
			func(set *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, reason, fmt string, args ...interface{}) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, set).Return([]Replica{r0}, nil)
				recorder.EXPECT().Eventf(set, corev1.EventTypeNormal, reason, fmt, args)

				s, rf, err := actuator.Reconcile(ctx, set)
				Expect(err).ToNot(HaveOccurred())
				Expect(s).To(Equal(status))
				Expect(rf).To(BeFalse())
			},

			Entry("should retry the shoot and return correct status if a replica has status ShootReconcileFailed and max retries not yet reached",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, now, pointer.Int32(1)),
				EventRetryingShootReconciliation, "Retrying shoot %s reconciliation", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, pointer.Int32(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcileFailedReason, now, pointer.Int32(maxShootRetries)),
				EventNotRetryingShootReconciliation, "Not retrying shoot %s reconciliation since max retries has been reached", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, pointer.Int32(1)),
				EventRetryingShootDeletion, "Retrying shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, pointer.Int32(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, pointer.Int32(maxShootRetries)),
				EventNotRetryingShootDeletion, "Not retrying shoot %s deletion since max retries has been reached", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconciling",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, before, nil),
				EventWaitingForShootReconciled, "Waiting for shoot %s to be reconciled", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil),
				EventWaitingForShootDeleted, "Waiting for shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should create the managed seed and return correct status if a replica has status ShootReconciled",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().CreateManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, now, nil),
				EventCreatingManagedSeed, "Creating managed seed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedPreparing",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, before, nil),
				EventWaitingForManagedSeedRegistered, "Waiting for managed seed %s to be registered", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil),
				EventWaitingForManagedSeedDeleted, "Waiting for managed seed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica seed is not ready",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, now, nil),
				EventWaitingForSeedReady, "Waiting for seed %s to be ready", getReplicaName(0),
			),
			Entry("should return correct status if a replica shoot is not healthy",
				set(2, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootNotHealthyReason, now, nil),
				EventWaitingForShootHealthy, "Waiting for shoot %s to be healthy", getReplicaFullName(0),
			),
			Entry("should create the shoot of a new replica and return correct status if all replicas are ready",
				set(2, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusHealthy, true)
					r1 := mockmanagedseedset.NewMockReplica(ctrl)
					rf.EXPECT().NewReplica(set(2, 1, "", "", nil), nil, nil, nil, false).Return(r1)
					r1.EXPECT().CreateShoot(ctx, gc, 1).Return(nil)
					r1.EXPECT().GetName().Return(getReplicaName(1))
				},
				status(2, 1, 2, getReplicaName(1), seedmanagementv1alpha1.ShootReconcilingReason, now, nil),
				EventCreatingShoot, "Creating shoot %s", getReplicaFullName(1),
			),
			Entry("should create the shoot of a new replica and return correct status if all replicas are ready and nextReplicaNumber is invalid",
				set(2, 0, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusHealthy, true)
					r1 := mockmanagedseedset.NewMockReplica(ctrl)
					rf.EXPECT().NewReplica(set(2, 0, "", "", nil), nil, nil, nil, false).Return(r1)
					r1.EXPECT().CreateShoot(ctx, gc, 1).Return(nil)
					r1.EXPECT().GetName().Return(getReplicaName(1))
				},
				status(2, 1, 2, getReplicaName(1), seedmanagementv1alpha1.ShootReconcilingReason, now, nil),
				EventCreatingShoot, "Creating shoot %s", getReplicaFullName(1),
			),
		)
	})

	Context("scaling in", func() {
		DescribeTable("#Reconcile",
			func(set *seedmanagementv1alpha1.ManagedSeedSet, setupReplicas func(), status *seedmanagementv1alpha1.ManagedSeedSetStatus, success bool, reason, fmt string, args ...interface{}) {
				setupReplicas()
				rg.EXPECT().GetReplicas(ctx, set).Return([]Replica{r0}, nil)
				if success {
					recorder.EXPECT().Eventf(set, corev1.EventTypeNormal, reason, fmt, args)
				} else {
					recorder.EXPECT().Eventf(set, corev1.EventTypeWarning, reason, fmt, args)
				}

				s, rf, err := actuator.Reconcile(ctx, set)
				if success {
					Expect(err).ToNot(HaveOccurred())
				} else {
					Expect(err).To(HaveOccurred())
				}
				Expect(s).To(Equal(status))
				Expect(rf).To(BeFalse())
			},

			Entry("should delete the shoot and return correct status if a replica has status ShootReconcileFailed",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconcileFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting shoot %s", getReplicaFullName(0),
			),
			Entry("should retry the shoot and return correct status if a replica has status ShootDeleteFailed and max retries not yet reached",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().RetryShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, pointer.Int32(1)), true,
				EventRetryingShootDeletion, "Retrying shoot %s deletion", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootReconcileFailed and max retries reached",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, pointer.Int32(maxShootRetries)),
				func() {
					expectReplica(r0, 0, StatusShootDeleteFailed, false, operationshoot.StatusUnhealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeleteFailedReason, now, pointer.Int32(maxShootRetries)), true,
				EventNotRetryingShootDeletion, "Not retrying shoot %s deletion since max retries has been reached", getReplicaFullName(0),
			),
			Entry("should delete the shoot and return correct status if a replica has status ShootReconciling",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciling, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting shoot %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ShootDeleting",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, before, nil), true,
				EventWaitingForShootDeleted, "Waiting for shoot %s to be deleted", getReplicaFullName(0),
			),
			Entry("should delete the shoot and return correct status if a replica has status ShootReconciled",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootReconcilingReason, nil),
				func() {
					expectReplica(r0, 0, StatusShootReconciled, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().DeleteShoot(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ShootDeletingReason, now, nil), true,
				EventDeletingShoot, "Deleting shoot %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica has status ManagedSeedPreparing",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedPreparing, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting managed seed %s", getReplicaFullName(0),
			),
			Entry("should return correct status if a replica has status ManagedSeedDeleting",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedDeleting, false, operationshoot.StatusHealthy, true)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, before, nil), true,
				EventWaitingForManagedSeedDeleted, "Waiting for managed seed %s to be deleted", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica seed is not ready",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedPreparingReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, false, operationshoot.StatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting managed seed %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed and return correct status if a replica shoot is not healthy",
				set(0, 1, getReplicaName(0), seedmanagementv1alpha1.SeedNotReadyReason, nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusUnhealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting managed seed %s", getReplicaFullName(0),
			),
			Entry("should delete the managed seed of a deletable replica and return correct status if all replicas are ready",
				set(0, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusHealthy, true)
					r0.EXPECT().DeleteManagedSeed(ctx, gc).Return(nil)
				},
				status(1, 0, 1, getReplicaName(0), seedmanagementv1alpha1.ManagedSeedDeletingReason, now, nil), true,
				EventDeletingManagedSeed, "Deleting managed seed %s", getReplicaFullName(0),
			),
			Entry("should fail if all replicas are ready and there are no deletable replicas",
				set(0, 1, "", "", nil),
				func() {
					expectReplica(r0, 0, StatusManagedSeedRegistered, true, operationshoot.StatusHealthy, false)
				},
				status(1, 1, 1, "", "", now, nil), false,
				gardencorev1beta1.EventReconcileError, "no deletable replicas found",
			),
		)
	})
})

func getReplicaName(ordinal int) string {
	return fmt.Sprintf("%s-%d", name, ordinal)
}

func getReplicaFullName(ordinal int) string {
	return fmt.Sprintf("%s/%s", namespace, getReplicaName(ordinal))
}
