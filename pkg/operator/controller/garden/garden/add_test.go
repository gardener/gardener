// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	operatorv1alpha1 "github.com/gardener/gardener/pkg/apis/operator/v1alpha1"
	. "github.com/gardener/gardener/pkg/operator/controller/garden/garden"
)

var _ = Describe("Add", func() {
	Describe("#HasOperationAnnotation", func() {
		var (
			p      predicate.Predicate
			garden *operatorv1alpha1.Garden
		)

		BeforeEach(func() {
			p = (&Reconciler{}).HasOperationAnnotation()
			garden = &operatorv1alpha1.Garden{}
		})

		Describe("#Create", func() {
			It("should return false because no operation annotation present", func() {
				Expect(p.Create(event.CreateEvent{Object: garden})).To(BeFalse())
			})

			DescribeTable("operation annotation present",
				func(operation string, matcher gomegatypes.GomegaMatcher) {
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(p.Create(event.CreateEvent{Object: garden})).To(matcher)
				},

				Entry("reconcile", "reconcile", BeTrue()),
				Entry("rotate-credentials-start", "rotate-credentials-start", BeTrue()),
				Entry("rotate-credentials-complete", "rotate-credentials-complete", BeTrue()),
				Entry("rotate-ca-start", "rotate-ca-start", BeTrue()),
				Entry("rotate-ca-complete", "rotate-ca-complete", BeTrue()),
				Entry("foo", "foo", BeFalse()),
			)
		})

		Describe("#Update", func() {
			It("should return false because no operation annotation present on old or new object", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: garden, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because operation annotation present on both old and new object", func() {
				metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", "reconcile")
				gardenOld := garden.DeepCopy()

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeFalse())
			})

			It("should return false because operation annotation present on old object", func() {
				gardenOld := garden.DeepCopy()
				metav1.SetMetaDataAnnotation(&gardenOld.ObjectMeta, "gardener.cloud/operation", "reconcile")

				Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(BeFalse())
			})

			DescribeTable("operation annotation present only on new object",
				func(operation string, matcher gomegatypes.GomegaMatcher) {
					gardenOld := garden.DeepCopy()
					metav1.SetMetaDataAnnotation(&garden.ObjectMeta, "gardener.cloud/operation", operation)

					Expect(p.Update(event.UpdateEvent{ObjectOld: gardenOld, ObjectNew: garden})).To(matcher)
				},

				Entry("reconcile", "reconcile", BeTrue()),
				Entry("rotate-credentials-start", "rotate-credentials-start", BeTrue()),
				Entry("rotate-credentials-complete", "rotate-credentials-complete", BeTrue()),
				Entry("rotate-ca-start", "rotate-ca-start", BeTrue()),
				Entry("rotate-ca-complete", "rotate-ca-complete", BeTrue()),
				Entry("foo", "foo", BeFalse()),
			)
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
})
