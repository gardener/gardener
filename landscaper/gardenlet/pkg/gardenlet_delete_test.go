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

package pkg

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/logger"
	mockclient "github.com/gardener/gardener/pkg/mock/controller-runtime/client"
	mock "github.com/gardener/gardener/pkg/mock/gardener/client/kubernetes"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/retry"
	retryfake "github.com/gardener/gardener/pkg/utils/retry/fake"
	"github.com/gardener/gardener/pkg/utils/test"

	"github.com/gardener/gardener/landscaper/gardenlet/pkg/apis/imports"
)

var _ = Describe("Gardenlet Landscaper deletion testing", func() {

	Describe("Tests that require a mock client", func() {
		var (
			landscaper Landscaper
			seed       = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{
				Name: "sweet-seed",
			}}

			mockController      *gomock.Controller
			mockGardenClient    *mockclient.MockClient
			mockGardenInterface *mock.MockInterface
			mockSeedInterface   *mock.MockInterface
			mockChartApplier    *mock.MockChartApplier

			ctx         = context.TODO()
			cleanupFunc func()
		)

		BeforeEach(func() {
			mockController = gomock.NewController(GinkgoT())

			mockGardenClient = mockclient.NewMockClient(mockController)
			mockGardenInterface = mock.NewMockInterface(mockController)

			mockSeedInterface = mock.NewMockInterface(mockController)
			mockChartApplier = mock.NewMockChartApplier(mockController)

			landscaper = Landscaper{
				gardenClient: mockGardenInterface,
				seedClient:   mockSeedInterface,
				log:          logger.NewNopLogger().WithContext(ctx),
				Imports: &imports.Imports{},
				gardenletConfiguration: &gardenletconfigv1alpha1.GardenletConfiguration{
					SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
						SeedTemplate: gardencorev1beta1.SeedTemplate{ObjectMeta: seed.ObjectMeta},
					},
				},
			}

			waiter := &retryfake.Ops{MaxAttempts: 1}
			cleanupFunc = test.WithVars(
				&retry.UntilTimeout, waiter.UntilTimeout,
			)
		})

		AfterEach(func() {
			mockController.Finish()
			cleanupFunc()
		})

		Describe("#Delete", func() {
			var (
				emptyShootList             = &gardencorev1beta1.ShootList{}
				shootListSeedInUseByShoots = gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							Spec: gardencorev1beta1.ShootSpec{
								SeedName: pointer.StringPtr(seed.Name),
							},
						},
					},
				}
				shootListSeedNotInUseByAnyShoot = gardencorev1beta1.ShootList{
					Items: []gardencorev1beta1.Shoot{
						{
							Spec: gardencorev1beta1.ShootSpec{
								SeedName: pointer.StringPtr("other-seed"),
							},
						},
					},
				}
			)

			It("fails to list Shoots", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().List(ctx, emptyShootList).Return(fmt.Errorf("fake error"))

				err := landscaper.Delete(ctx)
				Expect(err).To(HaveOccurred())
			})

			It("fails to check if Seed still exists", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().List(ctx, emptyShootList).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(fmt.Errorf("fake error"))

				err := landscaper.Delete(ctx)
				Expect(err).To(HaveOccurred())
			})

			It("fails because Seed is still used by Shoots", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().List(ctx, emptyShootList).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					*list = shootListSeedInUseByShoots
					return nil
				})
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				err := landscaper.Delete(ctx)
				Expect(err).To(HaveOccurred())
			})

			// more test cases for #waitForSeedDeletion below
			It("fails because it fails to wait for the Seed to be deleted (Seed still exists)", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().List(ctx, emptyShootList).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					*list = shootListSeedNotInUseByAnyShoot
					return nil
				})
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				// waitForSeedDeletion
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				err := landscaper.Delete(ctx)
				Expect(err).To(HaveOccurred())
			})

			It("should successfully delete the Gardenlet resources from the Seed cluster", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().List(ctx, emptyShootList).DoAndReturn(func(_ context.Context, list *gardencorev1beta1.ShootList, _ ...client.ListOption) error {
					*list = shootListSeedNotInUseByAnyShoot
					return nil
				})
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				// waitForSeedDeletion
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(apierrors.NewNotFound(schema.GroupResource{}, seed.Name))

				// chart applier
				mockSeedInterface.EXPECT().ChartApplier().Return(mockChartApplier)

				defer test.WithVars(
					&GetChartPath, func() string { return chartsRootPath },
				)()

				mockChartApplier.EXPECT().Delete(ctx, filepath.Join(chartsRootPath, "gardener", "gardenlet"), "garden", "gardenlet", kubernetes.Values(map[string]interface{}{}))

				err := landscaper.Delete(ctx)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("#waitForSeedDeletion", func() {
			It("fails to set deletion timestamp on seed", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(fmt.Errorf("fake error"))

				err := landscaper.waitForSeedDeletion(ctx, seed)
				Expect(err).To(HaveOccurred())
			})

			It("fails to check if Seed exists", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(fmt.Errorf("fake error"))

				err := landscaper.waitForSeedDeletion(ctx, seed)
				Expect(err).To(HaveOccurred())
			})

			It("fails - waiting for Seed to be deleted but still exists", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				err := landscaper.waitForSeedDeletion(ctx, seed)
				Expect(err).To(HaveOccurred())
			})

			It("successfully wait for Seed deletion", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Delete(ctx, seed).Return(nil)
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(apierrors.NewNotFound(schema.GroupResource{}, seed.Name))

				err := landscaper.waitForSeedDeletion(ctx, seed)
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Describe("#seedExists", func() {
			It("the requested seed exists", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(nil)

				exists, err := landscaper.seedExists(ctx, seed)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(Equal(true))

			})

			It("the requested seed does NOT exist", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(apierrors.NewNotFound(schema.GroupResource{}, seed.Name))

				exists, err := landscaper.seedExists(ctx, seed)
				Expect(err).ToNot(HaveOccurred())
				Expect(exists).To(Equal(false))
			})

			It("expecting an error", func() {
				mockGardenInterface.EXPECT().Client().Return(mockGardenClient)
				mockGardenClient.EXPECT().Get(ctx, kutil.Key(seed.Namespace, seed.Name), seed).Return(fmt.Errorf("fake error"))

				exists, err := landscaper.seedExists(ctx, seed)
				Expect(err).To(HaveOccurred())
				Expect(exists).To(Equal(false))
			})
		})
	})

	var (
		shoot1 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot1",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.StringPtr("seed1"),
			},
		}

		shoot2 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot2",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.StringPtr("seed1"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName: pointer.StringPtr("seed2"),
			},
		}

		shoot3 = gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "shoot3",
				Namespace: "garden-pr1",
			},
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: nil,
			},
		}

		shoots = []gardencorev1beta1.Shoot{
			shoot1,
			shoot2,
			shoot3,
		}
	)

	DescribeTable("#isSeedUsedByAnyShoot",
		func(seedName string, expected bool) {
			Expect(isSeedUsedByAnyShoot(seedName, shoots)).To(Equal(expected))
		},
		Entry("is used by shoot", "seed1", true),
		Entry("is used by shoot in migration", "seed2", true),
		Entry("is unused", "seed3", false),
	)
})
