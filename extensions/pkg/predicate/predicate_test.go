// Copyright 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package predicate_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	. "github.com/gardener/gardener/extensions/pkg/predicate"
	gardencore "github.com/gardener/gardener/pkg/apis/core"
	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
)

var _ = Describe("Predicate", func() {
	var (
		providerType  = "provider-type"
		extensionType = "extension-type"
		version       = "1.18"
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

	Describe("#AddTypePredicate", func() {
		var (
			object           client.Object
			extensionTypeFoo = "foo"

			purposeNormal = extensionsv1alpha1.Normal

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

		It("should add the HasType predicate of the passed extension to the given list of predicates", func() {
			predicates := AddTypePredicate([]predicate.Predicate{HasPurpose(purposeNormal)}, extensionType)

			Expect(len(predicates)).To(Equal(2))

			Expect(predicates[1].Create(createEvent)).To(BeTrue())
			Expect(predicates[1].Update(updateEvent)).To(BeTrue())
			Expect(predicates[1].Delete(deleteEvent)).To(BeTrue())
			Expect(predicates[1].Generic(genericEvent)).To(BeTrue())
		})

		It("should add OR of all the HasType predicates for the passed extensions to the given list of predicates", func() {
			predicates := AddTypePredicate([]predicate.Predicate{HasPurpose(purposeNormal)}, extensionType, extensionTypeFoo)

			Expect(len(predicates)).To(Equal(2))

			Expect(predicates[1].Create(createEvent)).To(BeTrue())
			Expect(predicates[1].Update(updateEvent)).To(BeTrue())
			Expect(predicates[1].Delete(deleteEvent)).To(BeTrue())
			Expect(predicates[1].Generic(genericEvent)).To(BeTrue())

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
			Expect(predicates[1].Create(createEvent)).To(BeTrue())
			Expect(predicates[1].Update(updateEvent)).To(BeTrue())
			Expect(predicates[1].Delete(deleteEvent)).To(BeTrue())
			Expect(predicates[1].Generic(genericEvent)).To(BeTrue())
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

	Describe("#ClusterShootProviderType", func() {
		It("should match the type", func() {
			var (
				predicate                                           = ClusterShootProviderType(extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, version, nil)
			)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the type", func() {
			var (
				predicate                                           = ClusterShootProviderType(extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents("other-extension-type", version, nil)
			)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})

	Describe("#GardenCoreProviderType", func() {
		var (
			object       client.Object
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			object = &gardencore.CloudProfile{
				Spec: gardencore.CloudProfileSpec{
					Type: providerType,
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

		It("the predicate should return true for the same providerType", func() {
			predicate := GardenCoreProviderType(providerType)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("the predicate should return false for different providerType", func() {
			predicate := GardenCoreProviderType("other-extension")

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
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

func computeEvents(
	extensionType string,
	kubernetesVersion string,
	kubernetesVersionOverwriteAnnotation *string,
) (
	event.CreateEvent,
	event.UpdateEvent,
	event.DeleteEvent,
	event.GenericEvent,
) {
	spec := gardencorev1beta1.ShootSpec{
		Provider: gardencorev1beta1.Provider{
			Type: extensionType,
		},
		Kubernetes: gardencorev1beta1.Kubernetes{
			Version: kubernetesVersion,
		},
	}

	var meta *metav1.ObjectMeta
	if kubernetesVersionOverwriteAnnotation != nil {
		meta = &metav1.ObjectMeta{
			Annotations: map[string]string{
				"alpha.csimigration.shoot.extensions.gardener.cloud/kubernetes-version": *kubernetesVersionOverwriteAnnotation,
			},
		}
	}

	cluster := computeClusterWithShoot("", meta, &spec, nil)

	return event.CreateEvent{Object: cluster},
		event.UpdateEvent{ObjectOld: cluster, ObjectNew: cluster},
		event.DeleteEvent{Object: cluster},
		event.GenericEvent{Object: cluster}
}
