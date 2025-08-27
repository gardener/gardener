// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package mutator_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/plugin/pkg/project/mutator"
)

var _ = Describe("Admission", func() {
	Describe("#Admit", func() {
		var (
			err              error
			project          core.Project
			admissionHandler admission.MutationInterface
			attrs            admission.Attributes

			projectName = "my-project"
			projectBase = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
			}

			userInfo user.Info
		)

		BeforeEach(func() {
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())

			project = projectBase

			userInfo = &user.DefaultInfo{Name: "foo"}
		})

		When("project is created", func() {
			BeforeEach(func() {
				attrs = admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
			})

			It("should maintain createdBy and project owner", func() {
				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

				Expect(project.Spec.CreatedBy).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "User",
					Name:     userInfo.GetName(),
				}))

				Expect(project.Spec.Owner).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "User",
					Name:     userInfo.GetName(),
				}))

				Expect(project.Spec.Members).To(ConsistOf(core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     userInfo.GetName(),
					},
					Roles: []string{
						core.ProjectMemberAdmin,
						core.ProjectMemberOwner,
					},
				}))
			})

			It("should not overwrite project owner", func() {
				project.Spec.Owner = &rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "User",
					Name:     "bar",
				}

				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

				Expect(project.Spec.Owner).To(Equal(&rbacv1.Subject{
					APIGroup: "rbac.authorization.k8s.io",
					Kind:     "User",
					Name:     "bar",
				}))
			})
		})

		When("project is updated", func() {
			BeforeEach(func() {
				attrs = admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
			})

			It("should add project owner to members", func() {
				projectOwner := core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "foo",
					},
					Roles: []string{
						core.ProjectMemberAdmin,
						core.ProjectMemberOwner,
					},
				}

				projectMemberBar := core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "bar",
					},
					Roles: []string{
						core.ProjectMemberViewer,
					},
				}

				project.Spec.Owner = &projectOwner.Subject
				project.Spec.Members = []core.ProjectMember{projectMemberBar}

				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

				Expect(project.Spec.Members).To(ConsistOf(projectMemberBar, projectOwner))
			})

			It("should not re-add owner as member", func() {
				projectOwner := core.ProjectMember{
					Subject: rbacv1.Subject{
						APIGroup: "rbac.authorization.k8s.io",
						Kind:     "User",
						Name:     "foo",
					},
					Roles: []string{
						core.ProjectMemberAdmin,
						core.ProjectMemberOwner,
					},
				}

				project.Spec.Owner = &projectOwner.Subject
				project.Spec.Members = []core.ProjectMember{projectOwner}

				Expect(admissionHandler.Admit(context.TODO(), attrs, nil)).To(Succeed())

				Expect(project.Spec.Members).To(ConsistOf(projectOwner))
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("ProjectMutator"))
		})
	})

	Describe("#New", func() {
		It("should handle CREATE and UPDATE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).To(BeFalse())
			Expect(dr.Handles(admission.Delete)).To(BeFalse())
		})
	})
})
