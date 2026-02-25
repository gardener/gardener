// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package controllerinstallation_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/controllerinstallation"
)

var _ = Describe("Add", func() {
	Describe("BackupBucketPredicate", func() {
		var (
			p            predicate.Predicate
			backupBucket *gardencorev1beta1.BackupBucket
		)

		BeforeEach(func() {
			p = BackupBucketPredicate(false)
			backupBucket = &gardencorev1beta1.BackupBucket{}
		})

		Describe("#Create", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				backupBucket.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Create(event.CreateEvent{Object: backupBucket})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no BackupBucket", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no BackupBucket", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket})).To(BeFalse())
			})

			It("should return false because neither seed name nor provider type changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: backupBucket})).To(BeFalse())
			})

			It("should return true because seed name changed", func() {
				oldBackupBucket := backupBucket.DeepCopy()
				backupBucket.Spec.SeedName = ptr.To("new-seed")
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: oldBackupBucket})).To(BeTrue())
			})

			It("should return true because provider type changed", func() {
				oldBackupBucket := backupBucket.DeepCopy()
				backupBucket.Spec.Provider.Type = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupBucket, ObjectOld: oldBackupBucket})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Delete(event.DeleteEvent{Object: backupBucket})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				backupBucket.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Delete(event.DeleteEvent{Object: backupBucket})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("BackupEntryPredicate", func() {
		var (
			p           predicate.Predicate
			backupEntry *gardencorev1beta1.BackupEntry
		)

		BeforeEach(func() {
			p = BackupEntryPredicate(false)
			backupEntry = &gardencorev1beta1.BackupEntry{}
		})

		Describe("#Create", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Create(event.CreateEvent{Object: backupEntry})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				backupEntry.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Create(event.CreateEvent{Object: backupEntry})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no BackupEntry", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no BackupEntry", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupEntry})).To(BeFalse())
			})

			It("should return false because neither seed name nor bucket name changed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupEntry, ObjectOld: backupEntry})).To(BeFalse())
			})

			It("should return true because seed name changed", func() {
				oldBackupEntry := backupEntry.DeepCopy()
				backupEntry.Spec.SeedName = ptr.To("new-seed")
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupEntry, ObjectOld: oldBackupEntry})).To(BeTrue())
			})

			It("should return true because bucket name changed", func() {
				oldBackupEntry := backupEntry.DeepCopy()
				backupEntry.Spec.BucketName = "bar"
				Expect(p.Update(event.UpdateEvent{ObjectNew: backupEntry, ObjectOld: oldBackupEntry})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Delete(event.DeleteEvent{Object: backupEntry})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				backupEntry.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Delete(event.DeleteEvent{Object: backupEntry})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("ShootPredicate", func() {
		var (
			p     predicate.Predicate
			shoot *gardencorev1beta1.Shoot
		)

		BeforeEach(func() {
			p = ShootPredicate(false)
			shoot = &gardencorev1beta1.Shoot{
				Spec: gardencorev1beta1.ShootSpec{
					Networking: &gardencorev1beta1.Networking{},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				shoot.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Create(event.CreateEvent{Object: shoot})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no Shoot", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no Shoot", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot})).To(BeFalse())
			})

			It("should return false because there is no relevant change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
			})

			It("should return true because seed name changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SeedName = ptr.To("new-seed")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because workers changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Workers = []gardencorev1beta1.Worker{{}}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because extensions changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Extensions = []gardencorev1beta1.Extension{{}}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because DNS changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.DNS = &gardencorev1beta1.DNS{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return false because both old and new networking fields are nil", func() {
				shoot.Spec.Networking = nil
				oldShoot := shoot.DeepCopy()
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return false because old networking field was nil and new doesn't have type", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Networking = nil
				shoot.Spec.Networking.Type = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return true because old networking field was nil and new has type", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Networking = nil
				shoot.Spec.Networking.Type = ptr.To("type")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because old networking field was non-nil and had type set and new is nil", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Networking.Type = ptr.To("type")
				shoot.Spec.Networking = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return false because old networking field was non-nil but had no type set and new is nil", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Networking = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeFalse())
			})

			It("should return true because networking type is changed", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Networking.Type = ptr.To("foo")
				shoot.Spec.Networking.Type = ptr.To("bar")
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})

			It("should return true because provider type changed", func() {
				oldShoot := shoot.DeepCopy()
				shoot.Spec.Provider.Type = "foo"
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false when seed name is not set", func() {
				Expect(p.Delete(event.DeleteEvent{Object: shoot})).To(BeFalse())
			})

			It("should return true when seed name is set", func() {
				shoot.Spec.SeedName = ptr.To("some-seed")
				Expect(p.Delete(event.DeleteEvent{Object: shoot})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("ControllerInstallationPredicate", func() {
		var (
			p                      predicate.Predicate
			controllerInstallation *gardencorev1beta1.ControllerInstallation
		)

		BeforeEach(func() {
			p = ControllerInstallationPredicate(false)
			controllerInstallation = &gardencorev1beta1.ControllerInstallation{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because new object is no ControllerInstallation", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is no ControllerInstallation", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation})).To(BeFalse())
			})

			It("should return false because there is no relevant change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: controllerInstallation})).To(BeFalse())
			})

			It("should return true because Required condition added", func() {
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.ControllerInstallationRequired, Status: gardencorev1beta1.ConditionTrue},
				}
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return true because Required condition changed", func() {
				controllerInstallation.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.ControllerInstallationRequired, Status: gardencorev1beta1.ConditionTrue},
				}
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.Status.Conditions[0].Status = gardencorev1beta1.ConditionFalse
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})

			It("should return true because Required condition removed", func() {
				controllerInstallation.Status.Conditions = []gardencorev1beta1.Condition{
					{Type: gardencorev1beta1.ControllerInstallationRequired, Status: gardencorev1beta1.ConditionTrue},
				}
				oldControllerInstallation := controllerInstallation.DeepCopy()
				controllerInstallation.Status.Conditions = nil
				Expect(p.Update(event.UpdateEvent{ObjectNew: controllerInstallation, ObjectOld: oldControllerInstallation})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
			})
		})
	})
})
