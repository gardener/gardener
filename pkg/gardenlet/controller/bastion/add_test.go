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

package bastion_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/apis/core"
	operationsv1alpha1 "github.com/gardener/gardener/pkg/apis/operations/v1alpha1"
	"github.com/gardener/gardener/pkg/gardenlet/apis/config"
	. "github.com/gardener/gardener/pkg/gardenlet/controller/bastion"
)

var _ = Describe("Add", func() {
	var (
		reconciler *Reconciler
		bastion    *operationsv1alpha1.Bastion
	)

	BeforeEach(func() {
		reconciler = &Reconciler{
			Config: config.GardenletConfiguration{
				SeedConfig: &config.SeedConfig{
					SeedTemplate: core.SeedTemplate{
						ObjectMeta: metav1.ObjectMeta{
							Name: "foo",
						},
					},
				},
			},
		}

		bastion = &operationsv1alpha1.Bastion{}
	})

	Describe("#BastionPredicate", func() {
		var p predicate.Predicate

		BeforeEach(func() {
			p = reconciler.BastionPredicate()
		})

		It("should return false when Bastion's seed name is not same as seed name in GardenletConfiguration seedConfig", func() {
			Expect(p.Create(event.CreateEvent{Object: bastion})).To(BeFalse())
			Expect(p.Update(event.UpdateEvent{ObjectNew: bastion})).To(BeFalse())
			Expect(p.Delete(event.DeleteEvent{Object: bastion})).To(BeFalse())
		})

		It("should return true when Bastion's seed name is same as seed name in GardenletConfiguration seedConfig", func() {
			bastion.Spec.SeedName = pointer.String("foo")
			Expect(p.Create(event.CreateEvent{Object: bastion})).To(BeTrue())
			Expect(p.Update(event.UpdateEvent{ObjectNew: bastion})).To(BeTrue())
			Expect(p.Delete(event.DeleteEvent{Object: bastion})).To(BeTrue())
		})
	})
})
