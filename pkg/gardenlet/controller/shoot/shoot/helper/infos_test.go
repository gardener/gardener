// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
	gardenerutils "github.com/gardener/gardener/pkg/utils/gardener"
	"github.com/gardener/gardener/pkg/utils/timewindow"
)

// Note: similar to the tested code itself, these tests are super verbose.
// DO NOT TRY TO REFACTOR OR SIMPLIFY THIS!
// If new cases come up or some cases haven't been covered yet, add individual test specs for all operation types.

var _ = Describe("CalculateControllerInfos", func() {
	var (
		cl    *testclock.FakeClock
		shoot *gardencorev1beta1.Shoot
		cfg   gardenletconfigv1alpha1.ShootControllerConfiguration

		timeWindow      timewindow.MaintenanceTimeWindow
		timeWindowBegin time.Time

		infos ControllerInfos
	)

	BeforeEach(func() {
		cl = testclock.NewFakeClock(time.Date(2022, 12, 12, 10, 0, 0, 0, time.UTC))

		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: ptr.To("seed"),
				Maintenance: &gardencorev1beta1.Maintenance{
					TimeWindow: &gardencorev1beta1.MaintenanceTimeWindow{
						Begin: "220000+0000",
						End:   "230000+0000",
					},
				},
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName:      ptr.To("seed"),
				LastOperation: &gardencorev1beta1.LastOperation{},
				Gardener: gardencorev1beta1.Gardener{
					// don't bother injecting an arbitrary version,
					// what matters is that this field contains the same version as the binary
					Version: version.Get().GitVersion,
				},
			},
		}

		timeWindow = *gardenerutils.EffectiveShootMaintenanceTimeWindow(shoot)
		m := timeWindow.Begin()
		now := cl.Now().UTC()
		timeWindowBegin = time.Date(now.Year(), now.Month(), now.Day(), m.Hour(), m.Minute(), m.Second(), 0, now.Location())

		// default shoot controller settings
		cfg = gardenletconfigv1alpha1.ShootControllerConfiguration{
			SyncPeriod:                 &metav1.Duration{Duration: time.Hour},
			RespectSyncPeriodOverwrite: ptr.To(false),
			ReconcileInMaintenanceOnly: ptr.To(false),
		}
	})

	JustBeforeEach(func() {
		infos = CalculateControllerInfos(shoot, cl, cfg)
	})

	Context("shoot creation", func() {
		BeforeEach(func() {
			shoot.Generation = 1
			shoot.Status.SeedName = nil
			shoot.Status.LastOperation = nil
		})

		JustBeforeEach(func() {
			Expect(infos.OperationType).To(Equal(gardencorev1beta1.LastOperationTypeCreate))
		})

		Context("creation is triggered", func() {
			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("creation was not successful yet", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("shoot is ignored", func() {
			BeforeEach(func() {
				cfg.RespectSyncPeriodOverwrite = ptr.To(true)
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootIgnore, "true")
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})
		})

		Context("shoot is failed", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation = &gardencorev1beta1.LastOperation{}
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeCreate
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})

			Context("shoot generation was increased", func() {
				BeforeEach(func() {
					shoot.Generation++
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})

			Context("gardenlet version was changed", func() {
				BeforeEach(func() {
					shoot.Status.Gardener.Version += "+1"
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})
		})
	})

	Context("shoot reconciliation", func() {
		BeforeEach(func() {
			shoot.Generation = 1
			shoot.Status.ObservedGeneration = 1
			shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		})

		JustBeforeEach(func() {
			Expect(infos.OperationType).To(Equal(gardencorev1beta1.LastOperationTypeReconcile))
		})

		Context("reconciliation is triggered by generation bump", func() {
			BeforeEach(func() {
				shoot.Generation++
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("reconciliation was not successful yet", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("reconciliations are not confined", func() {
			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should requeue with the general sync period", func() {
				requeueAfter := infos.RequeueAfter
				Expect(requeueAfter.Requeue).To(BeFalse())
				Expect(requeueAfter.RequeueAfter).To(Equal(cfg.SyncPeriod.Duration))
			})

			Context("shoot overwrites sync period", func() {
				var shootSyncPeriod time.Duration

				BeforeEach(func() {
					shootSyncPeriod = 2 * time.Hour

					cfg.RespectSyncPeriodOverwrite = ptr.To(true)
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootSyncPeriod, shootSyncPeriod.String())
				})

				It("should requeue with the shoot's sync period", func() {
					requeueAfter := infos.RequeueAfter
					Expect(requeueAfter.Requeue).To(BeFalse())
					Expect(requeueAfter.RequeueAfter).To(Equal(shootSyncPeriod))
				})
			})
		})

		Context("reconciliations are confined", func() {
			testReconciliationsConfined := func() {
				Context("currently not in maintenance time window", func() {
					It("should not reconcile the shoot immediately", func() {
						Expect(infos.ShouldReconcileNow).To(BeFalse())
						Expect(infos.ShouldOnlySyncClusterResource).To(BeFalse())
					})

					It("should enqueue the shoot during its next maintenance time window", func() {
						enqueueAfter := infos.EnqueueAfter
						Expect(enqueueAfter).To(BeNumerically(">", 0))
						Expect(enqueueAfter).To(BeNumerically("<", 23*time.Hour))

						nextReconciliation := cl.Now().Add(enqueueAfter)
						Expect(timeWindow.Contains(nextReconciliation)).To(BeTrue())
					})

					It("should requeue the shoot during its next maintenance time window", func() {
						requeueAfter := infos.RequeueAfter
						Expect(requeueAfter.Requeue).To(BeFalse())
						Expect(requeueAfter.RequeueAfter).To(BeNumerically(">", 0))
						Expect(requeueAfter.RequeueAfter).To(BeNumerically("<", 23*time.Hour))

						nextReconciliation := cl.Now().Add(requeueAfter.RequeueAfter)
						Expect(timeWindow.Contains(nextReconciliation)).To(BeTrue())
					})
				})

				Context("currently in maintenance time window", func() {
					BeforeEach(func() {
						cl.SetTime(timeWindowBegin.Add(5 * time.Minute))
					})

					Context("not reconciled in this maintenance time window yet", func() {
						It("should reconcile the shoot immediately", func() {
							// If a reconciliation request is passed to the reconciler (e.g., exponential requeue or requeue after),
							// handle it immediately instead of calculating a new random time in this time window.
							Expect(infos.ShouldReconcileNow).To(BeTrue())
							Expect(infos.ShouldOnlySyncClusterResource).To(BeFalse())
						})

						It("should enqueue the shoot during this maintenance time window", func() {
							// If we get an ADD event for this shoot, we still calculate a random time in this time window.
							// This is done to not reconcile all shoots that are currently in their maintenance time window
							// when the gardenlet starts up.
							enqueueAfter := infos.EnqueueAfter
							Expect(enqueueAfter).To(BeNumerically(">", 0))
							Expect(enqueueAfter).To(BeNumerically("<", time.Hour))

							nextReconciliation := cl.Now().Add(enqueueAfter)
							Expect(timeWindow.Contains(nextReconciliation)).To(BeTrue())
						})
					})

					Context("already reconciled in this maintenance time window", func() {
						BeforeEach(func() {
							shoot.Status.LastOperation.LastUpdateTime = metav1.NewTime(timeWindowBegin.Add(time.Minute))
						})

						It("should not reconcile the shoot immediately", func() {
							Expect(infos.ShouldReconcileNow).To(BeFalse())
							Expect(infos.ShouldOnlySyncClusterResource).To(BeFalse())
						})

						It("should enqueue the shoot during its next maintenance time window", func() {
							enqueueAfter := infos.EnqueueAfter
							Expect(enqueueAfter).To(BeNumerically(">", 23*time.Hour))
							Expect(enqueueAfter).To(BeNumerically("<", 47*time.Hour))

							nextReconciliation := cl.Now().Add(enqueueAfter)
							Expect(timeWindow.Contains(nextReconciliation)).To(BeTrue())
						})

						It("should requeue the shoot during its next maintenance time window", func() {
							requeueAfter := infos.RequeueAfter
							Expect(requeueAfter.Requeue).To(BeFalse())
							Expect(requeueAfter.RequeueAfter).To(BeNumerically(">", 23*time.Hour))
							Expect(requeueAfter.RequeueAfter).To(BeNumerically("<", 47*time.Hour))

							nextReconciliation := cl.Now().Add(requeueAfter.RequeueAfter)
							Expect(timeWindow.Contains(nextReconciliation)).To(BeTrue())
						})
					})
				})
			}

			Context("confined by operator (reconcileInMaintenanceOnly)", func() {
				BeforeEach(func() {
					cfg.ReconcileInMaintenanceOnly = ptr.To(true)
				})

				testReconciliationsConfined()
			})

			Context("confined by user (confineSpecUpdateRollout)", func() {
				BeforeEach(func() {
					shoot.Spec.Maintenance.ConfineSpecUpdateRollout = ptr.To(true)
				})

				testReconciliationsConfined()
			})
		})

		Context("shoot is ignored", func() {
			BeforeEach(func() {
				cfg.RespectSyncPeriodOverwrite = ptr.To(true)
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootIgnore, "true")
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})
		})

		Context("shoot is failed", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})

			Context("shoot generation was increased", func() {
				BeforeEach(func() {
					shoot.Generation++
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})

			Context("gardenlet version was changed", func() {
				BeforeEach(func() {
					shoot.Status.Gardener.Version += "+1"
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})
		})
	})

	Context("shoot migration", func() {
		BeforeEach(func() {
			shoot.Generation = 2
			shoot.Status.ObservedGeneration = 1
			shoot.Spec.SeedName = ptr.To("other")
			shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		})

		JustBeforeEach(func() {
			Expect(infos.OperationType).To(Equal(gardencorev1beta1.LastOperationTypeMigrate))
		})

		Context("migration is triggered", func() {
			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("migration was not successful yet", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("shoot is ignored", func() {
			BeforeEach(func() {
				cfg.RespectSyncPeriodOverwrite = ptr.To(true)
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootIgnore, "true")
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})
		})

		Context("shoot is failed", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})

			Context("shoot generation was increased", func() {
				BeforeEach(func() {
					shoot.Generation++
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})

			Context("gardenlet version was changed", func() {
				BeforeEach(func() {
					shoot.Status.Gardener.Version += "+1"
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})
		})
	})

	Context("shoot restoration", func() {
		BeforeEach(func() {
			shoot.Generation = 2
			shoot.Status.ObservedGeneration = 2
			shoot.Spec.SeedName = ptr.To("other")
			// after successful Migrate operation, the source gardenlet updates status.seedName to be spec.seedName
			shoot.Status.SeedName = ptr.To("other")
			shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeMigrate
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		})

		JustBeforeEach(func() {
			Expect(infos.OperationType).To(Equal(gardencorev1beta1.LastOperationTypeRestore))
		})

		Context("restoration is triggered", func() {
			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("restoration was not successful yet", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("shoot is ignored", func() {
			BeforeEach(func() {
				cfg.RespectSyncPeriodOverwrite = ptr.To(true)
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootIgnore, "true")
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})
		})

		Context("shoot is failed", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeRestore
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})

			Context("shoot generation was increased", func() {
				BeforeEach(func() {
					shoot.Generation++
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})

			Context("gardenlet version was changed", func() {
				BeforeEach(func() {
					shoot.Status.Gardener.Version += "+1"
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})
		})
	})

	Context("shoot deletion", func() {
		BeforeEach(func() {
			shoot.Generation = 2
			shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
			shoot.Status.ObservedGeneration = 1
			shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
		})

		JustBeforeEach(func() {
			Expect(infos.OperationType).To(Equal(gardencorev1beta1.LastOperationTypeDelete))
		})

		Context("deletion is triggered", func() {
			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("deletion was not successful yet", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateError
			})

			It("should reconcile the shoot immediately", func() {
				Expect(infos.ShouldReconcileNow).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})
		})

		Context("shoot is ignored", func() {
			BeforeEach(func() {
				cfg.RespectSyncPeriodOverwrite = ptr.To(true)
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.ShootIgnore, "true")
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})
		})

		Context("shoot is failed", func() {
			BeforeEach(func() {
				shoot.Status.ObservedGeneration = shoot.Generation
				shoot.Status.LastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
				shoot.Status.LastOperation.State = gardencorev1beta1.LastOperationStateFailed
			})

			It("should not reconcile the shoot but sync the cluster resource", func() {
				Expect(infos.ShouldReconcileNow).To(BeFalse())
				Expect(infos.ShouldOnlySyncClusterResource).To(BeTrue())
				Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
			})

			It("should not requeue the shoot after syncing the cluster resource", func() {
				Expect(infos.RequeueAfter).To(Equal(reconcile.Result{}))
			})

			Context("shoot generation was increased", func() {
				BeforeEach(func() {
					shoot.Generation++
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})

			Context("gardenlet version was changed", func() {
				BeforeEach(func() {
					shoot.Status.Gardener.Version += "+1"
				})

				It("should reconcile the shoot immediately", func() {
					Expect(infos.ShouldReconcileNow).To(BeTrue())
					Expect(infos.EnqueueAfter).To(Equal(time.Duration(0)))
				})
			})
		})
	})
})
