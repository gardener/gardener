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

package predicate_test

import (
	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/predicate"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

var _ = Describe("ignore mode", func() {
	var (
		managedResource *resourcesv1alpha1.ManagedResource
		predicate       predicate.Predicate
	)

	BeforeEach(func() {
		managedResource = &resourcesv1alpha1.ManagedResource{}
	})

	Describe("#NotIgnoreMode", func() {
		BeforeEach(func() {
			predicate = NotIgnoreMode()
		})

		Context("#Create", func() {
			It("should match because no ignore mode annotation", func() {
				Expect(predicate.Create(event.CreateEvent{Object: managedResource})).To(BeTrue())
			})

			It("should not match because ignore mode annotation", func() {
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")

				Expect(predicate.Create(event.CreateEvent{Object: managedResource})).To(BeFalse())
			})
		})

		Context("#Update", func() {
			It("should match because no ignore mode annotation", func() {
				Expect(predicate.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeTrue())
			})

			It("should not match because ignore mode annotation", func() {
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")

				Expect(predicate.Update(event.UpdateEvent{ObjectNew: managedResource})).To(BeFalse())
			})
		})

		Describe("#Delete", func() {
			It("should match because no ignore mode annotation", func() {
				Expect(predicate.Delete(event.DeleteEvent{Object: managedResource})).To(BeTrue())
			})

			It("should not match because ignore mode annotation", func() {
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")

				Expect(predicate.Delete(event.DeleteEvent{Object: managedResource})).To(BeFalse())
			})
		})

		Describe("#Generic", func() {
			It("should match because no ignore mode annotation", func() {
				Expect(predicate.Generic(event.GenericEvent{Object: managedResource})).To(BeTrue())
			})

			It("should not match because ignore mode annotation", func() {
				metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")

				Expect(predicate.Generic(event.GenericEvent{Object: managedResource})).To(BeFalse())
			})
		})
	})

	Describe("#IgnoreModeRemoved()", func() {
		BeforeEach(func() {
			predicate = IgnoreModeRemoved()
		})

		Context("#Create", func() {
			It("should always return true", func() {
				Expect(predicate.Create(event.CreateEvent{Object: managedResource})).To(BeTrue())
			})
		})

		Context("#Update", func() {
			DescribeTable("#Update",
				func(oldHasIgnoreMode, newHasIgnoreMode bool, matcher gomegatypes.GomegaMatcher) {
					old := managedResource.DeepCopy()

					if oldHasIgnoreMode {
						metav1.SetMetaDataAnnotation(&old.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")
					}
					if newHasIgnoreMode {
						metav1.SetMetaDataAnnotation(&managedResource.ObjectMeta, "resources.gardener.cloud/mode", "Ignore")
					}

					Expect(predicate.Update(event.UpdateEvent{
						ObjectNew: managedResource,
						ObjectOld: old,
					})).To(matcher)
				},

				Entry("old and new not ignored", false, false, BeFalse()),
				Entry("old ignored, new not ignored", true, false, BeTrue()),
				Entry("old not ignored, new ignored", false, true, BeFalse()),
				Entry("old and new ignored", true, true, BeFalse()),
			)
		})

		Describe("#Delete", func() {
			It("should always return true", func() {
				Expect(predicate.Delete(event.DeleteEvent{Object: managedResource})).To(BeTrue())
			})
		})

		Describe("#Generic", func() {
			It("should always return true", func() {
				Expect(predicate.Generic(event.GenericEvent{Object: managedResource})).To(BeTrue())
			})
		})
	})
})
