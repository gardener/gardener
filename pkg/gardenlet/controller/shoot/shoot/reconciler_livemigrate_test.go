// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("#checkGardenletVersionParity", func() {
	var (
		ctx                = context.TODO()
		sourceSeed, destSeed *gardencorev1beta1.Seed
		shoot              *gardencorev1beta1.Shoot
	)

	BeforeEach(func() {
		sourceSeed = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "source-seed"}}
		destSeed = &gardencorev1beta1.Seed{ObjectMeta: metav1.ObjectMeta{Name: "dest-seed"}}

		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "shoot",
				Namespace:   "garden",
				Annotations: map[string]string{v1beta1constants.AnnotationMigrationLiveMigrate: "true"},
			},
			Spec:   gardencorev1beta1.ShootSpec{SeedName: new("dest-seed")},
			Status: gardencorev1beta1.ShootStatus{SeedName: new("source-seed")},
		}
	})

	newReconciler := func(ownSeedName, localVersion string, seeds ...*gardencorev1beta1.Seed) *Reconciler {
		builder := fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme)
		for _, s := range seeds {
			builder = builder.WithObjects(s)
		}
		r := &Reconciler{
			GardenAPIReader: builder.Build(),
			Identity:        &gardencorev1beta1.Gardener{Version: localVersion},
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				SeedConfig: &gardenletconfigv1alpha1.SeedConfig{},
			},
		}
		r.Config.SeedConfig.Name = ownSeedName
		return r
	}

	It("returns nil when the shoot is no longer undergoing a live migration", func() {
		shoot.SetAnnotations(nil)
		r := newReconciler("source-seed", "1.2.3", destSeed)
		Expect(r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)).To(Succeed())
	})

	It("returns a non-parity error when seed config is not set", func() {
		r := newReconciler("source-seed", "1.2.3", destSeed)
		r.Config.SeedConfig = nil
		err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
		Expect(err).To(MatchError(ContainSubstring("seed config is not set")))
	})

	Describe("source role", func() {
		It("returns nil when both versions match", func() {
			destSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "1.2.3"}
			r := newReconciler("source-seed", "1.2.3", destSeed)
			Expect(r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)).To(Succeed())
		})

		It("returns error when versions differ", func() {
			destSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "1.2.4"}
			r := newReconciler("source-seed", "1.2.3", destSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`source: "1.2.3", destination: "1.2.4"`)))
		})

		It("returns error when local version is empty", func() {
			r := newReconciler("source-seed", "", destSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed"`)))
		})

		It("returns error when peer version is empty", func() {
			r := newReconciler("source-seed", "1.2.3", destSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed"`)))
		})

		It("returns error when peer seed is not found", func() {
			r := newReconciler("source-seed", "1.2.3")
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed" not found`)))
		})

		It("returns error when versions differ only by prerelease suffix (v1.155.0 vs v1.155.0-rc1)", func() {
			destSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "v1.155.0-rc1"}
			r := newReconciler("source-seed", "v1.155.0", destSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`source: "v1.155.0", destination: "v1.155.0-rc1"`)))
		})
	})

	Describe("destination role", func() {
		It("returns error when local version is empty", func() {
			r := newReconciler("dest-seed", "", sourceSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed"`)))
		})

		It("returns nil when both versions match", func() {
			sourceSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "1.2.3"}
			r := newReconciler("dest-seed", "1.2.3", sourceSeed)
			Expect(r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleDestination)).To(Succeed())
		})

		It("returns error with source/destination labelled correctly", func() {
			sourceSeed.Status.Gardener = &gardencorev1beta1.Gardener{Version: "1.2.3"}
			r := newReconciler("dest-seed", "1.2.4", sourceSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`source: "1.2.3", destination: "1.2.4"`)))
		})

		It("returns error when peer (source) version is empty", func() {
			r := newReconciler("dest-seed", "1.2.3", sourceSeed)
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed"`)))
		})

		It("returns error when peer seed is not found", func() {
			r := newReconciler("dest-seed", "1.2.3")
			err := r.checkGardenletVersionParity(ctx, shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed" not found`)))
		})
	})
})
