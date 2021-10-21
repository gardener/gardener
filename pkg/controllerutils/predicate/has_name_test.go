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
	. "github.com/gardener/gardener/pkg/controllerutils/predicate"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

var _ = Describe("Predicate", func() {
	Describe("#HasName", func() {
		var (
			name         string
			createEvent  event.CreateEvent
			updateEvent  event.UpdateEvent
			deleteEvent  event.DeleteEvent
			genericEvent event.GenericEvent
		)

		BeforeEach(func() {
			configMap := &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
				},
			}

			createEvent = event.CreateEvent{
				Object: configMap,
			}
			updateEvent = event.UpdateEvent{
				ObjectOld: configMap,
				ObjectNew: configMap,
			}
			deleteEvent = event.DeleteEvent{
				Object: configMap,
			}
			genericEvent = event.GenericEvent{
				Object: configMap,
			}
		})

		It("should match the name", func() {
			predicate := HasName(name)

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeTrue())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeTrue())
		})

		It("should not match the name", func() {
			predicate := HasName("anotherName")

			gomega.Expect(predicate.Create(createEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Update(updateEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Delete(deleteEvent)).To(gomega.BeFalse())
			gomega.Expect(predicate.Generic(genericEvent)).To(gomega.BeFalse())
		})
	})
})
