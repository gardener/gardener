// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package project_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apiserver/registry/core/project"
)

var _ = Describe("ToSelectableFields", func() {
	It("should return correct fields", func() {
		result := ToSelectableFields(newProject("foo"))

		Expect(result).To(HaveLen(2))
		Expect(result.Has(core.ProjectNamespace)).To(BeTrue())
		Expect(result.Get(core.ProjectNamespace)).To(Equal("foo"))
	})
})

var _ = Describe("GetAttrs", func() {
	It("should return error when object is not Project", func() {
		_, _, err := GetAttrs(&core.Seed{})
		Expect(err).To(HaveOccurred())
	})

	It("should return correct result", func() {
		ls, fs, err := GetAttrs(newProject("foo"))

		Expect(ls).To(HaveLen(1))
		Expect(ls.Get("foo")).To(Equal("bar"))
		Expect(fs.Get(core.ProjectNamespace)).To(Equal("foo"))
		Expect(err).NotTo(HaveOccurred())
	})
})

var _ = Describe("NamespaceTriggerFunc", func() {
	It("should return spec.namespace", func() {
		actual := NamespaceTriggerFunc(newProject("foo"))
		Expect(actual).To(Equal("foo"))
	})
})

var _ = Describe("MatchProject", func() {
	It("should return correct predicate", func() {
		ls, _ := labels.Parse("app=test")
		fs := fields.OneTermEqualSelector(core.ProjectNamespace, "foo")

		result := MatchProject(ls, fs)

		Expect(result.Label).To(Equal(ls))
		Expect(result.Field).To(Equal(fs))
		Expect(result.IndexFields).To(ConsistOf(core.ProjectNamespace))
	})
})

var _ = Describe("Canonicalize", func() {
	var (
		owner = rbacv1.Subject{
			APIGroup: "group",
			Kind:     "kind",
			Name:     "owner",
		}
		member1 = rbacv1.Subject{
			APIGroup: "group",
			Kind:     "kind",
			Name:     "member1",
		}
		member2 = rbacv1.Subject{
			APIGroup: "group",
			Kind:     "kind",
			Name:     "member2",
		}
		member3 = rbacv1.Subject{
			APIGroup: "group",
			Kind:     "kind",
			Name:     "member3",
		}
		extensionRole = "extension:role"
	)

	It("should do nothing if owner is nil", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{core.ProjectMemberOwner}},
				},
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{core.ProjectMemberOwner}},
				},
			},
		}))
	})

	It("should do nothing if owner is not (yet) a member", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
			},
		}))
	})

	It("should add the owner role to the owner member when not present", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1},
					{Subject: owner},
					{Subject: member2},
				},
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1},
					{Subject: owner, Roles: []string{core.ProjectMemberOwner}},
					{Subject: member2},
				},
			},
		}))
	})

	It("should do nothing if the owner role is already present for the owner member", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1},
					{Subject: owner, Roles: []string{core.ProjectMemberOwner}},
					{Subject: member2},
				},
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1},
					{Subject: owner, Roles: []string{core.ProjectMemberOwner}},
					{Subject: member2},
				},
			},
		}))
	})

	It("should remove the owner role from all non-owner members", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{core.ProjectMemberOwner}},
					{Subject: owner},
					{Subject: member2, Roles: []string{core.ProjectMemberOwner}},
					{Subject: member3, Roles: []string{core.ProjectMemberOwner, extensionRole, core.ProjectMemberOwner}},
				},
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{}},
					{Subject: owner, Roles: []string{core.ProjectMemberOwner}},
					{Subject: member2, Roles: []string{}},
					{Subject: member3, Roles: []string{extensionRole}},
				},
			},
		}))
	})

	It("should both add owner role to owner member and remove it from non-owner members", func() {
		project := &core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{core.ProjectMemberOwner, extensionRole}},
					{Subject: owner, Roles: []string{extensionRole}},
					{Subject: member2},
				},
			},
		}
		Strategy.Canonicalize(project)
		Expect(project).To(Equal(&core.Project{
			Spec: core.ProjectSpec{
				Owner: &owner,
				Members: []core.ProjectMember{
					{Subject: member1, Roles: []string{extensionRole}},
					{Subject: owner, Roles: []string{extensionRole, core.ProjectMemberOwner}},
					{Subject: member2},
				},
			},
		}))
	})
})

func newProject(namespace string) *core.Project {
	return &core.Project{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "test",
			Labels: map[string]string{"foo": "bar"},
		},
		Spec: core.ProjectSpec{
			Namespace: &namespace,
		},
	}
}
