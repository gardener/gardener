// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/utils/ptr"

	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var _ = Describe("Project defaulting", func() {
	var obj *Project

	BeforeEach(func() {
		obj = &Project{}
	})

	Describe("toleration defaulting", func() {
		It("should not add the 'protected' toleration if the namespace is not 'garden' (w/o existing project tolerations)", func() {
			obj.Spec.Namespace = ptr.To("foo")

			SetObjectDefaults_Project(obj)

			Expect(obj.Spec.Tolerations).To(BeNil())
		})

		It("should not add the 'protected' toleration if the namespace is not 'garden' (w/ existing project tolerations)", func() {
			obj.Spec.Namespace = ptr.To("foo")
			obj.Spec.Tolerations = &ProjectTolerations{
				Defaults:  []Toleration{{Key: "foo"}},
				Whitelist: []Toleration{{Key: "bar"}},
			}

			SetObjectDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: "foo"}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: "bar"}}))
		})

		It("should add the 'protected' toleration if the namespace is 'garden' (w/o existing project tolerations)", func() {
			obj.Spec.Namespace = ptr.To(v1beta1constants.GardenNamespace)
			obj.Spec.Tolerations = nil

			SetObjectDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: SeedTaintProtected}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: SeedTaintProtected}}))
		})

		It("should add the 'protected' toleration if the namespace is 'garden' (w/ existing project tolerations)", func() {
			obj.Spec.Namespace = ptr.To(v1beta1constants.GardenNamespace)
			obj.Spec.Tolerations = &ProjectTolerations{
				Defaults:  []Toleration{{Key: "foo"}},
				Whitelist: []Toleration{{Key: "bar"}},
			}

			SetObjectDefaults_Project(obj)

			Expect(obj.Spec.Tolerations.Defaults).To(Equal([]Toleration{{Key: "foo"}, {Key: SeedTaintProtected}}))
			Expect(obj.Spec.Tolerations.Whitelist).To(Equal([]Toleration{{Key: "bar"}, {Key: SeedTaintProtected}}))
		})
	})

	Describe("api group defaulting", func() {
		DescribeTable(
			"should default the owner api groups",
			func(owner *rbacv1.Subject, kind string, expectedAPIGroup string) {
				if owner != nil {
					owner.Kind = kind
				}

				obj.Spec.Owner = owner

				SetObjectDefaults_Project(obj)

				if owner != nil {
					Expect(obj.Spec.Owner.APIGroup).To(Equal(expectedAPIGroup))
				} else {
					Expect(obj.Spec.Owner).To(BeNil())
				}
			},
			Entry("do nothing because owner is nil", nil, "", ""),
			Entry("kind serviceaccount", &rbacv1.Subject{}, rbacv1.ServiceAccountKind, ""),
			Entry("kind user", &rbacv1.Subject{}, rbacv1.UserKind, rbacv1.GroupName),
			Entry("kind group", &rbacv1.Subject{}, rbacv1.GroupKind, rbacv1.GroupName),
		)

		It("should default the api groups of members", func() {
			member1 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member1",
				},
				Roles: []string{"role"},
			}
			member2 := ProjectMember{
				Subject: rbacv1.Subject{
					Kind: rbacv1.ServiceAccountKind,
					Name: "member2",
				},
				Roles: []string{"role"},
			}
			member3 := ProjectMember{
				Subject: rbacv1.Subject{
					Kind: rbacv1.UserKind,
					Name: "member3",
				},
				Roles: []string{"role"},
			}
			member4 := ProjectMember{
				Subject: rbacv1.Subject{
					Kind: rbacv1.GroupKind,
					Name: "member4",
				},
				Roles: []string{"role"},
			}

			obj.Spec.Members = []ProjectMember{member1, member2, member3, member4}

			SetObjectDefaults_Project(obj)

			Expect(obj.Spec.Members[0].APIGroup).To(Equal(member1.APIGroup))
			Expect(obj.Spec.Members[1].APIGroup).To(BeEmpty())
			Expect(obj.Spec.Members[2].APIGroup).To(Equal(rbacv1.GroupName))
			Expect(obj.Spec.Members[3].APIGroup).To(Equal(rbacv1.GroupName))
		})
	})

	Describe("member defaulting", func() {
		It("should default the roles of members", func() {
			member1 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member1",
				},
			}
			member2 := ProjectMember{
				Subject: rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member2",
				},
			}

			obj.Spec.Members = []ProjectMember{member1, member2}

			SetObjectDefaults_Project(obj)

			for _, m := range obj.Spec.Members {
				Expect(m.Role).NotTo(BeEmpty())
				Expect(m.Role).To(Equal(ProjectMemberViewer))
			}
		})
	})
})
