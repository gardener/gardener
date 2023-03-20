// Copyright 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("Migration", func() {
	Describe("#IsObjectBeingMigrated", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client

			obj        *gardencorev1beta1.BackupEntry
			sourceSeed *gardencorev1beta1.Seed
			seedName   = "seed"

			getSeedNamesFromObject = func(obj client.Object) (*string, *string) {
				backupEntry := obj.(*gardencorev1beta1.BackupEntry)
				return backupEntry.Spec.SeedName, backupEntry.Status.SeedName
			}
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

			sourceSeed = &gardencorev1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: "source-seed",
				},
				Spec: gardencorev1beta1.SeedSpec{
					Settings: &gardencorev1beta1.SeedSettings{
						OwnerChecks: &gardencorev1beta1.SeedSettingOwnerChecks{
							Enabled: true,
						},
					},
				},
			}

			obj = &gardencorev1beta1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: "entry",
				},
				Spec: gardencorev1beta1.BackupEntrySpec{
					SeedName: &seedName,
				},
				Status: gardencorev1beta1.BackupEntryStatus{
					SeedName: &sourceSeed.Name,
				},
			}
		})

		It("should return false if status.seedName is nil", func() {
			obj.Status.SeedName = nil

			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeFalse())
		})

		It("should return false if spec.seedName and status.seedName are equal", func() {
			obj.Status.SeedName = pointer.String("seed")

			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeFalse())
		})

		It("should return false if the obj does not belong to this seed", func() {
			obj.Spec.SeedName = pointer.String("another-seed")

			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeFalse())
		})

		It("should return false if the get call on source seed fails", func() {
			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeFalse())
		})

		It("should return true if the source seed has owner checks enabled", func() {
			Expect(fakeClient.Create(ctx, sourceSeed)).To(Succeed())

			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeTrue())
		})

		It("should return false if the source seed has owner checks disabled", func() {
			sourceSeed.Spec.Settings.OwnerChecks.Enabled = false
			Expect(fakeClient.Create(ctx, sourceSeed)).To(Succeed())

			Expect(IsObjectBeingMigrated(ctx, fakeClient, obj, seedName, getSeedNamesFromObject)).To(BeFalse())
		})
	})

	Describe("#GetResponsibleSeedName", func() {
		It("returns nothing if spec.seedName is not set", func() {
			Expect(GetResponsibleSeedName(nil, nil)).To(BeEmpty())
			Expect(GetResponsibleSeedName(nil, pointer.String("status"))).To(BeEmpty())
		})

		It("returns spec.seedName if status.seedName is not set", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), nil)).To(Equal("spec"))
		})

		It("returns status.seedName if the seedNames differ", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), pointer.String("status"))).To(Equal("status"))
		})

		It("returns the seedName if both are equal", func() {
			Expect(GetResponsibleSeedName(pointer.String("spec"), pointer.String("spec"))).To(Equal("spec"))
		})
	})
})
