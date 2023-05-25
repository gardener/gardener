// Copyright 2023 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package networkpolicyregistrar_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/networkpolicyregistrar"
)

var _ = Describe("Add", func() {
	Describe("#Garden Predicate", func() {
		var (
			p      predicate.Predicate
			garden *operatorv1alpha1.Garden
		)

		BeforeEach(func() {
			p = NetworkingPredicate()
			garden = &operatorv1alpha1.Garden{
				Spec: operatorv1alpha1.GardenSpec{
					RuntimeCluster: operatorv1alpha1.RuntimeCluster{
						Networking: operatorv1alpha1.RuntimeNetworking{
							Pods:     "10.1.0.0/16",
							Services: "10.2.0.0/16"},
					},
				},
			}
		})

		Describe("#Create", func() {
			It("should return false because spec.runtimeCluster.networking.pods is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Pods = ""

				Expect(p.Create(event.CreateEvent{Object: garden})).To(BeFalse())
			})

			It("should return false because spec.runtimeCluster.networking.services is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Services = ""

				Expect(p.Create(event.CreateEvent{Object: garden})).To(BeFalse())
			})

			It("should return true because both spec.runtimeCluster.networking.{pods, services} are present", func() {
				Expect(p.Create(event.CreateEvent{Object: garden})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false because spec.runtimeCluster.networking.pods is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Pods = ""

				Expect(p.Update(event.UpdateEvent{ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because spec.runtimeCluster.networking.services is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Services = ""

				Expect(p.Update(event.UpdateEvent{ObjectNew: garden})).To(BeFalse())
			})

			It("should return true because both spec.runtimeCluster.networking.{pods, services} are present", func() {
				Expect(p.Update(event.UpdateEvent{ObjectNew: garden})).To(BeTrue())
			})
		})

		Describe("#Delete", func() {
			It("should return false because spec.runtimeCluster.networking.pods is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Pods = ""

				Expect(p.Delete(event.DeleteEvent{Object: garden})).To(BeFalse())
			})

			It("should return false because spec.runtimeCluster.networking.services is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Services = ""

				Expect(p.Delete(event.DeleteEvent{Object: garden})).To(BeFalse())
			})

			It("should return false even if both spec.runtimeCluster.networking.{pods, services} are present", func() {
				Expect(p.Delete(event.DeleteEvent{Object: garden})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should return false because spec.runtimeCluster.networking.pods is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Pods = ""

				Expect(p.Generic(event.GenericEvent{Object: garden})).To(BeFalse())
			})

			It("should return false because spec.runtimeCluster.networking.services is empty", func() {
				garden.Spec.RuntimeCluster.Networking.Services = ""

				Expect(p.Generic(event.GenericEvent{Object: garden})).To(BeFalse())
			})

			It("should return false even if both spec.runtimeCluster.networking.{pods, services} are present", func() {
				Expect(p.Generic(event.GenericEvent{Object: garden})).To(BeFalse())
			})
		})
	})
})
