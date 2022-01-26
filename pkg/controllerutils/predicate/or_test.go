// Copyright 2018 The Kubernetes Authors.
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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	predicateutils "github.com/gardener/gardener/pkg/controllerutils/predicate"
)

var _ = Describe("Or", func() {
	Describe("#Or", func() {
		funcs := func(pass bool) predicate.Funcs {
			return predicate.Funcs{
				CreateFunc: func(event.CreateEvent) bool {
					return pass
				},
				DeleteFunc: func(event.DeleteEvent) bool {
					return pass
				},
				UpdateFunc: func(event.UpdateEvent) bool {
					return pass
				},
				GenericFunc: func(event.GenericEvent) bool {
					return pass
				},
			}
		}
		passFuncs := funcs(true)
		failFuncs := funcs(false)

		It("should return true when one of its predicates returns true", func() {
			o := predicateutils.Or(passFuncs, failFuncs)
			Expect(o.Create(event.CreateEvent{})).To(BeTrue())
			Expect(o.Update(event.UpdateEvent{})).To(BeTrue())
			Expect(o.Delete(event.DeleteEvent{})).To(BeTrue())
			Expect(o.Generic(event.GenericEvent{})).To(BeTrue())
		})

		It("should return false when all of its predicates return false", func() {
			o := predicateutils.Or(failFuncs, failFuncs)
			Expect(o.Create(event.CreateEvent{})).To(BeFalse())
			Expect(o.Update(event.UpdateEvent{})).To(BeFalse())
			Expect(o.Delete(event.DeleteEvent{})).To(BeFalse())
			Expect(o.Generic(event.GenericEvent{})).To(BeFalse())
		})
	})
})
