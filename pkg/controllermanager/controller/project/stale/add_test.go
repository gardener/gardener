// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package stale_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	. "github.com/gardener/gardener/pkg/controllermanager/controller/project/stale"
)

var _ = Describe("Add", func() {
	var reconciler *Reconciler

	BeforeEach(func() {
		reconciler = &Reconciler{}
	})

	Describe("ProjectPredicate", func() {
		var (
			p       predicate.Predicate
			project *gardencorev1beta1.Project
		)

		BeforeEach(func() {
			p = reconciler.ProjectPredicate()
			project = &gardencorev1beta1.Project{}
		})

		Describe("#Create", func() {
			It("should return true", func() {
				Expect(p.Create(event.CreateEvent{})).To(BeTrue())
			})
		})

		Describe("#Update", func() {
			It("should return false when old object is not project", func() {
				Expect(p.Update(event.UpdateEvent{})).To(BeFalse())
			})

			It("should return false when new object is not project", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: project})).To(BeFalse())
			})

			It("should return false when last activity time changed and observed generation is up-to-date", func() {
				oldProject := project.DeepCopy()
				project.Status.LastActivityTimestamp = &metav1.Time{}
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldProject, ObjectNew: project})).To(BeFalse())
			})

			It("should return true when last activity time of old and new object are equal", func() {
				Expect(p.Update(event.UpdateEvent{ObjectOld: project, ObjectNew: project})).To(BeTrue())
			})

			It("should return true when observed generation is not up-to-date", func() {
				oldProject := project.DeepCopy()
				project.Generation++
				Expect(p.Update(event.UpdateEvent{ObjectOld: oldProject, ObjectNew: project})).To(BeTrue())
			})
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
