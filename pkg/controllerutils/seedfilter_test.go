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
		c    *mockclient.MockReader

		ctx context.Context

		managedSeed       *seedmanagementv1alpha1.ManagedSeed
		shoot             *gardencorev1beta1.Shoot
		seedOfManagedSeed *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		c = mockclient.NewMockReader(ctrl)

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
				SeedName: pointer.String(seedName),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: pointer.String(seedName),
			},
		}
		seedOfManagedSeed = &gardencorev1beta1.Seed{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
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

		expectGetManagedSeed = func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, ms *seedmanagementv1alpha1.ManagedSeed) error {
					*ms = *managedSeed
					return nil
				},
			)
		}

		expectGetManagedSeedNotFound = func() {
			c.EXPECT().Get(ctx, kutil.Key(namespace, name), gomock.AssignableToTypeOf(&seedmanagementv1alpha1.ManagedSeed{})).DoAndReturn(
				func(_ context.Context, _ client.ObjectKey, _ *seedmanagementv1alpha1.ManagedSeed) error {
					return apierrors.NewNotFound(seedmanagementv1alpha1.Resource("ManagedSeed"), name)
				},
			)
		}
	)

	Describe("#ManagedSeedFilterFunc", func() {
		It("should return false if the specified object is not a managed seed", func() {
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(shoot)).To(BeFalse())
		})

		It("should return false with a shoot that is not found", func() {
			expectGetShootNotFound()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(managedSeed)).To(BeFalse())
		})

		It("should return false with a shoot that is not yet scheduled on a seed", func() {
			shoot.Spec.SeedName = nil
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(managedSeed)).To(BeFalse())
		})

		It("should return true with a shoot that is scheduled on the specified seed", func() {
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return true with a shoot that is scheduled on the specified seed (status different from spec)", func() {
			shoot.Spec.SeedName = pointer.String("foo")
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(managedSeed)).To(BeTrue())
		})

		It("should return false with a shoot that is scheduled on a different seed", func() {
			expectGetShoot()
			f := controllerutils.ManagedSeedFilterFunc(ctx, c, "foo")
			Expect(f(managedSeed)).To(BeFalse())
		})
	})

	Describe("#SeedOfManagedSeedFilterFunc", func() {
		It("should return false if the specified object is not a seed", func() {
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(shoot)).To(BeFalse())
		})

		It("should return false if the seed is not owned by a managed seed", func() {
			expectGetManagedSeedNotFound()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(seedOfManagedSeed)).To(BeFalse())
		})

		It("should return false with a shoot that is not found", func() {
			expectGetManagedSeed()
			expectGetShootNotFound()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(seedOfManagedSeed)).To(BeFalse())
		})

		It("should return false with a shoot that is not yet scheduled on a seed", func() {
			expectGetManagedSeed()
			shoot.Spec.SeedName = nil
			expectGetShoot()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(seedOfManagedSeed)).To(BeFalse())
		})

		It("should return true with a shoot that is scheduled on the specified seed", func() {
			expectGetManagedSeed()
			expectGetShoot()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(seedOfManagedSeed)).To(BeTrue())
		})

		It("should return true with a shoot that is scheduled on the specified seed (status different from spec)", func() {
			expectGetManagedSeed()
			shoot.Spec.SeedName = pointer.String("foo")
			expectGetShoot()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, seedName)
			Expect(f(seedOfManagedSeed)).To(BeTrue())
		})

		It("should return false with a shoot that is scheduled on a different seed", func() {
			expectGetManagedSeed()
			expectGetShoot()
			f := controllerutils.SeedOfManagedSeedFilterFunc(ctx, c, "foo")
			Expect(f(seedOfManagedSeed)).To(BeFalse())
		})
	})

	Describe("BackupEntry", func() {
		var backupEntry *gardencorev1beta1.BackupEntry

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
				f := controllerutils.BackupEntryFilterFunc(seedName)
				Expect(f(shoot)).To(BeFalse())
			})

			DescribeTable("filter BackupEntry by seedName",
				func(specSeedName, statusSeedName *string, filterSeedName string, match gomegatypes.GomegaMatcher) {
					f := controllerutils.BackupEntryFilterFunc(filterSeedName)
					backupEntry.Spec.SeedName = specSeedName
					backupEntry.Status.SeedName = statusSeedName
					Expect(f(backupEntry)).To(match)
				},

				Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName are nil", nil, nil, seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName does not match and BackupEntry.Status.SeedName is nil", pointer.String(otherSeed), nil, seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName and BackupEntry.Status.SeedName do not match", pointer.String(otherSeed), pointer.String(otherSeed), seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName is nil but BackupEntry.Status.SeedName matches", nil, pointer.String(seedName), seedName, BeFalse()),
				Entry("BackupEntry.Spec.SeedName matches and BackupEntry.Status.SeedName is nil", pointer.String(seedName), nil, seedName, BeTrue()),
				Entry("BackupEntry.Spec.SeedName does not match but BackupEntry.Status.SeedName matches", pointer.String(otherSeed), pointer.String(seedName), seedName, BeTrue()),
			)
		})

		Describe("#BackupEntryIsManagedByThisGardenlet", func() {
			DescribeTable("check BackupEntry by seedName",
				func(bucketSeedName string, match gomegatypes.GomegaMatcher) {
					backupEntry.Spec.SeedName = pointer.String(bucketSeedName)
					gc := &config.GardenletConfiguration{
						SeedConfig: &config.SeedConfig{
							SeedTemplate: gardencore.SeedTemplate{
								ObjectMeta: metav1.ObjectMeta{
									Name: seedName,
								},
							},
						},
					}
					Expect(controllerutils.BackupEntryIsManagedByThisGardenlet(backupEntry, gc)).To(match)
				},
				Entry("BackupEntry is not managed by this seed", otherSeed, BeFalse()),
				Entry("BackupEntry is managed by this seed", seedName, BeTrue()),
			)
		})
	})
})
