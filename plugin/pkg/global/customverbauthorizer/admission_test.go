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

package customverbauthorizer_test

import (
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	mockauthorizer "github.com/gardener/gardener/pkg/mock/apiserver/authorization/authorizer"
	. "github.com/gardener/gardener/plugin/pkg/global/customverbauthorizer"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

var _ = Describe("customverbauthorizer", func() {
	var (
		ctrl *gomock.Controller
		auth *mockauthorizer.MockAuthorizer
	)

	BeforeEach(func() {
		ctrl = gomock.NewController(GinkgoT())
		auth = mockauthorizer.NewMockAuthorizer(ctrl)
	})

	Describe("#Validate", func() {
		var (
			ctx     = context.TODO()
			project *core.Project

			attrs            admission.Attributes
			admissionHandler *CustomVerbAuthorizer

			userInfo            = &user.DefaultInfo{Name: "foo"}
			authorizeAttributes authorizer.AttributesRecord
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.SetAuthorizer(auth)

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
		})

		It("should do nothing because the resource is not Project", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Foo").WithVersion("version"), project.Namespace, project.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Validate(context.TODO(), attrs, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("Projects", func() {
			BeforeEach(func() {
				authorizeAttributes.Resource = "projects"
			})

			Context("modify-spec-tolerations-whitelist verb", func() {
				BeforeEach(func() {
					authorizeAttributes.Verb = CustomVerbModifyProjectTolerationsWhitelist
				})

				It("should always allow creating a project without whitelist tolerations", func() {
					attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
					err := admissionHandler.Validate(context.TODO(), attrs, nil)
					Expect(err).NotTo(HaveOccurred())
				})

				Describe("permissions granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionAllow, "", nil)
					})

					It("should allow creating a project with whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}

						attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should allow updating a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = append(project.Spec.Tolerations.Whitelist, core.Toleration{Key: "bar"})

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).NotTo(HaveOccurred())
					})

					It("should allow removing a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = nil

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).NotTo(HaveOccurred())
					})
				})

				Describe("permissions not granted", func() {
					BeforeEach(func() {
						auth.EXPECT().Authorize(ctx, authorizeAttributes).Return(authorizer.DecisionDeny, "", nil)
					})

					It("should forbid creating a project with whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}

						attrs = admission.NewAttributesRecord(project, nil, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).To(HaveOccurred())
					})

					It("should forbid updating a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = append(project.Spec.Tolerations.Whitelist, core.Toleration{Key: "bar"})

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).To(HaveOccurred())
					})

					It("should forbid removing a project's whitelist tolerations", func() {
						project.Spec.Tolerations = &core.ProjectTolerations{Whitelist: []core.Toleration{{Key: "foo"}}}
						oldProject := project.DeepCopy()
						project.Spec.Tolerations.Whitelist = nil

						attrs = admission.NewAttributesRecord(project, oldProject, core.Kind("Project").WithVersion("version"), project.Namespace, project.Name, core.Resource("projects").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, userInfo)
						err := admissionHandler.Validate(context.TODO(), attrs, nil)
						Expect(err).To(HaveOccurred())
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
			Expect(registered).To(ContainElement(PluginName))
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
		It("should not return error if", func() {
			cva, _ := New()
			Expect(cva.ValidateInitialization()).To(Succeed())
		})
	})
})
