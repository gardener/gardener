// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package validation_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("Project Validation Tests", func() {
	Describe("#ValidateProject, #ValidateProjectUpdate", func() {
		var project *core.Project

		BeforeEach(func() {
			project = &core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "project-1",
				},
				Spec: core.ProjectSpec{
					CreatedBy: &rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     "john.doe@example.com",
					},
					Owner: &rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     rbacv1.UserKind,
						Name:     "john.doe@example.com",
					},
					Members: []core.ProjectMember{
						{
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     rbacv1.UserKind,
								Name:     "alice.doe@example.com",
							},
							Role: core.ProjectMemberAdmin,
						},
						{
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     rbacv1.UserKind,
								Name:     "bob.doe@example.com",
							},
							Role: core.ProjectMemberViewer,
						},
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateProject(project)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid Project resources with empty metadata", func() {
			project.ObjectMeta = metav1.ObjectMeta{}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Projects having too long names", func() {
			project.ObjectMeta.Name = "project-name-too-long"

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeTooLong),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Projects having two consecutive hyphens", func() {
			project.ObjectMeta.Name = "in--valid"

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))))
		})

		It("should forbid Project specification with empty or invalid keys for description/purpose", func() {
			project.Spec.Description = makeStringPointer("")
			project.Spec.Purpose = makeStringPointer("")

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.description"),
			})), PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.purpose"),
			}))))
		})

		DescribeTable("owner validation",
			func(apiGroup, kind, name, namespace string, expectType field.ErrorType, field string) {
				subject := rbacv1.Subject{
					APIGroup:  apiGroup,
					Kind:      kind,
					Name:      name,
					Namespace: namespace,
				}

				project.Spec.Owner = &subject
				project.Spec.CreatedBy = &subject
				project.Spec.Members = []core.ProjectMember{
					{
						Subject: subject,
						Role:    core.ProjectMemberAdmin,
					},
				}

				errList := ValidateProject(project)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.owner.%s", field)),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.createdBy.%s", field)),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(expectType),
					"Field": Equal(fmt.Sprintf("spec.members[0].%s", field)),
				}))))
			},

			// general
			Entry("empty name", "rbac.authorization.k8s.io", rbacv1.UserKind, "", "", field.ErrorTypeRequired, "name"),
			Entry("unknown kind", "rbac.authorization.k8s.io", "unknown", "foo", "", field.ErrorTypeNotSupported, "kind"),

			// serviceaccounts
			Entry("invalid api group name", "apps/v1beta1", rbacv1.ServiceAccountKind, "foo", "default", field.ErrorTypeNotSupported, "apiGroup"),
			Entry("invalid name", "", rbacv1.ServiceAccountKind, "foo-", "default", field.ErrorTypeInvalid, "name"),
			Entry("no namespace", "", rbacv1.ServiceAccountKind, "foo", "", field.ErrorTypeRequired, "namespace"),

			// users
			Entry("invalid api group name", "rbac.authorization.invalid", rbacv1.UserKind, "john.doe@example.com", "", field.ErrorTypeNotSupported, "apiGroup"),

			// groups
			Entry("invalid api group name", "rbac.authorization.invalid", rbacv1.GroupKind, "groupname", "", field.ErrorTypeNotSupported, "apiGroup"),
		)

		DescribeTable("namespace immutability",
			func(old, new *string, matcher gomegatypes.GomegaMatcher) {
				project.Spec.Namespace = old
				newProject := prepareProjectForUpdate(project)
				newProject.Spec.Namespace = new

				errList := ValidateProjectUpdate(newProject, project)

				Expect(errList).To(matcher)
			},

			Entry("namespace change w/  preset namespace", makeStringPointer("garden-dev"), makeStringPointer("garden-core"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.namespace"),
			})))),
			Entry("namespace change w/o preset namespace", nil, makeStringPointer("garden-core"), BeEmpty()),
			Entry("no change (both unset)", nil, nil, BeEmpty()),
			Entry("no change (same value)", makeStringPointer("garden-dev"), makeStringPointer("garden-dev"), BeEmpty()),
		)

		It("should forbid Project updates trying to change the createdBy field", func() {
			newProject := prepareProjectForUpdate(project)
			newProject.Spec.CreatedBy.Name = "some-other-user"

			errorList := ValidateProjectUpdate(newProject, project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.createdBy"),
			}))))
		})

		It("should forbid Project updates trying to reset the owner field", func() {
			newProject := prepareProjectForUpdate(project)
			newProject.Spec.Owner = nil

			errorList := ValidateProjectUpdate(newProject, project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.owner"),
			}))))
		})
	})
})

func prepareProjectForUpdate(project *core.Project) *core.Project {
	p := project.DeepCopy()
	p.ResourceVersion = "1"
	return p
}
