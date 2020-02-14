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

package v1beta1_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Conversion", func() {
	var scheme *runtime.Scheme

	BeforeSuite(func() {
		scheme = runtime.NewScheme()
		Expect(scheme.AddConversionFuncs(
			Convert_v1beta1_Project_To_core_Project,
			Convert_core_Project_To_v1beta1_Project,
			Convert_v1beta1_ProjectSpec_To_core_ProjectSpec,
			Convert_core_ProjectSpec_To_v1beta1_ProjectSpec,
			Convert_v1beta1_ProjectMember_To_core_ProjectMember,
			Convert_core_ProjectMember_To_v1beta1_ProjectMember,
		)).NotTo(HaveOccurred())
	})

	Context("project conversions", func() {
		Describe("#Convert_v1beta1_ProjectSpec_To_core_ProjectSpec", func() {
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

				out *core.Project
				in  *Project
			)

			BeforeEach(func() {
				out = &core.Project{}
				in = &Project{}
			})

			It("should do nothing if owner is not (yet) a member", func() {
				in.Spec = ProjectSpec{
					Owner: &owner,
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
					},
				}))
			})

			It("should add the owner role to the owner member (not present yet)", func() {
				in.Spec = ProjectSpec{
					Owner: &owner,
					Members: []ProjectMember{
						{Subject: member1},
						{Subject: owner},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
						Members: []core.ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Roles:   []string{core.ProjectMemberOwner},
							},
							{Subject: member2},
						},
					},
				}))
			})

			It("should do nothing if the owner role is already present for the owner member", func() {
				in.Spec = ProjectSpec{
					Owner: &owner,
					Members: []ProjectMember{
						{Subject: member1},
						{
							Subject: owner,
							Roles:   []string{ProjectMemberOwner},
						},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
						Members: []core.ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Roles:   []string{core.ProjectMemberOwner},
							},
							{Subject: member2},
						},
					},
				}))
			})

			It("should remove the owner role from all non-owner members", func() {
				in.Spec = ProjectSpec{
					Owner: &owner,
					Members: []ProjectMember{
						{
							Subject: member1,
							Roles:   []string{ProjectMemberOwner},
						},
						{Subject: owner},
						{
							Subject: member2,
							Roles:   []string{ProjectMemberOwner},
						},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
						Members: []core.ProjectMember{
							{
								Subject: member1,
								Roles:   []string{},
							},
							{
								Subject: owner,
								Roles:   []string{core.ProjectMemberOwner},
							},
							{
								Subject: member2,
								Roles:   []string{},
							},
						},
					},
				}))
			})
		})

		Describe("#Convert_core_ProjectSpec_To_v1beta1_ProjectSpec", func() {
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
				ownerRole = ProjectMemberOwner

				out *Project
				in  *core.Project
			)

			BeforeEach(func() {
				out = &Project{}
				in = &core.Project{}
			})

			It("should do nothing if owner is not (yet) a member", func() {
				in.Spec = core.ProjectSpec{
					Owner: &owner,
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
					},
				}))
			})

			It("should add the owner role to the owner member (not present yet)", func() {
				in.Spec = core.ProjectSpec{
					Owner: &owner,
					Members: []core.ProjectMember{
						{Subject: member1},
						{Subject: owner},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Roles:   []string{ProjectMemberOwner},
							},
							{Subject: member2},
						},
					},
				}))
			})

			It("should do nothing if the owner role is already present for the owner member", func() {
				in.Spec = core.ProjectSpec{
					Owner: &owner,
					Members: []core.ProjectMember{
						{Subject: member1},
						{
							Subject: owner,
							Roles:   []string{core.ProjectMemberOwner},
						},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Role:    &ownerRole,
								Roles:   []string{ProjectMemberOwner},
							},
							{Subject: member2},
						},
					},
				}))
			})

			It("should remove the owner role from all non-owner members", func() {
				in.Spec = core.ProjectSpec{
					Owner: &owner,
					Members: []core.ProjectMember{
						{
							Subject: member1,
							Roles:   []string{core.ProjectMemberOwner},
						},
						{Subject: owner},
						{
							Subject: member2,
							Roles:   []string{core.ProjectMemberOwner},
						},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{
								Subject: member1,
								Roles:   []string{},
							},
							{
								Subject: owner,
								Roles:   []string{ProjectMemberOwner},
							},
							{
								Subject: member2,
								Roles:   []string{},
							},
						},
					},
				}))
			})
		})

		Describe("#Convert_v1beta1_ProjectMember_To_core_ProjectMember", func() {
			var (
				role = "foo"

				in  *ProjectMember
				out *core.ProjectMember
			)

			BeforeEach(func() {
				in = &ProjectMember{}
				out = &core.ProjectMember{}
			})

			It("should do nothing because role not set", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.ProjectMember{}))
			})

			It("should do nothing because role was found", func() {
				in = &ProjectMember{
					Role:  &role,
					Roles: []string{role, "bar"},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar"},
				}))
			})

			It("should add the role at the head of roles list", func() {
				in = &ProjectMember{
					Role:  &role,
					Roles: []string{"bar"},
				}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar"},
				}))
			})
		})

		Describe("#Convert_core_ProjectMember_To_v1beta1_ProjectMember", func() {
			var (
				role = "foo"

				in  *core.ProjectMember
				out *ProjectMember
			)

			BeforeEach(func() {
				in = &core.ProjectMember{}
				out = &ProjectMember{}
			})

			It("should do nothing because roles are not set", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&ProjectMember{}))
			})

			It("should add the first role to the role field", func() {
				in.Roles = []string{role, "bar"}

				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&ProjectMember{
					Role:  &role,
					Roles: []string{role, "bar"},
				}))
			})
		})
	})
})
