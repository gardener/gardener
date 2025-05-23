// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/gardener/gardener/pkg/controllermanager/controller/project/project"
)

var _ = Describe("RoleBindingPredicate", func() {
	var (
		p           predicate.Predicate
		roleBinding *rbacv1.RoleBinding
	)

	BeforeEach(func() {
		p = (&project.Reconciler{}).RoleBindingPredicate()
		roleBinding = &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				ResourceVersion: "1",
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName,
				Kind:     "ClusterRole",
				Name:     "project-member",
			},
			Subjects: []rbacv1.Subject{{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     "admin",
			}},
		}
	})

	Describe("#Create", func() {
		It("should return false", func() {
			Expect(p.Create(event.CreateEvent{})).To(BeFalse())
		})
	})

	Describe("#Update", func() {
		It("should return true for periodic cache resyncs", func() {
			Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: roleBinding.DeepCopy()})).To(BeTrue())
		})

		It("should return true if roleRef changed", func() {
			oldRoleBinding := roleBinding.DeepCopy()
			roleBinding.ResourceVersion = "2"
			roleBinding.RoleRef.Name = "other"

			Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeTrue())
		})

		It("should return true if subjects changed", func() {
			oldRoleBinding := roleBinding.DeepCopy()
			roleBinding.ResourceVersion = "2"
			roleBinding.Subjects = append(roleBinding.Subjects, rbacv1.Subject{
				APIGroup: rbacv1.GroupName,
				Kind:     rbacv1.UserKind,
				Name:     "viewer",
			})

			Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeTrue())
		})

		It("should return false if something else changed", func() {
			oldRoleBinding := roleBinding.DeepCopy()
			roleBinding.ResourceVersion = "2"
			metav1.SetMetaDataLabel(&roleBinding.ObjectMeta, "foo", "bar")

			Expect(p.Update(event.UpdateEvent{ObjectNew: roleBinding, ObjectOld: oldRoleBinding})).To(BeFalse())
		})
	})

	Describe("#Delete", func() {
		It("should return true", func() {
			Expect(p.Delete(event.DeleteEvent{})).To(BeTrue())
		})
	})

	Describe("#Generic", func() {
		It("should return false", func() {
			Expect(p.Generic(event.GenericEvent{})).To(BeFalse())
		})
	})
})
