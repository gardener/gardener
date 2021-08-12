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

package seed_test

import (
	"context"
	"encoding/json"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	mockkubernetes "github.com/gardener/gardener/pkg/client/kubernetes/mock"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/seed"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var _ = Describe("BackupBucketReconciler", func() {
	var (
		ctx  = context.TODO()
		ctrl *gomock.Controller
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	Describe("#Reconcile", func() {
		var (
			k8sGardenClient *mockkubernetes.MockInterface
			c               *mockclient.MockClient
			sw              *mockclient.MockStatusWriter

			seed, seedPatch *gardencorev1beta1.Seed
			bbs             []gardencorev1beta1.BackupBucket

			control reconcile.Reconciler
		)

		BeforeEach(func() {
			k8sGardenClient = mockkubernetes.NewMockInterface(ctrl)
			c = mockclient.NewMockClient(ctrl)
			sw = mockclient.NewMockStatusWriter(ctrl)
			c.EXPECT().Status().Return(sw).AnyTimes()
			seed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "seed",
				},
			}

			seedPatch = &gardencorev1beta1.Seed{}

			k8sGardenClient.EXPECT().Client().Return(c).AnyTimes()
		})

		JustBeforeEach(func() {
			sw.EXPECT().Patch(gomock.Any(), gomock.AssignableToTypeOf(seed), gomock.Any()).DoAndReturn(
				func(_ context.Context, obj client.Object, patch client.Patch, _ ...client.PatchOption) error {
					patchData, err := patch.Data(obj)
					Expect(err).NotTo(HaveOccurred())
					Expect(json.Unmarshal(patchData, seedPatch)).To(Succeed())
					return nil
				})

			control = NewDefaultBackupBucketControl(logger.NewNopLogger(), k8sGardenClient)

			c.EXPECT().Get(ctx, kutil.Key(seed.Name), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(func(_ context.Context, _ client.ObjectKey, obj *gardencorev1beta1.Seed) error {
				*obj = *seed
				return nil
			})
		})

		Context("when Seed has healthy backup buckets", func() {
			BeforeEach(func() {
				bbs = []gardencorev1beta1.BackupBucket{
					createBackupBucket("1", seed.Name, nil),
					createBackupBucket("2", "fooSeed", nil),
					createBackupBucket("3", "barSeed", nil),
					createBackupBucket("4", seed.Name, nil),
				}

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucketList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.BackupBucketList, opts ...client.ListOption) error {
					(&gardencorev1beta1.BackupBucketList{Items: bbs}).DeepCopyInto(list)
					return nil
				})
			})

			It("should set condition to `True` when none was given", func() {
				result, err := control.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(seedPatch.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Message": Equal("Backup Buckets are available."),
						"Reason":  Equal("BackupBucketsAvailable"),
						"Status":  Equal(gardencorev1beta1.ConditionTrue),
						"Type":    Equal(gardencorev1beta1.SeedBackupBucketsReady),
					}),
				))
			})

			It("should set condition to `True` when one was false", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Message: "foo",
						Reason:  "bar",
						Status:  gardencorev1beta1.ConditionTrue,
						Type:    gardencorev1beta1.SeedExtensionsReady,
					},
					{
						Message: "foo",
						Reason:  "bar",
						Status:  gardencorev1beta1.ConditionFalse,
						Type:    gardencorev1beta1.SeedBackupBucketsReady,
					},
				}

				result, err := control.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(seedPatch.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Message": Equal("Backup Buckets are available."),
						"Reason":  Equal("BackupBucketsAvailable"),
						"Status":  Equal(gardencorev1beta1.ConditionTrue),
						"Type":    Equal(gardencorev1beta1.SeedBackupBucketsReady),
					}),
				))
			})
		})

		Context("when Seed has unhealthy backup buckets", func() {
			BeforeEach(func() {
				bbs = []gardencorev1beta1.BackupBucket{
					createBackupBucket("1", seed.Name, &gardencorev1beta1.LastError{Description: "foo error"}),
					createBackupBucket("2", "fooSeed", nil),
					createBackupBucket("3", seed.Name, &gardencorev1beta1.LastError{Description: "bar error"}),
					createBackupBucket("4", "barSeed", nil),
				}

				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucketList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.BackupBucketList, opts ...client.ListOption) error {
					(&gardencorev1beta1.BackupBucketList{Items: bbs}).DeepCopyInto(list)
					return nil
				})
			})

			It("should set condition to `False`", func() {
				result, err := control.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(seedPatch.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Message": SatisfyAll(ContainSubstring("Name: 1, Error: foo error"), ContainSubstring("Name: 3, Error: bar error")),
						"Reason":  Equal("BackupBucketsError"),
						"Status":  Equal(gardencorev1beta1.ConditionFalse),
						"Type":    Equal(gardencorev1beta1.SeedBackupBucketsReady),
					}),
				))
			})
		})

		Context("when a Seed's unhealthy backup bucket switches", func() {
			BeforeEach(func() {
				bbs = []gardencorev1beta1.BackupBucket{
					createBackupBucket("1", seed.Name, &gardencorev1beta1.LastError{Description: "foo error"}),
					createBackupBucket("2", seed.Name, nil),
				}
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucketList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.BackupBucketList, opts ...client.ListOption) error {
					(&gardencorev1beta1.BackupBucketList{Items: bbs}).DeepCopyInto(list)
					return nil
				})
			})

			It("should set condition to `False` and remove successful bucket from message", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Message: "The following BackupBuckets have issues:\n* Name: 2, Error: some error",
						Reason:  "BackupBucketsError",
						Status:  gardencorev1beta1.ConditionFalse,
						Type:    gardencorev1beta1.SeedBackupBucketsReady,
					},
				}
				result, err := control.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(seedPatch.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Message": Equal("The following BackupBuckets have issues:\n* Name: 1, Error: foo error"),
						"Type":    Equal(gardencorev1beta1.SeedBackupBucketsReady),
					}),
				))
			})
		})

		Context("when a Seed's backup buckets are gone", func() {
			BeforeEach(func() {
				bbs = []gardencorev1beta1.BackupBucket{
					createBackupBucket("1", "fooSeed", &gardencorev1beta1.LastError{Description: "foo error"}),
					createBackupBucket("2", "barSeed", nil),
				}
				c.EXPECT().List(ctx, gomock.AssignableToTypeOf(&gardencorev1beta1.BackupBucketList{})).DoAndReturn(func(ctx context.Context, list *gardencorev1beta1.BackupBucketList, opts ...client.ListOption) error {
					(&gardencorev1beta1.BackupBucketList{Items: bbs}).DeepCopyInto(list)
					return nil
				})
			})

			It("should set condition to `False` and remove successful bucket from message", func() {
				seed.Status.Conditions = []gardencorev1beta1.Condition{
					{
						Message: "Backup Buckets are available.",
						Reason:  "BackupBucketsAvailable",
						Status:  gardencorev1beta1.ConditionTrue,
						Type:    gardencorev1beta1.SeedBackupBucketsReady,
					},
				}
				result, err := control.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(seed)})
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(reconcile.Result{}))
				Expect(seedPatch.Status.Conditions).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Message": Equal("Backup Buckets are gone."),
						"Reason":  Equal("BackupBucketsGone"),
						"Status":  Equal(gardencorev1beta1.ConditionUnknown),
						"Type":    Equal(gardencorev1beta1.SeedBackupBucketsReady),
					}),
				))
			})
		})
	})
})

func createBackupBucket(name, seedName string, lastErr *gardencorev1beta1.LastError) gardencorev1beta1.BackupBucket {
	return gardencorev1beta1.BackupBucket{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: gardencorev1beta1.BackupBucketSpec{
			SeedName: pointer.String(seedName),
		},
		Status: gardencorev1beta1.BackupBucketStatus{
			LastError: lastErr,
		},
	}
}
