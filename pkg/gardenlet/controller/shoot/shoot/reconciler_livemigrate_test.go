// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package shoot

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	testclock "k8s.io/utils/clock/testing"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta1helper "github.com/gardener/gardener/pkg/api/core/v1beta1/helper"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/apis/config/gardenlet/v1alpha1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
)

var _ = Describe("#checkLiveMigrationVersionParity", func() {
	var (
		ctx        = context.TODO()
		sourceSeed *gardencorev1beta1.Seed
		destSeed   *gardencorev1beta1.Seed
		shoot      *gardencorev1beta1.Shoot
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

	setPeerVersion := func(seed *gardencorev1beta1.Seed, version string) {
		seed.Status.Gardener = &gardencorev1beta1.Gardener{Version: version}
	}

	It("returns nil when the shoot is no longer undergoing a live migration", func() {
		shoot.SetAnnotations(nil)
		r := newReconciler("source-seed", "1.2.3", destSeed)
		Expect(r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)).To(Succeed())
	})

	It("returns a non-parity error when seed config is not set", func() {
		r := newReconciler("source-seed", "1.2.3", destSeed)
		r.Config.SeedConfig = nil
		err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
		Expect(err).To(MatchError(ContainSubstring("seed config is not set")))
		Expect(v1beta1helper.ExtractErrorCodes(err)).To(BeEmpty())
	})

	Describe("source role", func() {
		It("returns nil when both versions match", func() {
			setPeerVersion(destSeed, "1.2.3")
			r := newReconciler("source-seed", "1.2.3", destSeed)
			Expect(r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)).To(Succeed())
		})

		It("returns version-mismatch error when versions differ", func() {
			setPeerVersion(destSeed, "1.2.4")
			r := newReconciler("source-seed", "1.2.3", destSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring("source: 1.2.3, destination: 1.2.4")))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns version-mismatch error when local version is empty", func() {
			r := newReconciler("source-seed", "", destSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed"`)))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns version-mismatch error when peer version is empty", func() {
			r := newReconciler("source-seed", "1.2.3", destSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed"`)))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns error when peer seed is not found", func() {
			r := newReconciler("source-seed", "1.2.3")
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed" not found`)))
		})

		It("returns version-mismatch error when versions differ only by prerelease suffix (v1.155.0 vs v1.155.0-rc1)", func() {
			setPeerVersion(destSeed, "v1.155.0-rc1")
			r := newReconciler("source-seed", "v1.155.0", destSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleSource)
			Expect(err).To(MatchError(ContainSubstring("source: v1.155.0, destination: v1.155.0-rc1")))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})
	})

	Describe("destination role", func() {
		It("returns version-mismatch error when local version is empty", func() {
			r := newReconciler("dest-seed", "", sourceSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"dest-seed"`)))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns nil when both versions match", func() {
			setPeerVersion(sourceSeed, "1.2.3")
			r := newReconciler("dest-seed", "1.2.3", sourceSeed)
			Expect(r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleDestination)).To(Succeed())
		})

		It("returns version-mismatch error with source/destination labelled correctly", func() {
			setPeerVersion(sourceSeed, "1.2.3")
			r := newReconciler("dest-seed", "1.2.4", sourceSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring("source: 1.2.3, destination: 1.2.4")))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns version-mismatch error when peer (source) version is empty", func() {
			r := newReconciler("dest-seed", "1.2.3", sourceSeed)
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed"`)))
			Expect(v1beta1helper.ExtractErrorCodes(err)).To(ContainElement(gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch))
		})

		It("returns error when peer seed is not found", func() {
			r := newReconciler("dest-seed", "1.2.3")
			err := r.checkLiveMigrationVersionParity(ctx, logr.Discard(), shoot, v1beta1helper.LiveMigrationRoleDestination)
			Expect(err).To(MatchError(ContainSubstring(`"source-seed" not found`)))
		})
	})
})

