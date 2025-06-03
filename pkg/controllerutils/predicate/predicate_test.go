// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

var _ = Describe("Predicate", func() {
	var (
		extensionType = "extension-type"
	)

	Describe("#IsDeleting", func() {
		var (
			shoot        *gardencorev1beta1.Shoot
			predicate    predicate.Predicate
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{},
			}

			predicate = IsDeleting()

			createEvent = event.CreateEvent{
				Object: shoot,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: shoot,
				ObjectNew: shoot,
			}
			deleteEvent = event.DeleteEvent{
				Object: shoot,
			}
			genericEvent = event.GenericEvent{
				Object: shoot,
			}
		})

		Context("shoot doesn't have a deletion timestamp", func() {
			It("should be false", func() {
				Expect(predicate.Create(createEvent)).To(BeFalse())
				Expect(predicate.Update(updateEvent)).To(BeFalse())
				Expect(predicate.Delete(deleteEvent)).To(BeFalse())
				Expect(predicate.Generic(genericEvent)).To(BeFalse())
			})
		})

		Context("shoot has a deletion timestamp", func() {
			time := metav1.Now()

			BeforeEach(func() {
				shoot.DeletionTimestamp = &time
			})

			It("should be true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeTrue())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})
	})

	Describe("#HasName", func() {
		var (
			shoot        *gardencorev1beta1.Shoot
			predicate    predicate.Predicate
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{
				ObjectMeta: metav1.ObjectMeta{Name: "foobar"},
			}

			predicate = HasName(shoot.Name)

			createEvent = event.CreateEvent{
				Object: shoot,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: shoot,
				ObjectNew: shoot,
			}
			deleteEvent = event.DeleteEvent{
				Object: shoot,
			}
			genericEvent = event.GenericEvent{
				Object: shoot,
			}
		})

		Context("shoot has the requested name", func() {
			It("should be true", func() {
				Expect(predicate.Create(createEvent)).To(BeTrue())
				Expect(predicate.Update(updateEvent)).To(BeTrue())
				Expect(predicate.Delete(deleteEvent)).To(BeTrue())
				Expect(predicate.Generic(genericEvent)).To(BeTrue())
			})
		})

		Context("shoot does not have the requested name", func() {
			BeforeEach(func() {
				shoot.Name = "something-else"
			})

			It("should be false", func() {
				Expect(predicate.Create(createEvent)).To(BeFalse())
				Expect(predicate.Update(updateEvent)).To(BeFalse())
				Expect(predicate.Delete(deleteEvent)).To(BeFalse())
				Expect(predicate.Generic(genericEvent)).To(BeFalse())
			})
		})
	})

	DescribeTable("#ForEventTypes",
		func(events []EventType, createMatcher, updateMatcher, deleteMatcher, genericMatcher gomegatypes.GomegaMatcher) {
			p := ForEventTypes(events...)

			Expect(p.Create(event.CreateEvent{})).To(createMatcher)
			Expect(p.Update(event.UpdateEvent{})).To(updateMatcher)
			Expect(p.Delete(event.DeleteEvent{})).To(deleteMatcher)
			Expect(p.Generic(event.GenericEvent{})).To(genericMatcher)
		},

		Entry("none", nil, BeFalse(), BeFalse(), BeFalse(), BeFalse()),
		Entry("create", []EventType{Create}, BeTrue(), BeFalse(), BeFalse(), BeFalse()),
		Entry("update", []EventType{Update}, BeFalse(), BeTrue(), BeFalse(), BeFalse()),
		Entry("delete", []EventType{Delete}, BeFalse(), BeFalse(), BeTrue(), BeFalse()),
		Entry("generic", []EventType{Generic}, BeFalse(), BeFalse(), BeFalse(), BeTrue()),
		Entry("create, update", []EventType{Create, Update}, BeTrue(), BeTrue(), BeFalse(), BeFalse()),
		Entry("create, delete", []EventType{Create, Delete}, BeTrue(), BeFalse(), BeTrue(), BeFalse()),
		Entry("create, generic", []EventType{Create, Generic}, BeTrue(), BeFalse(), BeFalse(), BeTrue()),
		Entry("update, delete", []EventType{Update, Delete}, BeFalse(), BeTrue(), BeTrue(), BeFalse()),
		Entry("update, generic", []EventType{Update, Generic}, BeFalse(), BeTrue(), BeFalse(), BeTrue()),
		Entry("delete, generic", []EventType{Delete, Generic}, BeFalse(), BeFalse(), BeTrue(), BeTrue()),
		Entry("create, update, delete", []EventType{Create, Update, Delete}, BeTrue(), BeTrue(), BeTrue(), BeFalse()),
		Entry("create, update, generic", []EventType{Create, Update, Generic}, BeTrue(), BeTrue(), BeFalse(), BeTrue()),
		Entry("create, delete, generic", []EventType{Create, Delete, Generic}, BeTrue(), BeFalse(), BeTrue(), BeTrue()),
		Entry("update, delete, generic", []EventType{Update, Delete, Generic}, BeFalse(), BeTrue(), BeTrue(), BeTrue()),
		Entry("create, update, delete, generic", []EventType{Create, Update, Delete, Generic}, BeTrue(), BeTrue(), BeTrue(), BeTrue()),
	)

	Describe("#RelevantConditionsChanged", func() {
		var (
			p                 predicate.Predicate
			shoot             *gardencorev1beta1.Shoot
			conditionsToCheck = []gardencorev1beta1.ConditionType{"Foo", "Bar"}
		)

		BeforeEach(func() {
			shoot = &gardencorev1beta1.Shoot{}
			p = RelevantConditionsChanged(
				func(obj client.Object) []gardencorev1beta1.Condition {
					return obj.(*gardencorev1beta1.Shoot).Status.Conditions
				},
				conditionsToCheck...,
			)
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because there is no relevant change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: shoot})).To(BeFalse())
			})

			tests := func(conditionType gardencorev1beta1.ConditionType) {
				It("should return true because condition was added", func() {
					oldShoot := shoot.DeepCopy()
					shoot.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition was removed", func() {
					shoot.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := shoot.DeepCopy()
					shoot.Status.Conditions = nil
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition status was changed", func() {
					shoot.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := shoot.DeepCopy()
					shoot.Status.Conditions[0].Status = gardencorev1beta1.ConditionTrue
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition reason was changed", func() {
					shoot.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := shoot.DeepCopy()
					shoot.Status.Conditions[0].Reason = "reason"
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition message was changed", func() {
					shoot.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := shoot.DeepCopy()
					shoot.Status.Conditions[0].Message = "message"
					Expect(p.Update(event.UpdateEvent{ObjectNew: shoot, ObjectOld: oldShoot})).To(BeTrue())
				})
			}

			Context("first condition", func() {
				tests(conditionsToCheck[0])
			})

			Context("second condition", func() {
				tests(conditionsToCheck[1])
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

	Describe("#ManagedResourceConditionsChanged", func() {
		var (
			p               predicate.Predicate
			managedResource *resourcesv1alpha1.ManagedResource
		)

		BeforeEach(func() {
			managedResource = &resourcesv1alpha1.ManagedResource{}
			p = ManagedResourceConditionsChanged()
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because there is no relevant change", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: managedResource})).To(BeFalse())
			})

			tests := func(conditionType gardencorev1beta1.ConditionType) {
				It("should return true because condition was added", func() {
					oldShoot := managedResource.DeepCopy()
					managedResource.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition was removed", func() {
					managedResource.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := managedResource.DeepCopy()
					managedResource.Status.Conditions = nil
					Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition status was changed", func() {
					managedResource.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := managedResource.DeepCopy()
					managedResource.Status.Conditions[0].Status = gardencorev1beta1.ConditionTrue
					Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition reason was changed", func() {
					managedResource.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := managedResource.DeepCopy()
					managedResource.Status.Conditions[0].Reason = "reason"
					Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: oldShoot})).To(BeTrue())
				})

				It("should return true because condition message was changed", func() {
					managedResource.Status.Conditions = []gardencorev1beta1.Condition{{Type: conditionType}}
					oldShoot := managedResource.DeepCopy()
					managedResource.Status.Conditions[0].Message = "message"
					Expect(p.Update(event.UpdateEvent{ObjectNew: managedResource, ObjectOld: oldShoot})).To(BeTrue())
				})
			}

			Context("ResourcesApplied condition condition", func() {
				tests(resourcesv1alpha1.ResourcesApplied)
			})

			Context("ResourcesHealthy condition condition", func() {
				tests(resourcesv1alpha1.ResourcesHealthy)
			})

			Context("ResourcesProgressing condition condition", func() {
				tests(resourcesv1alpha1.ResourcesProgressing)
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

	Describe("#RelevantStatusChanged", func() {
		var (
			p                     predicate.Predicate
			extensionBackupBucket *extensionsv1alpha1.BackupBucket
			bucketName            = "bucket"
			extensionBackupEntry  *extensionsv1alpha1.BackupEntry
			entryName             = "entry"
		)

		BeforeEach(func() {
			extensionBackupBucket = &extensionsv1alpha1.BackupBucket{
				ObjectMeta: metav1.ObjectMeta{
					Name: bucketName,
				},
			}
			extensionBackupEntry = &extensionsv1alpha1.BackupEntry{
				ObjectMeta: metav1.ObjectMeta{
					Name: entryName,
				},
			}
			p = LastOperationChanged(GetExtensionLastOperation)
		})

		It("should return false for all events because the extension backupbucket has operation annotation reconcile", func() {
			metav1.SetMetaDataAnnotation(&extensionBackupBucket.ObjectMeta, "gardener.cloud/operation", "reconcile")

			Expect(p.Create(event.CreateEvent{Object: extensionBackupBucket})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: extensionBackupBucket})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: extensionBackupBucket})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: extensionBackupBucket})).To(BeFalse())
		})

		It("should not return false for create events just because the extension backupEntry has operation annotation restore or migrate", func() {
			extensionBackupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
			metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "migrate")

			Expect(p.Create(event.CreateEvent{Object: extensionBackupEntry})).To(BeTrue())

			metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "restore")

			Expect(p.Create(event.CreateEvent{Object: extensionBackupEntry})).To(BeTrue())
		})

		It("should return false for update because the extension backupEntry has operation annotation restore but the old backupEntry doesn't have it", func() {
			oldExtensionBackupEntry := extensionBackupEntry.DeepCopy()
			metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "restore")

			Expect(p.Update(event.UpdateEvent{ObjectOld: oldExtensionBackupEntry, ObjectNew: extensionBackupEntry})).To(BeFalse())
		})

		It("should not return false for update because of the operation annotation restore or migrate when the old backupEntry also have it", func() {
			metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "restore")
			oldExtensionBackupEntry := extensionBackupEntry.DeepCopy()
			extensionBackupEntry.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}

			Expect(p.Update(event.UpdateEvent{ObjectOld: oldExtensionBackupEntry, ObjectNew: extensionBackupEntry})).To(BeTrue())

			metav1.SetMetaDataAnnotation(&extensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "migrate")
			metav1.SetMetaDataAnnotation(&oldExtensionBackupEntry.ObjectMeta, "gardener.cloud/operation", "migrate")

			Expect(p.Update(event.UpdateEvent{ObjectOld: oldExtensionBackupEntry, ObjectNew: extensionBackupEntry})).To(BeTrue())
		})

		It("should return false for create and update because the extension backupbucket status has no lastOperation present", func() {
			extensionBackupBucket.Status.LastOperation = nil
			newExtensionBackupBucket := extensionBackupBucket.DeepCopy()

			Expect(p.Create(event.CreateEvent{Object: extensionBackupBucket})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: extensionBackupBucket})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: extensionBackupBucket})).To(BeFalse())
		})

		It("should return true for create events because the extension backupbucket status lastOperation state is Failed", func() {
			extensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}

			Expect(p.Create(event.CreateEvent{Object: extensionBackupBucket})).To(BeTrue())
		})

		It("should return false for create events because the extension backupbucket status lastOperation state is not Failed", func() {
			extensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}

			Expect(p.Create(event.CreateEvent{Object: extensionBackupBucket})).To(BeFalse())
		})

		It("should return true for  update events because the extension backupbucket status lastOperation state is Succeeded or Error or Failed and the old state is Processing", func() {
			extensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}
			newExtensionBackupBucket := extensionBackupBucket.DeepCopy()

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())
		})

		It("should return true for update events because the extension backupbucket status lastOperation state is Succeeded or Error or Failed and the old state is nil", func() {
			extensionBackupBucket.Status.LastOperation = nil
			newExtensionBackupBucket := extensionBackupBucket.DeepCopy()

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateError}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())

			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateFailed}
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeTrue())
		})

		It("should return false for update events because the extension backupbucket status lastOperation has changed from Succeeded or Error to Processing", func() {
			extensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}
			newExtensionBackupBucket := extensionBackupBucket.DeepCopy()
			newExtensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateProcessing}

			Expect(p.Create(event.CreateEvent{Object: newExtensionBackupBucket})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: newExtensionBackupBucket})).To(BeFalse())
			Expect(p.Generic(event.GenericEvent{Object: newExtensionBackupBucket})).To(BeFalse())
		})

		It("should return false for update events because the extension backupbucket status lastOperation is Succeeded but it's same as old Object", func() {
			extensionBackupBucket.Status.LastOperation = &gardencorev1beta1.LastOperation{State: gardencorev1beta1.LastOperationStateSucceeded}
			newExtensionBackupBucket := extensionBackupBucket.DeepCopy()

			Expect(p.Update(event.UpdateEvent{ObjectNew: newExtensionBackupBucket, ObjectOld: extensionBackupBucket})).To(BeFalse())
		})
	})

	Describe("#ReconciliationFinishedSuccessfully", func() {
		var lastOperation *gardencorev1beta1.LastOperation

		BeforeEach(func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
		})

		It("should return false because last operation is nil on new object", func() {
			oldLastOperation := lastOperation.DeepCopy()
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeFalse())
		})

		It("should return false because last operation type is 'Delete' on old object", func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
			oldLastOperation := lastOperation.DeepCopy()
			oldLastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeFalse())
		})

		It("should return false because last operation type is 'Delete' on new object", func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
			lastOperation.Type = gardencorev1beta1.LastOperationTypeDelete
			oldLastOperation := lastOperation.DeepCopy()
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeFalse())
		})

		It("should return false because last operation type is not 'Processing' on old object", func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
			lastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			lastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
			oldLastOperation := lastOperation.DeepCopy()
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeFalse())
		})

		It("should return false because last operation type is not 'Succeeded' on new object", func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
			lastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			lastOperation.State = gardencorev1beta1.LastOperationStateProcessing
			oldLastOperation := lastOperation.DeepCopy()
			oldLastOperation.State = gardencorev1beta1.LastOperationStateProcessing
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeFalse())
		})

		It("should return true because last operation type is 'Succeeded' on new object", func() {
			lastOperation = &gardencorev1beta1.LastOperation{}
			lastOperation.Type = gardencorev1beta1.LastOperationTypeReconcile
			lastOperation.State = gardencorev1beta1.LastOperationStateSucceeded
			oldLastOperation := lastOperation.DeepCopy()
			oldLastOperation.State = gardencorev1beta1.LastOperationStateProcessing
			Expect(ReconciliationFinishedSuccessfully(oldLastOperation, lastOperation)).To(BeTrue())
		})
	})

	Describe("#HasType", func() {
		var (
			object       client.Object
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			object = &extensionsv1alpha1.Extension{
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: extensionType,
					},
				},
			}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
		})

		It("should match the type", func() {
			predicate := HasType(extensionType)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the type", func() {
			predicate := HasType("anotherType")

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})

	Describe("#HasClass", func() {
		var (
			extensionClass *extensionsv1alpha1.ExtensionClass

			object       client.Object
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		JustBeforeEach(func() {
			object = &extensionsv1alpha1.Extension{
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Class: extensionClass,
					},
				},
			}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
		})

		testAndVerify := func(classes []extensionsv1alpha1.ExtensionClass, match gomegatypes.GomegaMatcher) {
			predicate := HasClass(classes...)

			Expect(predicate.Create(createEvent)).To(match)
			Expect(predicate.Update(updateEvent)).To(match)
			Expect(predicate.Delete(deleteEvent)).To(match)
			Expect(predicate.Generic(genericEvent)).To(match)
		}

		Context("when class is unset", func() {
			It("should match an empty class (nil)", func() {
				testAndVerify(nil, BeTrue())
			})

			It("should match an empty class (empty)", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{""}, BeTrue())
			})

			It("should match the 'shoot' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"shoot"}, BeTrue())
			})

			It("should not match the 'garden' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"garden"}, BeFalse())
			})

			It("should not match multiple classes", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"seed", "garden"}, BeFalse())
			})
		})

		Context("when class is set to 'shoot'", func() {
			BeforeEach(func() {
				extensionClass = ptr.To[extensionsv1alpha1.ExtensionClass]("shoot")
			})

			It("should match an empty class (nil)", func() {
				testAndVerify(nil, BeTrue())
			})

			It("should match an empty class (empty)", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{""}, BeTrue())
			})

			It("should match the 'shoot' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"shoot"}, BeTrue())
			})

			It("should not match the 'garden' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"garden"}, BeFalse())
			})

			It("should not match multiple classes", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"seed", "garden"}, BeFalse())
			})
		})

		Context("when class is set to 'garden'", func() {
			BeforeEach(func() {
				extensionClass = ptr.To[extensionsv1alpha1.ExtensionClass]("garden")
			})

			It("should not match an empty class (nil)", func() {
				testAndVerify(nil, BeFalse())
			})

			It("should not match an empty class (empty)", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{""}, BeFalse())
			})

			It("should not match the 'shoot' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"shoot"}, BeFalse())
			})

			It("should match the 'garden' class", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"garden"}, BeTrue())
			})

			It("should not match multiple classes", func() {
				testAndVerify([]extensionsv1alpha1.ExtensionClass{"shoot", "seed"}, BeFalse())
			})
		})
	})

	Describe("#AddTypePredicate", func() {
		var (
			object           client.Object
			extensionTypeFoo = "foo"

			createEvent   event.CreateEvent
			updateEvent   event.UpdateEvent
			deleteEvent   event.DeleteEvent
			genericEvent  event.GenericEvent
			truePredicate predicate.Predicate = predicate.NewPredicateFuncs(func(client.Object) bool { return true })
		)

		BeforeEach(func() {
			object = &extensionsv1alpha1.Extension{
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: extensionType,
					},
				},
			}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
		})

		It("should add the HasType predicate of the passed extension to the given list of predicates", func() {
			predicates := AddTypeAndClassPredicates([]predicate.Predicate{truePredicate}, extensionsv1alpha1.ExtensionClassShoot, extensionType)

			Expect(predicates).To(HaveLen(3))
			Expect(reflect.ValueOf(predicates[2])).To(Equal(reflect.ValueOf(truePredicate)), "predicate list should contain the passed predicate at last element")
			pred := predicate.And(predicates...)

			Expect(pred.Create(createEvent)).To(BeTrue())
			Expect(pred.Update(updateEvent)).To(BeTrue())
			Expect(pred.Delete(deleteEvent)).To(BeTrue())
			Expect(pred.Generic(genericEvent)).To(BeTrue())

			predicates = AddTypeAndClassPredicates([]predicate.Predicate{truePredicate}, extensionsv1alpha1.ExtensionClassShoot, extensionTypeFoo)

			Expect(predicates).To(HaveLen(3))
			pred = predicate.And(predicates...)

			Expect(pred.Create(createEvent)).To(BeFalse())
			Expect(pred.Update(updateEvent)).To(BeFalse())
			Expect(pred.Delete(deleteEvent)).To(BeFalse())
			Expect(pred.Generic(genericEvent)).To(BeFalse())
		})

		It("should add OR of all the HasType predicates for the passed extensions to the given list of predicates", func() {
			predicates := AddTypeAndClassPredicates([]predicate.Predicate{truePredicate}, extensionsv1alpha1.ExtensionClassShoot, extensionType, extensionTypeFoo)

			Expect(predicates).To(HaveLen(3))
			pred := predicate.And(predicates...)

			Expect(pred.Create(createEvent)).To(BeTrue())
			Expect(pred.Update(updateEvent)).To(BeTrue())
			Expect(pred.Delete(deleteEvent)).To(BeTrue())
			Expect(pred.Generic(genericEvent)).To(BeTrue())

			// checking HasType(extensionTypeFoo)
			object = &extensionsv1alpha1.Extension{
				Spec: extensionsv1alpha1.ExtensionSpec{
					DefaultSpec: extensionsv1alpha1.DefaultSpec{
						Type: extensionTypeFoo,
					},
				},
			}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
			Expect(pred.Create(createEvent)).To(BeTrue())
			Expect(pred.Update(updateEvent)).To(BeTrue())
			Expect(pred.Delete(deleteEvent)).To(BeTrue())
			Expect(pred.Generic(genericEvent)).To(BeTrue())
		})
	})

	Describe("#HasPurpose", func() {
		var (
			object        *extensionsv1alpha1.ControlPlane
			purposeFoo    = extensionsv1alpha1.Purpose("foo")
			purposeNormal = extensionsv1alpha1.Normal

			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			object = &extensionsv1alpha1.ControlPlane{}
			createEvent = event.CreateEvent{
				Object: object,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: object,
				ObjectNew: object,
			}
			deleteEvent = event.DeleteEvent{
				Object: object,
			}
			genericEvent = event.GenericEvent{
				Object: object,
			}
		})

		It("should return true because purpose is 'normal' and spec is nil", func() {
			predicate := HasPurpose(purposeNormal)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should return true because purpose is 'normal' and spec is 'normal'", func() {
			object.Spec.Purpose = &purposeNormal
			predicate := HasPurpose(purposeNormal)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should return false because purpose is not 'normal' and spec is nil", func() {
			predicate := HasPurpose(purposeFoo)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})

		It("should return false because purpose does not match", func() {
			object.Spec.Purpose = &purposeFoo
			predicate := HasPurpose(extensionsv1alpha1.Exposure)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})

		It("should return true because purpose matches", func() {
			object.Spec.Purpose = &purposeFoo
			predicate := HasPurpose(purposeFoo)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})
	})
})
