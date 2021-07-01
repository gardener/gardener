// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package project_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/registry/core/project"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

var _ = Describe("Strategy", func() {
	var (
		ctx = context.TODO()
	)

	Context("member duplicates", func() {
		var (
			duplicateMembers = []core.ProjectMember{
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "system:serviceaccount:foo:bar",
					},
					Roles: []string{
						"role1",
						"role2",
					},
				},
				{
					Subject: rbacv1.Subject{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "bar",
						Namespace: "foo",
					},
					Roles: []string{
						"role2",
						"role3",
						"role4",
						"role5",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.GroupKind,
						Name:     "baz",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.GroupKind,
						Name:     "baz",
					},
					Roles: []string{
						"role0",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "bazz",
					},
					Roles: []string{
						"role-1",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "bazz",
					},
				},
			}

			expectedMembers = []core.ProjectMember{
				{
					Subject: rbacv1.Subject{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "bar",
						Namespace: "foo",
					},
					Roles: []string{
						"role1",
						"role2",
						"role3",
						"role4",
						"role5",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.GroupKind,
						Name:     "baz",
					},
					Roles: []string{
						"role0",
					},
				},
				{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "bazz",
					},
					Roles: []string{
						"role-1",
					},
				},
			}
		)

		Describe("#PrepareForCreate", func() {
			It("should merge duplicate members", func() {
				obj := &core.Project{
					Spec: core.ProjectSpec{
						Members: duplicateMembers,
					},
				}

				Strategy.PrepareForCreate(ctx, obj)

				Expect(obj.Generation).To(Equal(int64(1)))
				Expect(obj.Status).To(Equal(core.ProjectStatus{}))
				Expect(obj.Spec.Members).To(Equal(expectedMembers))
			})
		})

		Describe("#PrepareForUpdate", func() {
			It("should merge duplicate members", func() {
				obj := &core.Project{
					Spec: core.ProjectSpec{
						Members: duplicateMembers,
					},
					Status: core.ProjectStatus{ObservedGeneration: 123},
				}

				newObj := obj.DeepCopy()
				newObj.Spec.Description = pointer.String("new description")
				newObj.Status = core.ProjectStatus{}

				Strategy.PrepareForUpdate(ctx, newObj, obj)

				Expect(newObj.Generation).To(Equal(int64(1)))
				Expect(newObj.Status).To(Equal(obj.Status))
				Expect(newObj.Spec.Members).To(Equal(expectedMembers))
			})
		})
	})
})

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
