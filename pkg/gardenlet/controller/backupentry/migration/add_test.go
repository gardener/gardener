// Copyright (c) 2022 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package migration_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/backupentry/migration"
)

var _ = Describe("Add", func() {
	var (
		ctx         = context.TODO()
		reconciler  *Reconciler
		backupEntry *gardencorev1beta1.BackupEntry
		entryName   = "entry"
		fakeClient  client.Client
		sourceSeed  *gardencorev1beta1.Seed
	)

	BeforeEach(func() {
		fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()

		reconciler = &Reconciler{
			GardenClient: fakeClient,
			Config: config.GardenletConfiguration{
				SeedConfig: &config.SeedConfig{
					SeedTemplate: gardencore.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name: "seed",
						},
						Spec: gardencore.SeedSpec{
							Settings: &gardencore.SeedSettings{
								OwnerChecks: &gardencore.SeedSettingOwnerChecks{
									Enabled: true,
								},
							},
						},
					},
				},
			}}

		backupEntry = &gardencorev1beta1.BackupEntry{
			ObjectMeta: metav1.ObjectMeta{
				Name: entryName,
			},
			Spec: gardencorev1beta1.BackupEntrySpec{
				SeedName: pointer.String("seed"),
			},
			Status: gardencorev1beta1.BackupEntryStatus{
				SeedName: pointer.String("source-seed"),
			},
		}

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
	})

	Describe("#SeedNamePredicate", func() {
		var (
			p predicate.Predicate
		)

		BeforeEach(func() {
			p = reconciler.IsBeingMigratedPredicate()
			Expect(inject.StopChannelInto(ctx.Done(), p)).To(BeTrue())
		})

		It("should return false if the specified object is not a BackupEntry", func() {
			verifyPredicate(p, &corev1.Secret{}, BeFalse())
		})

		It("should return false if status.SeedName is nil", func() {
			backupEntry.Status.SeedName = nil

			verifyPredicate(p, backupEntry, BeFalse())
		})

		It("should return false if spec.Status.SeedName status.SeedName are equal", func() {
			backupEntry.Status.SeedName = pointer.String("seed")

			verifyPredicate(p, backupEntry, BeFalse())
		})

		It("should return false if the backupEntry does not belong to this seed", func() {
			backupEntry.Spec.SeedName = pointer.String("another-seed")

			verifyPredicate(p, backupEntry, BeFalse())
		})

		It("should return false if the get call on source seed fails", func() {
			verifyPredicate(p, backupEntry, BeFalse())
		})

		It("should return true if the source seed has ownerchecks enabled", func() {
			Expect(fakeClient.Create(ctx, sourceSeed)).To(Succeed())

			verifyPredicate(p, backupEntry, BeTrue())
		})

		It("should return true if the source seed has ownerchecks disabled", func() {
			sourceSeed.Spec.Settings.OwnerChecks.Enabled = false
			Expect(fakeClient.Create(ctx, sourceSeed)).To(Succeed())

			verifyPredicate(p, backupEntry, BeFalse())
		})
	})
})

func verifyPredicate(p predicate.Predicate, obj client.Object, match gomegatypes.GomegaMatcher) {
	ExpectWithOffset(1, p.Create(event.CreateEvent{Object: obj})).To(match)
	ExpectWithOffset(1, p.Update(event.UpdateEvent{ObjectNew: obj})).To(match)
	ExpectWithOffset(1, p.Delete(event.DeleteEvent{Object: obj})).To(match)
	ExpectWithOffset(1, p.Generic(event.GenericEvent{Object: obj})).To(match)
}
