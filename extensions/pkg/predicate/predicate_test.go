// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	"encoding/json"
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/extensions/pkg/predicate"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Predicate", func() {
	var (
		extensionType = "extension-type"
	)

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

func computeClusterWithShoot(
	name string,
	shootMeta *metav1.ObjectMeta,
	shootSpec *gardencorev1beta1.ShootSpec,
	shootStatus *gardencorev1beta1.ShootStatus,
) *extensionsv1alpha1.Cluster {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		},
	}

	if shootMeta != nil {
		shoot.ObjectMeta = *shootMeta
	}
	if shootSpec != nil {
		shoot.Spec = *shootSpec
	}
	if shootStatus != nil {
		shoot.Status = *shootStatus
	}

	shootJSON, err := json.Marshal(shoot)
	Expect(err).To(Succeed())

	return &extensionsv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{Raw: shootJSON},
		},
	}
}
