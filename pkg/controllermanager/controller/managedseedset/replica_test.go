// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package managedseedset_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	seedmanagementv1alpha1constants "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1/constants"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/managedseedset"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	mockclient "github.com/gardener/gardener/third_party/mock/controller-runtime/client"
)

const (
	ordinal     = int32(42)
	replicaName = name + "-42"
)

var _ = Describe("Replica", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient
		ctx  context.Context

		managedSeedSet *seedmanagementv1alpha1.ManagedSeedSet

		now = metav1.Now()
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)
		ctx = context.TODO()

		managedSeedSet = &seedmanagementv1alpha1.ManagedSeedSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSetSpec{
				Template: seedmanagementv1alpha1.ManagedSeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: seedmanagementv1alpha1.ManagedSeedSpec{
						Gardenlet: seedmanagementv1alpha1.GardenletConfig{
							Config: runtime.RawExtension{
								Object: &gardenletconfigv1alpha1.GardenletConfiguration{
									SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
										SeedTemplate: gardencorev1beta1.SeedTemplate{
											Spec: gardencorev1beta1.SeedSpec{
												Ingress: &gardencorev1beta1.Ingress{
													Domain: "ingress.replica-name.example.com",
												},
											},
										},
									},
								},
							},
						},
					},
				},
				ShootTemplate: gardencorev1beta1.ShootTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"foo": "bar",
						},
					},
					Spec: gardencorev1beta1.ShootSpec{
						DNS: &gardencorev1beta1.DNS{
							Domain: ptr.To("replica-name.example.com"),
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		shoot = func(
			deletionTimestamp *metav1.Time,
			lastOperationType gardencorev1beta1.LastOperationType,
			lastOperationState gardencorev1beta1.LastOperationState,
			shootStatus gardenerutils.ShootStatus,
			protected bool,
		) *gardencorev1beta1.Shoot {
			labels := make(map[string]string)
			if shootStatus != "" {
				labels[v1beta1constants.ShootStatus] = string(shootStatus)
			}
			annotations := make(map[string]string)
			if protected {
				annotations[seedmanagementv1alpha1constants.AnnotationProtectFromDeletion] = "true"
			}
			var lastOperation *gardencorev1beta1.LastOperation
			if lastOperationType != "" && lastOperationState != "" {
				lastOperation = &gardencorev1beta1.LastOperation{
					Type:  lastOperationType,
					State: lastOperationState,
				}
			}
			return &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					Namespace:         namespace,
					DeletionTimestamp: deletionTimestamp,
					Labels:            labels,
					Annotations:       annotations,
				},
				Status: gardencorev1beta1.ShootStatus{
					LastOperation: lastOperation,
				},
			}
		}
		managedSeed = func(deletionTimestamp *metav1.Time, seedRegistered, protected bool) *seedmanagementv1alpha1.ManagedSeed {
			annotations := make(map[string]string)
			if protected {
				annotations[seedmanagementv1alpha1constants.AnnotationProtectFromDeletion] = "true"
			}
			var conditions []gardencorev1beta1.Condition
			if seedRegistered {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   seedmanagementv1alpha1.SeedRegistered,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			return &seedmanagementv1alpha1.ManagedSeed{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					Namespace:         namespace,
					DeletionTimestamp: deletionTimestamp,
					Annotations:       annotations,
				},
				Status: seedmanagementv1alpha1.ManagedSeedStatus{
					Conditions: conditions,
				},
			}
		}
		seed = func(deletionTimestamp *metav1.Time, gardenletReady, backupBucketsReady, seedSystemComponentsHealthy bool) *gardencorev1beta1.Seed {
			var conditions []gardencorev1beta1.Condition
			if gardenletReady {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedGardenletReady,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			if backupBucketsReady {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedBackupBucketsReady,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			if seedSystemComponentsHealthy {
				conditions = append(conditions, gardencorev1beta1.Condition{
					Type:   gardencorev1beta1.SeedSystemComponentsHealthy,
					Status: gardencorev1beta1.ConditionTrue,
				})
			}
			return &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name:              replicaName,
					DeletionTimestamp: deletionTimestamp,
				},
				Status: gardencorev1beta1.SeedStatus{
					Conditions: conditions,
				},
			}
		}
	)

	DescribeTable("#GetName",
		func(shoot *gardencorev1beta1.Shoot, name string) {
			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			Expect(replica.GetName()).To(Equal(name))
		},
		Entry("should return an empty string", nil, ""),
		Entry("should return the shoot name", shoot(nil, "", "", "", false), replicaName),
	)

	DescribeTable("#GetFullName",
		func(shoot *gardencorev1beta1.Shoot, fullName string) {
			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			Expect(replica.GetFullName()).To(Equal(fullName))
		},
		Entry("should return an empty string", nil, ""),
		Entry("should return the shoot full name", shoot(nil, "", "", "", false), namespace+"/"+replicaName),
	)

	DescribeTable("#GetObjectKey",
		func(shoot *gardencorev1beta1.Shoot, expected client.ObjectKey) {
			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			Expect(replica.GetObjectKey()).To(Equal(expected))
		},
		Entry("should return an empty key", nil, client.ObjectKey{}),
		Entry("should return the shoot's key", shoot(nil, "", "", "", false), client.ObjectKey{Namespace: namespace, Name: replicaName}),
	)

	DescribeTable("#GetOrdinal",
		func(shoot *gardencorev1beta1.Shoot, ordinal int32) {
			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			Expect(replica.GetOrdinal()).To(Equal(ordinal))
		},
		Entry("should return -1", nil, int32(-1)),
		Entry("should return the ordinal from the shoot name", shoot(nil, "", "", "", false), ordinal),
	)

	DescribeTable("#GetStatus",
		func(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, status ReplicaStatus) {
			replica := NewReplica(managedSeedSet, shoot, managedSeed, nil, false)
			Expect(replica.GetStatus()).To(Equal(status))
		},
		Entry("should return Unknown", nil, nil, StatusUnknown),
		Entry("should return ShootReconciled",
			shoot(nil, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateSucceeded, "", false), nil, StatusShootReconciled),
		Entry("should return ShootReconcileFailed",
			shoot(nil, gardencorev1beta1.LastOperationTypeReconcile, gardencorev1beta1.LastOperationStateFailed, "", false), nil, StatusShootReconcileFailed),
		Entry("should return ShootDeleteFailed",
			shoot(&now, gardencorev1beta1.LastOperationTypeDelete, gardencorev1beta1.LastOperationStateFailed, "", false), nil, StatusShootDeleteFailed),
		Entry("should return ShootReconciling",
			shoot(nil, "", "", "", false), nil, StatusShootReconciling),
		Entry("should return ShootDeleting",
			shoot(&now, "", "", "", false), nil, StatusShootDeleting),
		Entry("should return ManagedSeedRegistered",
			shoot(nil, "", "", "", false), managedSeed(nil, true, false), StatusManagedSeedRegistered),
		Entry("should return ManagedSeedPreparing",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), StatusManagedSeedPreparing),
		Entry("should return ManagedSeedDeleting",
			shoot(nil, "", "", "", false), managedSeed(&now, false, false), StatusManagedSeedDeleting),
	)

	DescribeTable("#IsSeedReady",
		func(seed *gardencorev1beta1.Seed, seedReady bool) {
			replica := NewReplica(managedSeedSet, shoot(nil, "", "", "", false),
				managedSeed(nil, true, false), seed, false)
			Expect(replica.IsSeedReady()).To(Equal(seedReady))
		},
		Entry("should return false", seed(nil, false, false, false), false),
		Entry("should return true", seed(nil, true, false, true), true),
		Entry("should return true", seed(nil, true, true, true), true),
		Entry("should return false", seed(nil, true, true, false), false),
		Entry("should return false", seed(&now, true, true, true), false),
	)

	DescribeTable("#GetShootHealthStatus",
		func(shoot *gardencorev1beta1.Shoot, shootStatus gardenerutils.ShootStatus) {
			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			Expect(replica.GetShootHealthStatus()).To(Equal(shootStatus))
		},
		Entry("should return unhealthy",
			nil, gardenerutils.ShootStatusUnhealthy),
		Entry("should return progressing",
			shoot(nil, "", "", "", false), gardenerutils.ShootStatusProgressing),
		Entry("should return healthy",
			shoot(nil, "", "", gardenerutils.ShootStatusHealthy, false), gardenerutils.ShootStatusHealthy),
		Entry("should return progressing",
			shoot(nil, "", "", gardenerutils.ShootStatusProgressing, false), gardenerutils.ShootStatusProgressing),
		Entry("should return unhealthy",
			shoot(nil, "", "", gardenerutils.ShootStatusUnhealthy, false), gardenerutils.ShootStatusUnhealthy),
		Entry("should return unknown",
			shoot(nil, "", "", gardenerutils.ShootStatusUnknown, false), gardenerutils.ShootStatusUnknown),
	)

	DescribeTable("#IsDeletable",
		func(shoot *gardencorev1beta1.Shoot, managedSeed *seedmanagementv1alpha1.ManagedSeed, hasScheduledShoots, deletable bool) {
			replica := NewReplica(managedSeedSet, shoot, managedSeed, nil, hasScheduledShoots)
			Expect(replica.IsDeletable()).To(Equal(deletable))
		},
		Entry("should return true",
			nil, nil, false, true),
		Entry("should return true",
			shoot(nil, "", "", "", false), nil, false, true),
		Entry("should return true",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), false, true),
		Entry("should return false",
			shoot(nil, "", "", "", true), nil, false, false),
		Entry("should return false",
			shoot(nil, "", "", "", false), managedSeed(nil, false, true), false, false),
		Entry("should return false",
			shoot(nil, "", "", "", false), managedSeed(nil, false, false), true, false),
	)

	Describe("#CreateShoot", func() {
		It("should create the shoot", func() {
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ ...client.CreateOption) error {
					Expect(s).To(Equal(&gardencorev1beta1.Shoot{
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicaName,
							Namespace: namespace,
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeedSet, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
							},
						},
						Spec: gardencorev1beta1.ShootSpec{
							DNS: &gardencorev1beta1.DNS{
								Domain: ptr.To(replicaName + ".example.com"),
							},
						},
					}))
					return nil
				},
			)

			replica := NewReplica(managedSeedSet, nil, nil, nil, false)
			err := replica.CreateShoot(ctx, c, ordinal)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#CreateManagedSeed", func() {
		It("should create the managed seed", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Create(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.CreateOption) error {
					Expect(ms).To(Equal(&seedmanagementv1alpha1.ManagedSeed{
						ObjectMeta: metav1.ObjectMeta{
							Name:      replicaName,
							Namespace: namespace,
							Labels: map[string]string{
								"foo": "bar",
							},
							OwnerReferences: []metav1.OwnerReference{
								*metav1.NewControllerRef(managedSeedSet, seedmanagementv1alpha1.SchemeGroupVersion.WithKind("ManagedSeedSet")),
							},
						},
						Spec: seedmanagementv1alpha1.ManagedSeedSpec{
							Shoot: &seedmanagementv1alpha1.Shoot{
								Name: replicaName,
							},
							Gardenlet: seedmanagementv1alpha1.GardenletConfig{
								Config: runtime.RawExtension{
									Object: &gardenletconfigv1alpha1.GardenletConfiguration{
										SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
											SeedTemplate: gardencorev1beta1.SeedTemplate{
												Spec: gardencorev1beta1.SeedSpec{
													Ingress: &gardencorev1beta1.Ingress{
														Domain: "ingress." + replicaName + ".example.com",
													},
												},
											},
										},
									},
								},
							},
						},
					}))
					return nil
				},
			)

			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			err := replica.CreateManagedSeed(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#DeleteShoot", func() {
		It("should clean the retries, confirm the deletion, and delete the shoot", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					Expect(s.Annotations).To(HaveKeyWithValue(v1beta1constants.ConfirmationDeletion, "true"))
					*shoot = *s
					return nil
				},
			)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ ...client.DeleteOption) error {
					Expect(s).To(Equal(shoot))
					return nil
				},
			)

			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			err := replica.DeleteShoot(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#DeleteManagedSeed", func() {
		It("should delete the managed seed", func() {
			managedSeed := managedSeed(nil, false, false)
			c.EXPECT().Delete(ctx, gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, ms *seedmanagementv1alpha1.ManagedSeed, _ ...client.DeleteOption) error {
					Expect(ms).To(Equal(managedSeed))
					return nil
				},
			)

			replica := NewReplica(managedSeedSet, nil, managedSeed, nil, false)
			err := replica.DeleteManagedSeed(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#RetryShoot", func() {
		It("should managedSeedSet the operation to retry and the retries to 1", func() {
			shoot := shoot(nil, "", "", "", false)
			c.EXPECT().Patch(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{}), gomock.Any()).DoAndReturn(
				func(_ context.Context, s *gardencorev1beta1.Shoot, _ client.Patch, _ ...client.PatchOption) error {
					Expect(s.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenerOperation, v1beta1constants.ShootOperationRetry))
					return nil
				},
			)

			replica := NewReplica(managedSeedSet, shoot, nil, nil, false)
			err := replica.RetryShoot(ctx, c)
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
