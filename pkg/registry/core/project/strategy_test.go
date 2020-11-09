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
				newObj.Spec.Description = pointer.StringPtr("new description")
				newObj.Status = core.ProjectStatus{}

				Strategy.PrepareForUpdate(ctx, newObj, obj)

				Expect(newObj.Generation).To(Equal(int64(1)))
				Expect(newObj.Status).To(Equal(obj.Status))
				Expect(newObj.Spec.Members).To(Equal(expectedMembers))
			})
		})
	})
})
