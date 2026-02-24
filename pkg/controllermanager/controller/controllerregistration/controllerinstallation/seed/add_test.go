// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package seed_test

import (
	"context"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/controller/controllerinstallation"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation/seed"
)

var _ = Describe("Add", func() {
	var reconciler *controllerinstallation.Reconciler

	BeforeEach(func() {
		reconciler = &controllerinstallation.Reconciler{}
	})

	Context("Mappers", func() {
		var (
			ctx        = context.TODO()
			log        logr.Logger
			fakeClient client.Client
		)

		BeforeEach(func() {
			log = logr.Discard()
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			reconciler.Client = fakeClient
		})

		Describe("#MapToAllSeeds", func() {
			var seed1, seed2 *gardencorev1beta1.Seed

			BeforeEach(func() {
				seed1 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}}
				seed2 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}}

				Expect(fakeClient.Create(ctx, seed1)).To(Succeed())
				Expect(fakeClient.Create(ctx, seed2)).To(Succeed())
			})

			It("should map to all seeds", func() {
				Expect(MapToAllSeeds(log, reconciler)(ctx, nil)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed1.Name}},
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed2.Name}},
				))
			})
		})

		Describe("#MapBackupBucketToSeed", func() {
			var (
				backupBucket *gardencorev1beta1.BackupBucket
				seedName     = "seed"
			)

			BeforeEach(func() {
				backupBucket = &gardencorev1beta1.BackupBucket{Spec: gardencorev1beta1.BackupBucketSpec{SeedName: &seedName}}
			})

			It("should return nil when seed name is not set", func() {
				backupBucket.Spec.SeedName = nil
				Expect(MapBackupBucketToSeed(ctx, backupBucket)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(MapBackupBucketToSeed(ctx, backupBucket)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}},
				))
			})
		})

		Describe("#MapBackupEntryToSeed", func() {
			var (
				backupEntry *gardencorev1beta1.BackupEntry
				seedName    = "seed"
			)

			BeforeEach(func() {
				backupEntry = &gardencorev1beta1.BackupEntry{Spec: gardencorev1beta1.BackupEntrySpec{SeedName: &seedName}}
			})

			It("should return nil when seed name is not set", func() {
				backupEntry.Spec.SeedName = nil
				Expect(MapBackupEntryToSeed(ctx, backupEntry)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(MapBackupEntryToSeed(ctx, backupEntry)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}},
				))
			})
		})

		Describe("#MapShootToSeed", func() {
			var (
				shoot    *gardencorev1beta1.Shoot
				seedName = "seed"
			)

			BeforeEach(func() {
				shoot = &gardencorev1beta1.Shoot{Spec: gardencorev1beta1.ShootSpec{SeedName: &seedName}}
			})

			It("should return nil when seed name is not set", func() {
				shoot.Spec.SeedName = nil
				Expect(MapShootToSeed(ctx, shoot)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(MapShootToSeed(ctx, shoot)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}},
				))
			})
		})

		Describe("#MapControllerInstallationToSeed", func() {
			var (
				controllerInstallation *gardencorev1beta1.ControllerInstallation
				seedName               = "seed"
			)

			BeforeEach(func() {
				controllerInstallation = &gardencorev1beta1.ControllerInstallation{Spec: gardencorev1beta1.ControllerInstallationSpec{SeedRef: &corev1.ObjectReference{Name: seedName}}}
			})

			It("should map to the seed", func() {
				Expect(MapControllerInstallationToSeed(ctx, controllerInstallation)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seedName}},
				))
			})
		})

		Describe("#MapControllerDeploymentToAllSeeds", func() {
			var (
				deploymentName = "deployment"

				controllerDeployment   *gardencorev1.ControllerDeployment
				controllerRegistration *gardencorev1beta1.ControllerRegistration
				seed1, seed2           *gardencorev1beta1.Seed
			)

			BeforeEach(func() {
				controllerDeployment = &gardencorev1.ControllerDeployment{ObjectMeta: metav1.ObjectMeta{Name: deploymentName}}
				controllerRegistration = &gardencorev1beta1.ControllerRegistration{
					ObjectMeta: metav1.ObjectMeta{GenerateName: "registration-"},
					Spec: gardencorev1beta1.ControllerRegistrationSpec{
						Deployment: &gardencorev1beta1.ControllerRegistrationDeployment{
							DeploymentRefs: []gardencorev1beta1.DeploymentRef{{Name: deploymentName}},
						},
					},
				}

				seed1 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed1"}}
				seed2 = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "seed2"}}

				Expect(fakeClient.Create(ctx, seed1)).To(Succeed())
				Expect(fakeClient.Create(ctx, seed2)).To(Succeed())
			})

			It("should return nil because there is no ControllerRegistration referencing the deployment", func() {
				Expect(MapControllerDeploymentToAllSeeds(log, reconciler)(ctx, controllerDeployment)).To(BeEmpty())
			})

			It("should map to all seeds the seed because there is a ControllerRegistration referencing the deployment", func() {
				Expect(fakeClient.Create(ctx, controllerRegistration)).To(Succeed())

				Expect(MapControllerDeploymentToAllSeeds(log, reconciler)(ctx, controllerDeployment)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed1.Name}},
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed2.Name}},
				))
			})
		})
	})
})
