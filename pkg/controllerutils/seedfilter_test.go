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

package controllerutils_test

import (
	"context"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	seedmanagementv1alpha1 "github.com/gardener/gardener/pkg/apis/seedmanagement/v1alpha1"
	"github.com/gardener/gardener/pkg/controllerutils"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	name      = "test"
	namespace = "garden"
	seedName  = "test-seed"
	otherSeed = "new-test-seed"
)

var _ = Describe("seedfilter", func() {
	var (
		ctrl *gomock.Controller
		c    *mockclient.MockClient

		ctx context.Context

		managedSeed *seedmanagementv1alpha1.ManagedSeed
		shoot       *gardencorev1beta1.Shoot
		seed        *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockClient(ctrl)

		ctx = context.TODO()

		managedSeed = &seedmanagementv1alpha1.ManagedSeed{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: seedmanagementv1alpha1.ManagedSeedSpec{
				Shoot: &seedmanagementv1alpha1.Shoot{
					Name: name,
				},
			},
		}
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.StringPtr(seedName),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: pointer.StringPtr(seedName),
			},
		}
		seed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"test-label": "test",
				},
			},
		}
	})

	AfterEach(func() {
		ctrl.Finish()
	})

	var (
		expectGetShoot = func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Shoot) error {
					*s = *shoot
					return nil
				},
			)
		}

		expectGetShootNotFound = func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&gardencorev1beta1.Shoot{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *gardencorev1beta1.Shoot) error {
					return apierrors.NewNotFound(gardencorev1beta1.Resource("shoot"), name)
				},
			)
		}

		expectGetSeed = func() {
			c.EXPECT().Get(ctx, kutil.Key(seedName), gomock.AssignableToTypeOf(&gardencorev1beta1.Seed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, s *gardencorev1beta1.Seed) error {
					*s = *seed
					return nil
				},
			)
		}
	)

	Describe("#ManagedSeedFilterFunc", func() {
		It("should return false if the specified object is not a managed seed", func() {
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName, nil)
			Expect(f(shoot)).To(BeFalse())
		})

		It("should return false with a shoot that is not found", func() {
			expectGetShootNotFound()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName, nil)
			Expect(f(managedSeed)).To(BeFalse())
		})

		It("should return false with a shoot that is not yet scheduled on a seed", func() {
			shoot.Spec.SeedName = nil
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName, nil)
			Expect(f(managedSeed)).To(BeFalse())
		})

		It("should return true with a shoot that is scheduled on the specified seed", func() {
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName, nil)
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return true with a shoot that is scheduled on the specified seed (status different from spec)", func() {
			shoot.Spec.SeedName = pointer.StringPtr("foo")
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName, nil)
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return false with a shoot that is scheduled on a different seed", func() {
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, "foo", nil)
			Expect(f(managedSeed)).To(BeFalse())
		})

		It("should return true with a shoot that is scheduled on a seed selected by the specified label selector", func() {
			expectGetShoot()
			expectGetSeed()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, "", &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test",
				},
			})
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return true with a shoot that is scheduled on a seed selected by the specified label selector (status different from spec)", func() {
			shoot.Spec.SeedName = pointer.StringPtr("foo")
			expectGetShoot()
			expectGetSeed()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, "", &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test",
				},
			})
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return false with a shoot that is scheduled on a seed not selected by the specified label selector", func() {
			expectGetShoot()
			expectGetSeed()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, "", &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"foo": "bar",
				},
			})
			Expect(f(managedSeed)).To(BeFalse())
		})
	})

	Describe("BackupEntry", func() {
		var (
			backupEntry       *gardencorev1beta1.BackupEntry
			seedLabelSelector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "test",
				},
			}
			otherSeedLabelSelector = &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-label": "new-test",
				},
			}
		)

		BeforeEach(func() {
			backupEntry = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
			}
		})

		Describe("#BackupEntryFilterFunc", func() {
			It("should return false if the specified object is not a BackupEntry", func() {
				f := controllerutils.BackupEntryFilterFunc(ctx, c, seedName, nil)
				Expect(f(shoot)).To(BeFalse())
			})

			DescribeTable("filter BackupEntry by seedName",
				func(specSeedName, statusSeedName *string, filterSeedName string, match gomegatypes.GomegaMatcher) {
					f := controllerutils.BackupEntryFilterFunc(ctx, c, filterSeedName, nil)
					backupEntry.Spec.SeedName = specSeedName
					backupEntry.Status.SeedName = statusSeedName
					Expect(f(backupEntry)).To(match)
				},

				Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName are nil", nil, nil, seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName does not match and BackupEntry.Status.SeedName is nil", pointer.StringPtr(otherSeed), nil, seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName do not match", pointer.StringPtr(otherSeed), pointer.StringPtr(otherSeed), seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName is nil but BackupEntry.Status.SeedName matches", nil, pointer.StringPtr(seedName), seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName matches and BackupEntry.Status.SeedName is nil", pointer.StringPtr(seedName), nil, seedName, BeTrue()),
				Entry("BackupEntry.Spec.SeedName does not match but BackupEntry.Status.SeedName matches", pointer.StringPtr(otherSeed), pointer.StringPtr(seedName), seedName, BeTrue()),
			)

			DescribeTable("filter BackupEntry by Seed label selector",
				func(specSeedName, statusSeedName *string, labelSelector *metav1.LabelSelector, match gomegatypes.GomegaMatcher) {
					expectGetSeed()
					f := controllerutils.BackupEntryFilterFunc(ctx, c, "", labelSelector)
					backupEntry.Spec.SeedName = specSeedName
					backupEntry.Status.SeedName = statusSeedName
					Expect(f(backupEntry)).To(match)
				},
				Entry("BackupEntry.Spec.SeedName does not match and BackupEntry.Status.SeedName is nil", pointer.StringPtr(seedName), nil, otherSeedLabelSelector, BeFalse()),
				Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName do not match", pointer.StringPtr(seedName), pointer.StringPtr(seedName), otherSeedLabelSelector, BeFalse()),
				Entry("BackupEntry.Spec.SeedName matches and BackupEntry.Status.SeedName is nil", pointer.StringPtr(seedName), nil, seedLabelSelector, BeTrue()),
				Entry("BackupEntry.Spec.SeedName does not match but BackupEntry.Status.SeedName matches", pointer.StringPtr(otherSeed), pointer.StringPtr(seedName), seedLabelSelector, BeTrue()),
			)
		})

		Describe("#BackupEntryIsManagedByThisGardenlet", func() {
			DescribeTable("check BackupEntry by seedName",
				func(bucketSeedName string, match gomegatypes.GomegaMatcher) {
					backupEntry.Spec.SeedName = pointer.StringPtr(bucketSeedName)
					gc := &config.GardenletConfiguration{
						SeedConfig: &config.SeedConfig{
							SeedTemplate: gardencore.SeedTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: seedName,
								},
							},
						},
					}
					Expect(controllerutils.BackupEntryIsManagedByThisGardenlet(ctx, c, backupEntry, gc)).To(match)
				},
				Entry("BackupEntry is not managed by this seed", otherSeed, BeFalse()),
				Entry("BackupEntry is managed by this seed", seedName, BeTrue()),
			)

			DescribeTable("check BackupEntry by seed label selector",
				func(labelSelector *metav1.LabelSelector, match gomegatypes.GomegaMatcher) {
					backupEntry.Spec.SeedName = pointer.StringPtr(seedName)
					gc := &config.GardenletConfiguration{
						SeedSelector: labelSelector,
					}
					expectGetSeed()
					Expect(controllerutils.BackupEntryIsManagedByThisGardenlet(ctx, c, backupEntry, gc)).To(match)
				},
				Entry("BackupEntry is not managed by this seed", otherSeedLabelSelector, BeFalse()),
				Entry("BackupEntry is managed by this seed", seedLabelSelector, BeTrue()),
			)
		})
	})
})
