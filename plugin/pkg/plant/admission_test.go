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
	"context"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/plant"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
)

var _ = Describe("Admission", func() {
	Describe("#Admit", func() {
		var (
			plant               core.Plant
			project             core.Project
			attrs               admission.Attributes
			admissionHandler    *Handler
			coreInformerFactory coreinformers.SharedInformerFactory

			namespaceName = "garden-my-project"
			projectName   = "my-project"
			projectBase   = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespaceName,
				},
				Spec: core.ProjectSpec{
					Namespace: &namespaceName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

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

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, defaultUserInfo)

			Expect(plant.Annotations).NotTo(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, defaultUserName))

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(plant.Annotations).To(HaveKeyWithValue(v1beta1constants.GardenCreatedBy, defaultUserName))
		})

		It("should reject Plant resources referencing same kubeconfig secret", func() {
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

			Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())

			Expect(coreInformerFactory.Core().InternalVersion().Plants().Informer().GetStore().Add(&existingPlant)).To(Succeed())

			attrs := admission.NewAttributesRecord(&plant, nil, core.Kind("Plant").WithVersion("version"), plant.Namespace, plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("another plant resource already exists"))
		})

		It("should do nothing because the resource is not a Plant", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("SomeOtherResource").WithVersion("version"), "", plant.Name, core.Resource("some-other-resource").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Validate(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should not deny the object because it is updated", func() {
			plant.Namespace = namespaceName
			plant.Spec.SecretRef.Name = "secretref"

			Expect(coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)).To(Succeed())
			attrs = admission.NewAttributesRecord(&plant, &plant, core.Kind("Plant").WithVersion("version"), "", plant.Name, core.Resource("plants").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			Expect(coreInformerFactory.Core().InternalVersion().Plants().Informer().GetStore().Add(&core.Plant{})).To(Succeed())

			err := admissionHandler.Validate(context.TODO(), attrs, nil)
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