var _ = Describe("#blockLiveMigrationOnParityError", func() {
	var (
		ctx           = context.TODO()
		fakeClock     = testclock.NewFakeClock(time.Now())
		gardenClient  client.Client
		shoot         *gardencorev1beta1.Shoot
		r             *Reconciler
		parityLastErr = gardencorev1beta1.LastError{
			Description: "live migration blocked: versions differ",
			Codes:       []gardencorev1beta1.ErrorCode{gardencorev1beta1.ErrorLiveMigrationSeedVersionMismatch},
		}
	)

	BeforeEach(func() {
		shoot = &gardencorev1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "shoot",
				Namespace:  "garden",
				Finalizers: []string{gardencorev1beta1.GardenerName},
			},
			Spec:   gardencorev1beta1.ShootSpec{SeedName: new("dest-seed")},
			Status: gardencorev1beta1.ShootStatus{SeedName: new("source-seed")},
		}
		gardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).WithStatusSubresource(&gardencorev1beta1.Shoot{}).WithObjects(shoot).Build()
		r = &Reconciler{
			GardenClient: gardenClient,
			Identity:     &gardencorev1beta1.Gardener{Version: "1.2.3"},
			Clock:        fakeClock,
			Config: gardenletconfigv1alpha1.GardenletConfiguration{
				Controllers: &gardenletconfigv1alpha1.GardenletControllerConfiguration{
					Shoot: &gardenletconfigv1alpha1.ShootControllerConfiguration{
						RetryDuration: &metav1.Duration{Duration: 10 * time.Minute},
					},
				},
			},
		}
	})

	readShoot := func() *gardencorev1beta1.Shoot {
		s := &gardencorev1beta1.Shoot{}
		ExpectWithOffset(1, gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), s)).To(Succeed())
		return s
	}

	It("patches a retryable LastOperation error and requeues after the parity interval", func() {
		result, err := r.blockLiveMigrationOnParityError(ctx, readShoot(), parityLastErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(liveMigrationVersionParityRequeueInterval))

		updated := readShoot()
		Expect(updated.Status.LastOperation).NotTo(BeNil())
		Expect(updated.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
		Expect(updated.Status.LastOperation.Type).To(Equal(gardencorev1beta1.LastOperationTypeLiveMigrate))
		Expect(updated.Status.LastOperation.Description).To(Equal(parityLastErr.Description + " Operation will be retried."))
		Expect(updated.Status.LastErrors).To(Equal([]gardencorev1beta1.LastError{parityLastErr}))
	})

	It("keeps LastOperation in state Error across repeated blocked reconciliations", func() {
		for range 3 {
			result, err := r.blockLiveMigrationOnParityError(ctx, readShoot(), parityLastErr)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.RequeueAfter).To(Equal(liveMigrationVersionParityRequeueInterval))
			Expect(readShoot().Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
		}
	})

	It("still retries when the retry cycle has already elapsed (forceRetry)", func() {
		shoot := readShoot()
		shoot.Status.RetryCycleStartTime = &metav1.Time{Time: fakeClock.Now().Add(-time.Hour)}
		r.Config.Controllers.Shoot.RetryDuration = &metav1.Duration{Duration: time.Minute}

		result, err := r.blockLiveMigrationOnParityError(ctx, shoot, parityLastErr)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.RequeueAfter).To(Equal(liveMigrationVersionParityRequeueInterval))

		updated := readShoot()
		Expect(updated.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateError))
		Expect(updated.Status.LastOperation.Description).To(HaveSuffix(" Operation will be retried."))
	})

	It("suppresses the patch error with the parity description and does not requeue", func() {
		rMissing := &Reconciler{
			GardenClient: fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build(),
			Identity:     r.Identity,
			Clock:        r.Clock,
			Config:       r.Config,
		}

		result, err := rMissing.blockLiveMigrationOnParityError(ctx, shoot, parityLastErr)
		Expect(err).To(MatchError(ContainSubstring(parityLastErr.Description)))
		Expect(result.RequeueAfter).To(BeZero())
	})

	It("unblocks the retry cycle on success: LastErrors are cleared and LastOperation is Succeeded", func() {
		Expect(gardenClient.Get(ctx, client.ObjectKeyFromObject(shoot), shoot)).To(Succeed())
		shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{
			Type:        gardencorev1beta1.LastOperationTypeLiveMigrate,
			State:       gardencorev1beta1.LastOperationStateError,
			Description: parityLastErr.Description + " Operation will be retried.",
		}
		shoot.Status.LastErrors = []gardencorev1beta1.LastError{parityLastErr}
		Expect(gardenClient.Status().Update(ctx, shoot)).To(Succeed())

		Expect(r.patchShootStatusOperationSuccess(ctx, readShoot(), gardencorev1beta1.LastOperationTypeLiveMigrate)).To(Succeed())

		updated := readShoot()
		Expect(updated.Status.LastOperation.State).To(Equal(gardencorev1beta1.LastOperationStateSucceeded))
		Expect(updated.Status.LastErrors).To(BeEmpty())
	})
})
