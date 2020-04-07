// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	extensionscontroller "github.com/gardener/gardener/extensions/pkg/controller"
	extensionspredicate "github.com/gardener/gardener/extensions/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("Predicate", func() {
	Describe("#HasType", func() {
		var (
			extensionType string
			object        runtime.Object
			createEvent   event.CreateEvent
			updateEvent   event.UpdateEvent
			deleteEvent   event.DeleteEvent
			genericEvent  event.GenericEvent
		)

		BeforeEach(func() {
			extensionType = "extension-type"
			object = &v1alpha1.Extension{
				Spec: v1alpha1.ExtensionSpec{
					DefaultSpec: v1alpha1.DefaultSpec{
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
			predicate := extensionspredicate.HasType(extensionType)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the type", func() {
			predicate := extensionspredicate.HasType("anotherType")

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})

	Describe("#HasName", func() {
		var (
			name         string
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			objectMeta := metav1.ObjectMeta{
				Name: name,
			}
			createEvent = event.CreateEvent{
				Meta: &objectMeta,
			}
			updateEvent = event.UpdateEvent{
				MetaNew: &objectMeta,
				MetaOld: &objectMeta,
			}
			deleteEvent = event.DeleteEvent{
				Meta: &objectMeta,
			}
			genericEvent = event.GenericEvent{
				Meta: &objectMeta,
			}
		})

		It("should match the name", func() {
			predicate := extensionspredicate.HasName(name)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the name", func() {
			predicate := extensionspredicate.HasName("anotherName")

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})

	const (
		extensionType = "extension-type"
		version       = "1.18"
	)

	Describe("#ClusterShootProviderType", func() {
		var (
			decoder runtime.Decoder
			err     error
		)

		BeforeEach(func() {
			decoder, err = extensionscontroller.NewGardenDecoder()
			Expect(err).To(Succeed())
		})

		It("should match the type", func() {
			var (
				predicate                                           = extensionspredicate.ClusterShootProviderType(decoder, extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, version)
			)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the type", func() {
			var (
				predicate                                           = extensionspredicate.ClusterShootProviderType(decoder, extensionType)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents("other-extension-type", version)
			)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})

	Describe("#ClusterShootKubernetesVersionAtLeast", func() {
		var (
			decoder runtime.Decoder
			err     error
		)

		BeforeEach(func() {
			decoder, err = extensionscontroller.NewGardenDecoder()
			Expect(err).To(Succeed())
		})

		It("should match the minimum kubernetes version", func() {
			var (
				predicate                                           = extensionspredicate.ClusterShootKubernetesVersionAtLeast(decoder, version)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, version)
			)

			Expect(predicate.Create(createEvent)).To(BeTrue())
			Expect(predicate.Update(updateEvent)).To(BeTrue())
			Expect(predicate.Delete(deleteEvent)).To(BeTrue())
			Expect(predicate.Generic(genericEvent)).To(BeTrue())
		})

		It("should not match the minimum kubernetes version", func() {
			var (
				predicate                                           = extensionspredicate.ClusterShootKubernetesVersionAtLeast(decoder, version)
				createEvent, updateEvent, deleteEvent, genericEvent = computeEvents(extensionType, "1.17")
			)

			Expect(predicate.Create(createEvent)).To(BeFalse())
			Expect(predicate.Update(updateEvent)).To(BeFalse())
			Expect(predicate.Delete(deleteEvent)).To(BeFalse())
			Expect(predicate.Generic(genericEvent)).To(BeFalse())
		})
	})
})

func computeEvents(extensionType, kubernetesVersion string) (event.CreateEvent, event.UpdateEvent, event.DeleteEvent, event.GenericEvent) {
	shoot := &gardencorev1beta1.Shoot{
		TypeMeta: metav1.TypeMeta{
			APIVersion: gardencorev1beta1.SchemeGroupVersion.String(),
			Kind:       "Shoot",
		},
		Spec: gardencorev1beta1.ShootSpec{
			Provider: gardencorev1beta1.Provider{
				Type: extensionType,
			},
			Kubernetes: gardencorev1beta1.Kubernetes{
				Version: kubernetesVersion,
			},
		},
	}

	shootJSON, err := json.Marshal(shoot)
	Expect(err).To(Succeed())

	cluster := &extensionsv1alpha1.Cluster{
		Spec: extensionsv1alpha1.ClusterSpec{
			Shoot: runtime.RawExtension{Raw: shootJSON},
		},
	}

	return event.CreateEvent{Object: cluster},
		event.UpdateEvent{ObjectOld: cluster, ObjectNew: cluster},
		event.DeleteEvent{Object: cluster},
		event.GenericEvent{Object: cluster}
}
