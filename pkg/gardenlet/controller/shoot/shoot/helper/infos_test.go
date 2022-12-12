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

package helper_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/component-base/version"
	"k8s.io/utils/clock"
	testclock "k8s.io/utils/clock/testing"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	gardenletconfig "github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/shoot/shoot/helper"
)

// Note: similar to the tested code itself, these tests are super verbose.
// DO NOT TRY TO REFACTOR OR SIMPLIFY THIS!
// If new cases come up or some cases haven't been covered yet, add individual test specs for all operation types.

var _ = Describe("CalculateControllerInfos", func() {
	var (
		cl    clock.Clock
		shoot *gardencorev1beta1.Shoot
		cfg   gardenletconfig.ShootControllerConfiguration

		infos ControllerInfos
	)

	BeforeEach(func() {
		cl = testclock.NewFakeClock(time.Now())

		shoot = &gardencorev1beta1.Shoot{
			Spec: gardencorev1beta1.ShootSpec{
				SeedName: pointer.String("seed"),
			},
			Status: gardencorev1beta1.ShootStatus{
				SeedName:      pointer.String("seed"),
				LastOperation: &gardencorev1beta1.LastOperation{},
				Gardener: gardencorev1beta1.Gardener{
					// don't bother injecting an arbitrary version,
					// what matters is that this field contains the same version as the binary
					Version: version.Get().GitVersion,
				},
			},
		}

		// default shoot controller settings
		cfg = gardenletconfig.ShootControllerConfiguration{
			SyncPeriod:                 &metav1.Duration{Duration: time.Hour},
			RespectSyncPeriodOverwrite: pointer.Bool(false),
			ReconcileInMaintenanceOnly: pointer.Bool(false),
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
				cfg.RespectSyncPeriodOverwrite = pointer.Bool(true)
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

	Context("shoot migration", func() {
		BeforeEach(func() {
			shoot.Generation = 2
			shoot.Status.ObservedGeneration = 1
			shoot.Spec.SeedName = pointer.String("other")
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
				cfg.RespectSyncPeriodOverwrite = pointer.Bool(true)
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
			shoot.Spec.SeedName = pointer.String("other")
			// after successful Migrate operation, the source gardenlet updates status.seedName to be spec.seedName
			shoot.Status.SeedName = pointer.String("other")
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
				cfg.RespectSyncPeriodOverwrite = pointer.Bool(true)
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
				cfg.RespectSyncPeriodOverwrite = pointer.Bool(true)
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
