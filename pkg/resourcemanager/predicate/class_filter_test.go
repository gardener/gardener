// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package predicate_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/event"

	resourcesv1alpha1 "github.com/gardener/gardener/pkg/apis/resources/v1alpha1"
	. "github.com/gardener/gardener/pkg/resourcemanager/predicate"
)

var _ = Describe("ClassFilter", func() {
	var (
		filter *ClassFilter

		differentClass     = "diff"
		differentFinalizer = fmt.Sprintf("%s-%s", FinalizerName, differentClass)

		mrWithoutFinalizerDifferentClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: &differentClass,
			},
		}

		mrDifferentFinalizerDifferentClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{differentFinalizer},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: &differentClass,
			},
		}

		mrSameFinalizerDifferentClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{FinalizerName},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: &differentClass,
			},
		}

		mrWithoutFinalizerSameClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To(""),
			},
		}

		mrDifferentFinalizerSameClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{differentFinalizer},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To(""),
			},
		}

		mrSameFinalizerSameClass = &resourcesv1alpha1.ManagedResource{
			ObjectMeta: metav1.ObjectMeta{
				Finalizers: []string{FinalizerName},
			},
			Spec: resourcesv1alpha1.ManagedResourceSpec{
				Class: ptr.To(""),
			},
		}
	)

	When("class is empty", func() {
		BeforeEach(func() {
			filter = NewClassFilter("")
		})

		DescribeTable("Responsible",
			func(mr *resourcesv1alpha1.ManagedResource, responsible bool) {
				resp := filter.Responsible(mr)
				Expect(resp).To(Equal(responsible))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, false),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, true),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, true),
		)

		DescribeTable("IsTransferringResponsibility",
			func(mr *resourcesv1alpha1.ManagedResource, shouldCleanup bool) {
				cleanup := filter.IsTransferringResponsibility(mr)
				Expect(cleanup).To(Equal(shouldCleanup))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, true),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, false),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, false),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, false),
		)

		DescribeTable("IsWaitForCleanupRequired",
			func(mr *resourcesv1alpha1.ManagedResource, shouldWait bool) {
				wait := filter.IsWaitForCleanupRequired(mr)
				Expect(wait).To(Equal(shouldWait))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, false),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, false),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, false),
		)

		DescribeTable("Create",
			func(mr *resourcesv1alpha1.ManagedResource, expected bool) {
				got := filter.Create(event.CreateEvent{
					Object: mr,
				})
				Expect(got).To(Equal(expected))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, true),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, true),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, true),
		)

		DescribeTable("Delete",
			func(mr *resourcesv1alpha1.ManagedResource, expected bool) {
				got := filter.Delete(event.DeleteEvent{
					Object: mr,
				})
				Expect(got).To(Equal(expected))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, true),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, true),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, true),
		)

		DescribeTable("Update",
			func(mr *resourcesv1alpha1.ManagedResource, expected bool) {
				got := filter.Update(event.UpdateEvent{
					ObjectNew: mr,
				})
				Expect(got).To(Equal(expected))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, true),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, true),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, true),
		)

		DescribeTable("Generic",
			func(mr *resourcesv1alpha1.ManagedResource, expected bool) {
				got := filter.Generic(event.GenericEvent{
					Object: mr,
				})
				Expect(got).To(Equal(expected))
			},
			Entry("MR without a finalizer and with different class", mrWithoutFinalizerDifferentClass, false),
			Entry("MR with different finalizer and with different class", mrDifferentFinalizerDifferentClass, false),
			Entry("MR with same finalizer and with different class", mrSameFinalizerDifferentClass, true),
			Entry("MR without a finalizer and with same class", mrWithoutFinalizerSameClass, true),
			Entry("MR with different finalizer and with same class", mrDifferentFinalizerSameClass, true),
			Entry("MR with same finalizer and with same class", mrSameFinalizerSameClass, true),
		)
	})

	When("class is 'all'", func() {
		BeforeEach(func() {
			filter = NewClassFilter("*")
		})

		Describe("#Responsible", func() {
			It("should be responsible for any class", func() {
				Expect(filter.Responsible(&resourcesv1alpha1.ManagedResource{})).To(BeTrue())
				Expect(filter.Responsible(&resourcesv1alpha1.ManagedResource{Spec: resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("")}})).To(BeTrue())
				Expect(filter.Responsible(&resourcesv1alpha1.ManagedResource{Spec: resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("foo")}})).To(BeTrue())
				Expect(filter.Responsible(&resourcesv1alpha1.ManagedResource{Spec: resourcesv1alpha1.ManagedResourceSpec{Class: ptr.To("bar")}})).To(BeTrue())
			})
		})
	})
})
