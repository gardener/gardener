// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package customverbauthorizer_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/admission"
	servieaccount "k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/externalversions"
	. "github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"
	mockauthorizer "github.com/gardener/gardener/third_party/mock/apiserver/authorization/authorizer"
)

var _ = Describe("customverbauthorizer", func() {
	var auth *mockauthorizer.MockAuthorizer

	BeforeEach(func() {
		ctrl := gomock.NewController(GinkgoT())
		auth = mockauthorizer.NewMockAuthorizer(ctrl)
	})

	Describe("#Validate", func() {
		var (
			ctx = context.Background()

			attrs               admission.Attributes
			admissionHandler    *CustomVerbAuthorizer
			coreInformerFactory gardencoreinformers.SharedInformerFactory

			userInfo            = &user.DefaultInfo{Name: "foo"}
			authorizeAttributes authorizer.AttributesRecord
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.SetAuthorizer(auth)
			admissionHandler.AssignReadyFunc(func() bool { return true })
			coreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetCoreInformerFactory(coreInformerFactory)
		})

		Context("Projects", func() {
			var (
				project *core.Project
			)

			BeforeEach(func() {
				project = &core.Project{
					ObjectMeta: metav1.ObjectMeta{
						Name: "dummy",
					},
				}

				authorizeAttributes = authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "core.gardener.cloud",
					Namespace:       project.Namespace,
					Name:            project.Name,
					ResourceRequest: true,
				}

				authorizeAttributes.Resource = "projects"
			})

			It("should do nothing because the resource is not Project", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), project.Namespace, project.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("modify-spec-tolerations-whitelist verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbModifyProjectTolerationsWhitelist
				})

				It("should always allow creating a project without whitelist tolerations", func() {
					attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)
					})

					It("should allow creating a project with whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}

						attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow updating a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = append(project.Spec.Tolerations.Whitelist, core.Toleration{Key: "bar"})

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow removing a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = nil

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)
					})

					It("should forbid creating a project with whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}

						attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid updating a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = append(project.Spec.Tolerations.Whitelist, core.Toleration{Key: "bar"})

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid removing a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = nil

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})
				})
			})

			Context("manage-members verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbProjectManageMembers
				})

				var (
					owner = rbacv1.Subject{
						Kind: rbacv1.UserKind,
						Name: "owner",
					}
					projectMembersWithHumans = []core.ProjectMember{
						{
							Subject: owner,
						},
						{
							Subject: rbacv1.Subject{
								Kind: rbacv1.UserKind,
								Name: "foo",
							},
						},
						{
							Subject: rbacv1.Subject{
								Kind: rbacv1.GroupKind,
								Name: "bar",
							},
						},
					}
					projectMembersWithoutHumans = []core.ProjectMember{
						{
							Subject: rbacv1.Subject{
								Kind: rbacv1.ServiceAccountKind,
								Name: "foo",
							},
						},
						{
							Subject: rbacv1.Subject{
								Kind: rbacv1.UserKind,
								Name: servieaccount.ServiceAccountUsernamePrefix + "foo:bar",
							},
						},
					}
				)

				BeforeEach(func() {
					project.Spec.Owner = &owner
				})

				It("should always allow creating a project without members", func() {
					attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow creating a project with only owner as member", func() {
					project.Spec.Owner = &owner
					project.Spec.Members = []core.ProjectMember{{Subject: owner}}
					attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow adding non-human members to project", func() {
					project.Spec.Members = projectMembersWithHumans
					oldProject := project.DeepCopy()
					project.Spec.Members = append(projectMembersWithHumans, projectMembersWithoutHumans...)

					attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil).AnyTimes()
					})

					Context("CREATE", func() {
						It("should allow creating a project with human members if creator=owner", func() {
							project.Spec.Members = projectMembersWithHumans
							project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow creating a project without human members if creator=owner", func() {
							project.Spec.Members = projectMembersWithoutHumans
							project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow creating a project with owner plus additional human members", func() {
							project.Spec.Owner = &owner
							project.Spec.Members = append([]core.ProjectMember{{Subject: owner}}, projectMembersWithHumans...)
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})
					})

					Context("UPDATE", func() {
						It("should allow to add human users", func() {
							project.Spec.Members = projectMembersWithoutHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = append(projectMembersWithoutHumans, projectMembersWithHumans...)

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow to remove human users", func() {
							project.Spec.Members = projectMembersWithHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = projectMembersWithoutHumans

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil).AnyTimes()
					})

					Context("CREATE", func() {
						It("should allow creating a project without human members if owner=nil (meaning creator=owner)", func() {
							project.Spec.Owner = nil
							project.Spec.Members = projectMembersWithoutHumans
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow creating a project with owner plus additional human members if creator=owner", func() {
							project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
							project.Spec.Members = append([]core.ProjectMember{{Subject: owner}}, projectMembersWithHumans...)
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow creating a project with human members if owner=nil (meaning creator=owner)", func() {
							project.Spec.Owner = nil
							project.Spec.Members = projectMembersWithHumans
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should forbid creating a project with human members if creator!=owner", func() {
							project.Spec.Owner = &owner
							project.Spec.Members = projectMembersWithHumans
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
						})

						It("should forbid creating a project with owner plus additional human members if creator!=owner", func() {
							project.Spec.Owner = &owner
							project.Spec.Members = append([]core.ProjectMember{{Subject: owner}}, projectMembersWithHumans...)
							attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
						})
					})

					Context("UPDATE", func() {
						It("should allow to add human users (user=owner)", func() {
							project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
							project.Spec.Members = projectMembersWithoutHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = append(projectMembersWithoutHumans, projectMembersWithHumans...)

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should allow to remove human users (user=owner)", func() {
							project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
							project.Spec.Members = projectMembersWithHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = projectMembersWithoutHumans

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
						})

						It("should forbid to add human users (user!=owner)", func() {
							project.Spec.Owner = &owner
							project.Spec.Members = projectMembersWithoutHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = append(projectMembersWithoutHumans, projectMembersWithHumans...)

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
						})

						It("should forbid to remove human users (user!=owner)", func() {
							project.Spec.Owner = &owner
							project.Spec.Members = projectMembersWithHumans
							oldProject := project.DeepCopy()
							project.Spec.Members = projectMembersWithoutHumans

							attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
							Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
						})
					})
				})
			})

			Context("owner configuration", func() {
				Context("CREATE", func() {
					It("should allow setting the owner", func() {
						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
						attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				Context("UPDATE", func() {
					BeforeEach(func() {
						authorizeAttributes.Verb = CustomVerbProjectManageMembers
					})

					It("should succeed without owner change", func() {
						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
						oldProject := project.DeepCopy()

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow changing the owner for owner", func() {
						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: userInfo.Name}
						oldProject := project.DeepCopy()
						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: "new-owner"}

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow changing the owner for uam user", func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: "old-owner"}
						oldProject := project.DeepCopy()
						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: "new-owner"}

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should deny changing the owner", func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: "old-owner"}
						oldProject := project.DeepCopy()
						project.Spec.Owner.Name = "new-owner"

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("not allowed to manage owner")))
					})

					It("should deny unsetting the owner", func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

						project.Spec.Owner = &rbacv1.Subject{Kind: rbacv1.UserKind, Name: "owner"}
						oldProject := project.DeepCopy()
						project.Spec.Owner = nil

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(MatchError(ContainSubstring("not allowed to manage owner")))
					})
				})
			})
		})

		Context("NamespacedCloudProfiles", func() {
			var (
				parentCloudProfile     *v1beta1.CloudProfile
				namespacedCloudProfile *core.NamespacedCloudProfile
			)

			BeforeEach(func() {
				parentCloudProfile = &v1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name: "parent-cloud-profile",
					},
				}
				Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

				namespacedCloudProfile = &core.NamespacedCloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dummy",
						Namespace: "dummy-namespace",
					},
					Spec: core.NamespacedCloudProfileSpec{
						Parent: core.CloudProfileReference{
							Name: parentCloudProfile.Name,
							Kind: "CloudProfile",
						},
					},
				}

				authorizeAttributes = authorizer.AttributesRecord{
					User:            userInfo,
					APIGroup:        "core.gardener.cloud",
					Namespace:       namespacedCloudProfile.Namespace,
					Name:            namespacedCloudProfile.Name,
					ResourceRequest: true,
				}

				authorizeAttributes.Resource = "namespacedcloudprofiles"
			})

			It("should do nothing because the resource is not NamespacedCloudProfile", func() {
				attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Validate(ctx, attrs, nil)
				Expect(err).NotTo(HaveOccurred())
			})

			Context("modify-spec-kubernetes verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbNamespacedCloudProfileModifyKubernetes
				})

				It("should always allow creating a NamespacedCloudProfile without kubernetes settings", func() {
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)
					})

					It("should allow creating a NamespacedCloudProfile with kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow updating a NamespacedCloudProfile's kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(48 * time.Hour)})},
						}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow removing a NamespacedCloudProfile's kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.Kubernetes = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)
					})

					It("should forbid creating a NamespacedCloudProfile with kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid updating a NamespacedCloudProfile's kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(48 * time.Hour)})},
						}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid removing a NamespacedCloudProfile's kubernetes section", func() {
						namespacedCloudProfile.Spec.Kubernetes = &core.KubernetesSettings{Versions: []core.ExpirableVersion{
							{Version: "1.29.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})},
						}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.Kubernetes = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})
				})
			})

			Context("modify-spec-machineimages verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbNamespacedCloudProfileModifyMachineImages
				})

				It("should always allow creating a NamespacedCloudProfile without machineImages settings", func() {
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)
					})

					It("should allow creating a NamespacedCloudProfile with machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow updating a NamespacedCloudProfile's machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(48 * time.Hour)})}},
							}},
						}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow removing a NamespacedCloudProfile's machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.MachineImages = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)
					})

					It("should forbid creating a NamespacedCloudProfile with machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid updating a NamespacedCloudProfile's machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(48 * time.Hour)})}},
							}},
						}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid removing a NamespacedCloudProfile's machineImages section", func() {
						namespacedCloudProfile.Spec.MachineImages = []core.MachineImage{
							{Name: "dummy-image", Versions: []core.MachineImageVersion{
								{ExpirableVersion: core.ExpirableVersion{Version: "1.0.0", ExpirationDate: ptr.To(metav1.Time{Time: time.Now().Add(24 * time.Hour)})}},
							}},
						}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.MachineImages = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})
				})
			})

			Context("modify-spec-providerconfig verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbNamespacedCloudProfileModifyProviderConfig
				})

				It("should always allow creating a NamespacedCloudProfile without providerConfig settings", func() {
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)
					})

					It("should allow creating a NamespacedCloudProfile with providerConfig section", func() {
						namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte{}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})

					It("should allow removing a NamespacedCloudProfile's providerConfig section", func() {
						namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte{}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.ProviderConfig = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)
					})

					It("should forbid creating a NamespacedCloudProfile with providerConfig section", func() {
						namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte{}}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid removing a NamespacedCloudProfile's providerConfig section", func() {
						namespacedCloudProfile.Spec.ProviderConfig = &runtime.RawExtension{Raw: []byte{}}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.ProviderConfig = nil

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})
				})
			})

			Context("raise-spec-limits verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbNamespacedCloudProfileRaiseLimits
				})

				It("should always allow creating a NamespacedCloudProfile without limits", func() {
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow creating a NamespacedCloudProfile without limits.MaxNodesTotal", func() {
					namespacedCloudProfile.Spec.Limits = &core.Limits{}
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow creating a NamespacedCloudProfile with any limits.MaxNodesTotal if there is no limit in the parent CloudProfile", func() {
					namespacedCloudProfile.Spec.Limits = &core.Limits{
						MaxNodesTotal: ptr.To(int32(15)),
					}
					attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow removing a NamespacedCloudProfile's limits section", func() {
					namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(15))}
					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Spec.Limits = nil

					attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				It("should always allow decreasing a NamespacedCloudProfile's limits.maxNodesTotal to a lower or equal to value than in CloudProfile's limits", func() {
					parentCloudProfile.Spec.Limits = &v1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}
					Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())

					namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(15))}
					oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
					namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(10))}

					attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
					Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
				})

				When("permission is granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)

						parentCloudProfile.Spec.Limits = &v1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())
					})

					It("should allow creating a NamespacedCloudProfile with limits above parent CloudProfile limits", func() {
						namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(15))}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).To(Succeed())
					})
				})

				When("permission is not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)

						parentCloudProfile.Spec.Limits = &v1beta1.Limits{MaxNodesTotal: ptr.To(int32(10))}
						Expect(coreInformerFactory.Core().V1beta1().CloudProfiles().Informer().GetStore().Add(parentCloudProfile)).To(Succeed())
					})

					It("should forbid creating a NamespacedCloudProfile with a higher limits.maxNodesTotal value", func() {
						namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(15))}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, nil, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})

					It("should forbid modification of a NamespacedCloudProfile's limits.maxNodesTotal to a value still too high", func() {
						namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(15))}
						oldNamespacedCloudProfile := namespacedCloudProfile.DeepCopy()
						namespacedCloudProfile.Spec.Limits = &core.Limits{MaxNodesTotal: ptr.To(int32(13))}

						attrs = admission.NewAttributesRecord(namespacedCloudProfile, oldNamespacedCloudProfile, core.Kind("NamespacedCloudProfile").WithVersion("version"), namespacedCloudProfile.Namespace, namespacedCloudProfile.Name, core.Resource("namespacedcloudprofiles").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						Expect(admissionHandler.Validate(ctx, attrs, nil)).NotTo(Succeed())
					})
				})
			})
		})
	})

	Describe("#Register", func() {
		It("should register the plugin", func() {
			plugins := admission.NewPlugins()
			Register(plugins)

			registered := plugins.Registered()
			Expect(registered).To(HaveLen(1))
			Expect(registered).To(ContainElement("CustomVerbAuthorizer"))
		})
	})

	Describe("#NewFactory", func() {
		It("should create a new PluginFactory", func() {
			f, err := NewFactory(nil)

			Expect(f).NotTo(BeNil())
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("#New", func() {
		It("should only handle CREATE and UPDATE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should not return error", func() {
			cva, _ := New()
			cva.SetCoreInformerFactory(gardencoreinformers.NewSharedInformerFactory(nil, 0))
			Expect(cva.ValidateInitialization()).To(Succeed())
		})
	})
})
