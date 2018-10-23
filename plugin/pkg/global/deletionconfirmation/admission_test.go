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

package deletionconfirmation_test

import (
	. "github.com/gardener/gardener/plugin/pkg/global/deletionconfirmation"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/client-go/tools/cache"
)

var _ = Describe("deleteconfirmation", func() {
	Describe("#Admit", func() {
		var (
			shoot   garden.Shoot
			project garden.Project

			shootStore   cache.Store
			projectStore cache.Store

			attrs            admission.Attributes
			admissionHandler *DeletionConfirmation

			gardenInformerFactory gardeninformers.SharedInformerFactory
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			shootStore = gardenInformerFactory.Garden().InternalVersion().Shoots().Informer().GetStore()
			projectStore = gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore()

			shoot = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "dummy",
					Namespace: "dummy",
				},
			}
			project = garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummy",
				},
			}
		})

		It("should do nothing because the resource is not Shoot or Project", func() {
			attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Foo").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("foos").WithVersion("version"), "", admission.Delete, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		Context("Shoot resources", func() {
			It("should do nothing because the resource is already removed", func() {
				attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`shoot.garden.sapcloud.io "dummy" not found`))
			})

			Context("no annotation", func() {
				It("should reject for nil annotation field", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)

					shootStore.Add(&shoot)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should reject for false annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)

					shoot.Annotations = map[string]string{
						common.ConfirmationDeletion: "false",
					}
					shootStore.Add(&shoot)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should succeed for true annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Delete, false, nil)

					shoot.Annotations = map[string]string{
						common.ConfirmationDeletion: "true",
					}
					shootStore.Add(&shoot)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("Project resources", func() {
			It("should do nothing because the resource is already removed", func() {
				attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Project").WithVersion("version"), "", project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Delete, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal(`project.garden.sapcloud.io "dummy" not found`))
			})

			Context("no annotation", func() {
				It("should reject for nil annotation field", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Project").WithVersion("version"), "", project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Delete, false, nil)

					projectStore.Add(&project)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should reject for false annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Project").WithVersion("version"), "", project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Delete, false, nil)

					project.Annotations = map[string]string{
						common.ConfirmationDeletion: "false",
					}
					projectStore.Add(&project)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should succeed for true annotation value", func() {
					attrs = admission.NewAttributesRecord(nil, nil, garden.Kind("Project").WithVersion("version"), "", project.Name, garden.Resource("projects").WithVersion("version"), "", admission.Delete, false, nil)

					project.Annotations = map[string]string{
						common.ConfirmationDeletion: "true",
					}
					projectStore.Add(&project)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
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
		It("should only handle DELETE operations", func() {
			dr, err := New()

			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Update)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).To(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return error if no ShootLister or ProjectLister is set", func() {
			dr, _ := New()

			err := dr.ValidateInitialization()

			Expect(err).To(HaveOccurred())
		})

		It("should not return error if ShootLister and ProjectLister are set", func() {
			dr, _ := New()
			dr.SetInternalGardenInformerFactory(gardeninformers.NewSharedInformerFactory(nil, 0))

			err := dr.ValidateInitialization()

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
