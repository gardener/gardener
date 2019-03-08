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

package validator_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"

	. "github.com/gardener/gardener/plugin/pkg/plant/validator"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("validator", func() {
	Describe("#Admit", func() {
		var (
			plant                 core.Plant
			project               garden.Project
			attrs                 admission.Attributes
			admissionHandler      *ValidatePlant
			coreClient            *fake.Clientset
			gardenInformerFactory gardeninformers.SharedInformerFactory

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

			coreClient = &fake.Clientset{}
			admissionHandler.SetInternalCoreClientset(coreClient)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			project = projectBase
			plant = core.Plant{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummyPlant",
				},
				Spec: core.PlantSpec{
					SecretRef: corev1.SecretReference{
						Name:      "test",
						Namespace: "test",
					},
				},
			}
		})
		It("should reject Plant resources references same kubeconfig secret", func() {
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
					SecretRef: corev1.SecretReference{
						Name:      "test",
						Namespace: "test",
					},
				},
			}
			err := gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			Expect(err).NotTo(HaveOccurred())

			coreClient.AddReactor("list", "plants", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &core.PlantList{
					Items: []core.Plant{existingPlant},
				}, nil
			})

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Create, false, nil)
			err = admissionHandler.Validate(attrs)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("another plant resource already exists"))
		})

		It("should reject Plant resources not fulfilling length constraints", func() {
			tooLongName := "too-long-namespace"
			project.ObjectMeta = metav1.ObjectMeta{
				Name: tooLongName,
			}
			plant.ObjectMeta = metav1.ObjectMeta{
				Name:      "too-long-name",
				Namespace: namespaceName,
			}

			err := gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
			Expect(err).NotTo(HaveOccurred())

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)
			err = admissionHandler.Validate(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsBadRequest(err)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("name must not exceed"))

		})
		It("should do nothing because the resource is not a Plant", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("SomeOtherResource").WithVersion("version"), "", plant.Name, core.Resource("some-other-resource").WithVersion("version"), "", admission.Create, false, nil)
			err := admissionHandler.Validate(attrs)

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

			coreClient.AddReactor("list", "plants", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &core.PlantList{
					Items: []core.Plant{},
				}, nil
			})

			err = admissionHandler.Validate(attrs)
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
