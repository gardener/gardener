// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
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
							Roles: []string{core.ProjectMemberAdmin},
						},
						{
							Subject: rbacv1.Subject{
								APIGroup: "rbac.authorization.k8s.io",
								Kind:     rbacv1.UserKind,
								Name:     "bob.doe@example.com",
							},
							Roles: []string{core.ProjectMemberViewer, core.ProjectMemberUserAccessManager},
						},
					},
				},
			}
		})

		It("should not return any errors", func() {
			errorList := ValidateProject(project)

			Expect(errorList).To(BeEmpty())
		})

		DescribeTable("Project metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				project.ObjectMeta = objectMeta

				errorList := ValidateProject(project)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid Project with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Project with empty name",
				metav1.ObjectMeta{Name: ""},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should allow Project with '.' in the name",
				metav1.ObjectMeta{Name: "project.a"},
				BeEmpty(),
			),
			Entry("should forbid Project with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "project_a"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Project having too long names",
				metav1.ObjectMeta{Name: "project-name-too-long"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Project with namespace gardener-system-seed-lease",
				metav1.ObjectMeta{Name: "project-1", Namespace: "gardener-system-seed-lease"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("metadata.namespace"),
				}))),
			),
			Entry("should forbid Project with namespace gardener-system-shoot-issuer",
				metav1.ObjectMeta{Name: "project-1", Namespace: "gardener-system-shoot-issuer"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("metadata.namespace"),
				}))),
			),
			Entry("should forbid Project with namespace gardener-system-public",
				metav1.ObjectMeta{Name: "project-1", Namespace: "gardener-system-public"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("metadata.namespace"),
				}))),
			),
			Entry("should forbid Project with name containing two consecutive hyphens",
				metav1.ObjectMeta{Name: "in--valid"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid Project specification with empty or invalid key for description", func() {
			project.Spec.Description = ptr.To("")

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.description"),
			}))))
		})

		It("should forbid Project specification with empty or invalid key for purpose", func() {
			project.Spec.Purpose = ptr.To("")

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.purpose"),
			}))))
		})

		It("should forbid invalid service account usernames", func() {
			project.Spec.Members = append(project.Spec.Members,
				core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "system:serviceaccount:abcd",
					},
				},
			)

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.members[2].name"),
			}))))
		})

		It("should forbid duplicate members", func() {
			project.Spec.Members = append(project.Spec.Members,
				core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.UserKind,
						Name:     "system:serviceaccount:foo:bar",
					},
				},
				core.ProjectMember{
					Subject: rbacv1.Subject{
						Kind:      rbacv1.ServiceAccountKind,
						Name:      "bar",
						Namespace: "foo",
					},
				},
				core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.GroupKind,
						Name:     "baz",
					},
				},
				core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: rbacv1.GroupName,
						Kind:     rbacv1.GroupKind,
						Name:     "baz",
					},
				},
			)

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.members[3]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.members[5]"),
				})),
			))
		})

		It("should not allow duplicate in roles", func() {
			project.Spec.Members[0].Roles = []string{"admin", "admin"}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal("spec.members[0].roles[1]"),
			}))))
		})

		It("should not allow to use unknown roles without extension prefix", func() {
			project.Spec.Members[0].Roles = []string{"unknown-role"}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.members[0].roles[0]"),
			}))))
		})

		It("should prevent extension roles from being too long", func() {
			project.Spec.Members[0].Roles = []string{"extension:astringthatislongerthan15chars"}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeTooLong),
				"Field": Equal("spec.members[0].roles[0]"),
			}))))
		})

		It("should prevent extension roles from containing invalid characters", func() {
			project.Spec.Members[0].Roles = []string{"extension:/?as"}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.members[0].roles[0]"),
			}))))
		})

		It("should allow to use unknown roles with extension prefix", func() {
			project.Spec.Members[0].Roles = []string{"extension:unknown-role"}

			errorList := ValidateProject(project)

			Expect(errorList).To(BeEmpty())
		})

		It("should not allow using the owner role more than once", func() {
			project.Spec.Members[0].Roles = append(project.Spec.Members[0].Roles, core.ProjectMemberOwner)
			project.Spec.Members[1].Roles = append(project.Spec.Members[1].Roles, core.ProjectMemberOwner)

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.members[1].roles[2]"),
			}))))
		})

		DescribeTable("subject validation",
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
						Roles:   []string{core.ProjectMemberAdmin},
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

		It("should forbid invalid tolerations", func() {
			tolerations := []core.Toleration{
				{},
				{Key: "foo"},
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "baz"},
				{Key: "baz", Value: ptr.To("baz")},
			}
			project.Spec.Tolerations = &core.ProjectTolerations{
				Defaults:  tolerations,
				Whitelist: tolerations,
			}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.tolerations.defaults[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.defaults[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.defaults[4]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.defaults[6]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.tolerations.whitelist[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.whitelist[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.whitelist[4]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations.whitelist[6]"),
				})),
			))
		})

		It("should forbid using a default toleration which is not in the whitelist", func() {
			project.Spec.Tolerations = &core.ProjectTolerations{
				Defaults: []core.Toleration{{Key: "foo"}},
			}

			errorList := ValidateProject(project)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.tolerations.defaults[0]"),
				})),
			))
		})

		Context("dual approval for deletion config", func() {
			It("should forbid empty resources", func() {
				project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion, core.DualApprovalForDeletion{})

				Expect(ValidateProject(project)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.dualApprovalForDeletion[0].resource"),
					})),
				))
			})

			It("should forbid unsupported resources", func() {
				project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion, core.DualApprovalForDeletion{
					Resource: "foos",
				})

				Expect(ValidateProject(project)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.dualApprovalForDeletion[0].resource"),
					})),
				))
			})

			It("should forbid duplicate resources", func() {
				project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion,
					core.DualApprovalForDeletion{Resource: "shoots"},
					core.DualApprovalForDeletion{Resource: "shoots"},
				)

				Expect(ValidateProject(project)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.dualApprovalForDeletion[1].resource"),
					})),
				))
			})

			It("should forbid invalid label selectors", func() {
				project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion, core.DualApprovalForDeletion{
					Resource: "shoots",
					Selector: metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
				})

				Expect(ValidateProject(project)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.dualApprovalForDeletion[0].selector.matchLabels"),
					})),
				))
			})

			It("should allow valid configurations", func() {
				project.Spec.DualApprovalForDeletion = append(project.Spec.DualApprovalForDeletion, core.DualApprovalForDeletion{
					Resource:               "shoots",
					Selector:               metav1.LabelSelector{MatchLabels: map[string]string{}},
					IncludeServiceAccounts: ptr.To(false),
				})

				Expect(ValidateProject(project)).To(BeEmpty())
			})
		})

		DescribeTable("namespace immutability",
			func(old, new *string, matcher gomegatypes.GomegaMatcher) {
				project.Spec.Namespace = old
				newProject := prepareProjectForUpdate(project)
				newProject.Spec.Namespace = new

				errList := ValidateProjectUpdate(newProject, project)

				Expect(errList).To(matcher)
			},

			Entry("namespace change w/ preset namespace", ptr.To("garden-dev"), ptr.To("garden-core"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.namespace"),
			})))),
			Entry("namespace change w/o preset namespace", nil, ptr.To("garden-core"), BeEmpty()),
			Entry("no change (both unset)", nil, nil, BeEmpty()),
			Entry("no change (same value)", ptr.To("garden-dev"), ptr.To("garden-dev"), BeEmpty()),
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
