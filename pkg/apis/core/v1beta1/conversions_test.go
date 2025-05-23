// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package v1beta1_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/v1beta1"
)

var _ = Describe("Conversion", func() {
	var scheme *runtime.Scheme

	BeforeEach(func() {
		scheme = runtime.NewScheme()
		Expect(SchemeBuilder.AddToScheme(scheme)).ToNot(HaveOccurred())
	})

	Describe("Project conversion", func() {
		Describe("#Convert_v1alpha1_ProjectSpec_To_core_ProjectSpec", func() {
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
				member4 = rbacv1.Subject{
					APIGroup: "group",
					Kind:     "kind",
					Name:     "member4",
				}

				extensionRole = "extension:role"

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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
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
							Role:    ProjectMemberOwner,
						},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
						Members: []core.ProjectMember{
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
						{
							Subject: member3,
							Roles:   []string{ProjectMemberOwner, extensionRole, ProjectMemberOwner},
						},
						{
							Subject: member4,
							Role:    ProjectMemberOwner,
						},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.Project{
					Spec: core.ProjectSpec{
						Owner: &owner,
						Members: []core.ProjectMember{
							{
								Subject: member1,
								Roles:   nil,
							},
							{
								Subject: owner,
								Roles:   []string{core.ProjectMemberOwner},
							},
							{
								Subject: member2,
								Roles:   nil,
							},
							{
								Subject: member3,
								Roles:   []string{extensionRole},
							},
							{
								Subject: member4,
							},
						},
					},
				}))
			})
		})

		Describe("#Convert_core_ProjectSpec_To_v1alpha1_ProjectSpec", func() {
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
				ownerRole     = ProjectMemberOwner
				extensionRole = "extension:role"

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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
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
						{Subject: owner, Roles: []string{"foo"}},
						{Subject: member2},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Role:    "foo",
								Roles:   []string{ProjectMemberOwner},
							},
							{Subject: member2},
						},
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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Role:    ProjectMemberOwner,
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

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{Subject: member1},
							{
								Subject: owner,
								Role:    ownerRole,
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
						{
							Subject: member3,
							Roles:   []string{core.ProjectMemberOwner, extensionRole, core.ProjectMemberOwner},
						},
					},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&Project{
					Spec: ProjectSpec{
						Owner: &owner,
						Members: []ProjectMember{
							{
								Subject: member1,
								Roles:   nil,
							},
							{
								Subject: owner,
								Role:    ProjectMemberOwner,
							},
							{
								Subject: member2,
								Roles:   nil,
							},
							{
								Subject: member3,
								Role:    extensionRole,
							},
						},
					},
				}))
			})
		})

		Describe("#Convert_v1alpha1_ProjectMember_To_core_ProjectMember", func() {
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
				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.ProjectMember{}))
			})

			It("should do nothing because role was found", func() {
				in = &ProjectMember{
					Role:  role,
					Roles: []string{role, "bar"},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar"},
				}))
			})

			It("should reorder the roles list to make sure the role is at the head", func() {
				in = &ProjectMember{
					Role:  role,
					Roles: []string{"bar", role},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar"},
				}))
			})

			It("should reorder the roles list to make sure the role is at the head even if there are duplicates", func() {
				in = &ProjectMember{
					Role:  role,
					Roles: []string{"bar", role, role, role, "hugo"},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar", "hugo"},
				}))
			})

			It("should add the role to the head of roles list", func() {
				in = &ProjectMember{
					Role:  role,
					Roles: []string{"bar"},
				}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&core.ProjectMember{
					Roles: []string{role, "bar"},
				}))
			})
		})

		Describe("#Convert_core_ProjectMember_To_v1alpha1_ProjectMember", func() {
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
				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&ProjectMember{}))
			})

			It("should add the first role to the role field and remove it from the list", func() {
				in.Roles = []string{role, "bar"}

				Expect(scheme.Convert(in, out, nil)).To(Succeed())
				Expect(out).To(Equal(&ProjectMember{
					Role:  role,
					Roles: []string{"bar"},
				}))
			})
		})
	})

	Describe("InternalSecret conversion", func() {
		var (
			in  *InternalSecret
			out *core.InternalSecret
		)

		BeforeEach(func() {
			in = &InternalSecret{}
			out = &core.InternalSecret{}
		})

		It("should merge data and stringData", func() {
			in.Data = map[string][]byte{"foo": []byte("bla"), "baz": []byte("bing")}
			in.StringData = map[string]string{"bar": "blub"}

			Expect(scheme.Convert(in, out, nil)).NotTo(HaveOccurred())
			Expect(out.Data).To(And(
				HaveKeyWithValue("foo", BeEquivalentTo("bla")),
				HaveKeyWithValue("baz", BeEquivalentTo("bing")),
				HaveKeyWithValue("bar", BeEquivalentTo("blub")),
			))
		})

		It("should overwrite data with stringData", func() {
			in.Data = map[string][]byte{"foo": []byte("bla"), "baz": []byte("bing")}
			in.StringData = map[string]string{"foo": "boo", "bar": "blub"}

			Expect(scheme.Convert(in, out, nil)).NotTo(HaveOccurred())
			Expect(out.Data).To(And(
				HaveKeyWithValue("foo", BeEquivalentTo("boo")),
				HaveKeyWithValue("baz", BeEquivalentTo("bing")),
				HaveKeyWithValue("bar", BeEquivalentTo("blub")),
			))
		})
	})

	Describe("ControllerDeployment conversion", func() {
		Describe("convert from v1beta1 to internal", func() {
			var (
				in  *ControllerDeployment
				out *core.ControllerDeployment
			)

			BeforeEach(func() {
				in = &ControllerDeployment{}
				out = &core.ControllerDeployment{}
			})

			Context("helm type", func() {
				BeforeEach(func() {
					in.Type = "helm"
					in.ProviderConfig = runtime.RawExtension{
						Raw: []byte(`{"chart":"Zm9v","values":{"foo":["bar","baz"]}}`),
					}
				})

				It("should convert legacy helm deployment to new structure", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(BeEmpty(), "type is empty for non-custom type")
					Expect(out.ProviderConfig).To(BeNil(), "providerConfig is empty for non-custom type")
					Expect(out.Helm).To(Equal(&core.HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}))
				})

				It("should correctly handle empty providerConfig", func() {
					in.ProviderConfig.Raw = nil

					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Helm).To(Equal(&core.HelmControllerDeployment{
						RawChart: nil,
						Values:   nil,
					}))
				})
			})

			Context("custom type", func() {
				BeforeEach(func() {
					in.Type = "custom"
					in.ProviderConfig = runtime.RawExtension{
						Raw: []byte(`{"foo":"bar"}`),
					}
				})

				It("should keep type and providerConfig", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(Equal("custom"))
					Expect(out.ProviderConfig).To(Equal(&runtime.Unknown{
						ContentType: "application/json",
						Raw:         []byte(`{"foo":"bar"}`),
					}))
					Expect(out.Helm).To(BeNil())
				})
			})
		})

		Describe("convert from internal to v1beta1", func() {
			var (
				in  *core.ControllerDeployment
				out *ControllerDeployment
			)

			BeforeEach(func() {
				in = &core.ControllerDeployment{}
				out = &ControllerDeployment{}
			})

			Context("helm type with rawChart", func() {
				BeforeEach(func() {
					in.Helm = &core.HelmControllerDeployment{
						RawChart: []byte("foo"),
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}
				})

				It("should convert new helm deployment to legacy structure", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(Equal("helm"))
					Expect(out.ProviderConfig).To(Equal(runtime.RawExtension{
						Raw: []byte(`{"chart":"Zm9v","values":{"foo":["bar","baz"]}}`),
					}))
				})
			})

			Context("helm type with ociRepository", func() {
				BeforeEach(func() {
					in.Helm = &core.HelmControllerDeployment{
						OCIRepository: &core.OCIRepository{
							Repository: ptr.To("url"),
							Tag:        ptr.To("1.0.0"),
							Digest:     ptr.To("sha256:foo"),
						},
						Values: &apiextensionsv1.JSON{
							Raw: []byte(`{"foo":["bar","baz"]}`),
						},
					}
				})

				It("should convert new helm deployment to legacy structure", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(Equal("helm"))
					Expect(out.ProviderConfig).To(Equal(runtime.RawExtension{
						Raw: []byte(`{"values":{"foo":["bar","baz"]},"ociRepository":{"repository":"url","tag":"1.0.0","digest":"sha256:foo"}}`),
					}))
				})
			})

			Context("custom type", func() {
				BeforeEach(func() {
					in.Type = "custom"
					in.ProviderConfig = &runtime.Unknown{
						ContentType: "application/json",
						Raw:         []byte(`{"foo":"bar"}`),
					}
				})

				It("should keep type and providerConfig", func() {
					Expect(scheme.Convert(in, out, nil)).To(Succeed())

					Expect(out.Type).To(Equal("custom"))
					Expect(out.ProviderConfig).To(Equal(runtime.RawExtension{
						Raw: []byte(`{"foo":"bar"}`),
					}))
				})
			})
		})
	})
})
