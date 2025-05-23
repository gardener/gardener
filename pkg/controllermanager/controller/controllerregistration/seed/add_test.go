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
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	gardencorev1 "github.com/gardener/gardener/pkg/apis/core/v1"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/controllerregistration/seed"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("SeedPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.SeedPredicate()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			var seed *gardencorev1beta1.Seed

			BeforeEach(func() {
				seed = &gardencorev1beta1.Seed{}
			})

			It("should return false because new object is not a seed", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false because old object is not a seed", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed})).To(BeFalse())
			})

			It("should return false because neither spec, annotations, labels changed nor deletion timestamp got set", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: seed})).To(BeFalse())
			})

			It("should return true because spec has changed", func() {
				oldSeed := seed.DeepCopy()
				seed.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{Type: "foo"}
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})

			It("should return true because annotations have changed", func() {
				oldSeed := seed.DeepCopy()
				metav1.SetMetaDataAnnotation(&seed.ObjectMeta, "foo", "bar")

				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})

			It("should return true because labels have changed", func() {
				oldSeed := seed.DeepCopy()
				metav1.SetMetaDataLabel(&oldSeed.ObjectMeta, "foo", "bar")
				metav1.SetMetaDataLabel(&seed.ObjectMeta, "foo", "baz")

				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})

			It("should return true because deletion timestamp got set", func() {
				oldSeed := seed.DeepCopy()
				seed.DeletionTimestamp = &metav1.Time{}
				Expect(p.Update(event.UpdateEvent{ObjectNew: seed, ObjectOld: oldSeed})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return true", func() {
				Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should return true", func() {
				Expect(p.Generic(event.GenericEvent{})).To(BeTrue())
			})
		})
	})

	Describe("BackupBucketPredicate", func() {
		var (
			p            predicate.Predicate
			backupBucket *gardencorev1beta1.BackupBucket
		)

		BeforeEach(func() {
			p = reconciler.BackupBucketPredicate()
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
			p = reconciler.BackupEntryPredicate()
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
			p = reconciler.ShootPredicate()
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
			p = reconciler.ControllerInstallationPredicate()
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
				Expect(reconciler.MapToAllSeeds(log)(ctx, nil)).To(ConsistOf(
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
				Expect(reconciler.MapBackupBucketToSeed(ctx, backupBucket)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(reconciler.MapBackupBucketToSeed(ctx, backupBucket)).To(ConsistOf(
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
				Expect(reconciler.MapBackupEntryToSeed(ctx, backupEntry)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(reconciler.MapBackupEntryToSeed(ctx, backupEntry)).To(ConsistOf(
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
				Expect(reconciler.MapShootToSeed(ctx, shoot)).To(BeEmpty())
			})

			It("should map to the seed", func() {
				Expect(reconciler.MapShootToSeed(ctx, shoot)).To(ConsistOf(
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
				controllerInstallation = &gardencorev1beta1.ControllerInstallation{Spec: gardencorev1beta1.ControllerInstallationSpec{SeedRef: corev1.ObjectReference{Name: seedName}}}
			})

			It("should map to the seed", func() {
				Expect(reconciler.MapControllerInstallationToSeed(ctx, controllerInstallation)).To(ConsistOf(
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
				Expect(reconciler.MapControllerDeploymentToAllSeeds(log)(ctx, controllerDeployment)).To(BeEmpty())
			})

			It("should map to all seeds the seed because there is a ControllerRegistration referencing the deployment", func() {
				Expect(fakeClient.Create(ctx, controllerRegistration)).To(Succeed())

				Expect(reconciler.MapControllerDeploymentToAllSeeds(log)(ctx, controllerDeployment)).To(ConsistOf(
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed1.Name}},
					reconcile.Request{NamespacedName: types.NamespacedName{Name: seed2.Name}},
				))
			})
		})
	})
})
