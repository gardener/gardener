// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package plant_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	"k8s.io/apiserver/pkg/authentication/user"

	. "github.com/gardener/gardener/plugin/pkg/plant"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Admission", func() {
	Describe("#Admit", func() {
		var (
			plant                     core.Plant
			project                   garden.Project
			attrs                     admission.Attributes
			admissionHandler          *AdmitPlant
			gardenInformerFactory     gardeninformers.SharedInformerFactory
			gardencoreInformerFactory gardencoreinformers.SharedInformerFactory

			namespaceName = "garden-my-project"
			projectName   = "my-project"
			projectBase   = garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespaceName,
				},
				Spec: garden.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			gardencoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)

			admissionHandler.SetInternalCoreInformerFactory(gardencoreInformerFactory)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			project = projectBase

			plant = core.Plant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummyPlant",
				},
				Spec: core.PlantSpec{
					SecretRef: corev1.LocalObjectReference{
						Name: "test",
					},
				},
			}
		})
		It("should add the created-by annotation", func() {
			var (
				defaultUserName = "test-user"
				defaultUserInfo = &user.DefaultInfo{Name: defaultUserName}
			)

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, garden.Resource("plants").WithVersion("version"), "", admission.Create, false, defaultUserInfo)

			Expect(plant.Annotations).NotTo(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))

			err := admissionHandler.Admit(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(plant.Annotations).To(HaveKeyWithValue(common.GardenCreatedBy, defaultUserName))
		})
		It("should reject Plant resources referencing same kubeconfig secret", func() {
			project.ObjectMeta = metav1.ObjectMeta{
				Namespace: namespaceName,
			}
			plant.Namespace = namespaceName
			existingPlant := core.Plant{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "existingPlant",
					Namespace: namespaceName,
				},
				Spec: core.PlantSpec{
					SecretRef: corev1.LocalObjectReference{
						Name: "test",
					},
				},
			}
			err := gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			Expect(err).NotTo(HaveOccurred())

			err = gardencoreInformerFactory.Core().InternalVersion().Plants().Informer().GetStore().Add(&existingPlant)
			Expect(err).NotTo(HaveOccurred())

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Create, false, nil)
			err = admissionHandler.Validate(attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("another plant resource already exists"))
		})
		It("should do nothing because the resource is not a Plant", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("SomeOtherResource").WithVersion("version"), "", plant.Name, core.Resource("some-other-resource").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Validate(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})
		It("should not deny the object because it is updated", func() {
			project.ObjectMeta = metav1.ObjectMeta{
				Namespace: namespaceName,
			}
			plant.Namespace = namespaceName
			plant.Spec.SecretRef.Name = "secretref"
			err := gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			Expect(err).NotTo(HaveOccurred())
			attrs = admission.NewAttributesRecord(&plant, &plant, core.Kind("Plant").WithVersion("version"), "", plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Update, false, nil)

			err = gardencoreInformerFactory.Core().InternalVersion().Plants().Informer().GetStore().Add(&core.Plant{})
			Expect(err).NotTo(HaveOccurred())

			err = admissionHandler.Validate(attrs, nil)
			Expect(err).NotTo(HaveOccurred())
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
		It("should only handle CREATE or UPDATE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeTrue())
			Expect(dr.Handles(admission.Connect)).NotTo(BeTrue())
			Expect(dr.Handles(admission.Delete)).NotTo(BeTrue())
		})
	})

	Describe("#ValidateInitialization", func() {
		It("should return no error", func() {
			dr, _ := New()

			err := dr.ValidateInitialization()

			Expect(err).NotTo(HaveOccurred())
		})
	})
})
