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

package resources_test

import (
	"github.com/gardener/gardener/pkg/apis/core"
	"github.com/gardener/gardener/pkg/client/core/clientset/internalversion/fake"
	. "github.com/gardener/gardener/plugin/pkg/controllerregistration/resources"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("resources", func() {
	Describe("#Admit", func() {
		var (
			controllerRegistration core.ControllerRegistration

			attrs            admission.Attributes
			admissionHandler *Resources

			coreClient *fake.Clientset

			resourceKind = "Foo"
			resourceType = "bar"
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })

			coreClient = &fake.Clientset{}
			admissionHandler.SetInternalCoreClientset(coreClient)

			controllerRegistration = core.ControllerRegistration{
				ObjectMeta: metav1.ObjectMeta{
					Name: "dummy",
				},
				Spec: core.ControllerRegistrationSpec{
					Resources: []core.ControllerResource{
						{
							Kind: resourceKind,
							Type: resourceType,
						},
					},
				},
			}
		})

		It("should do nothing because the resource is not ControllerRegistration", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("SomeOtherResource").WithVersion("version"), "", controllerRegistration.Name, core.Resource("some-other-resource").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Validate(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow the object because no other resource in the system uses the kind/type combination", func() {
			attrs = admission.NewAttributesRecord(&controllerRegistration, nil, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Validate(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should not deny the object because it is updated", func() {
			attrs = admission.NewAttributesRecord(&controllerRegistration, &controllerRegistration, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Update, false, nil)

			coreClient.AddReactor("list", "controllerregistrations", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &core.ControllerRegistrationList{
					Items: []core.ControllerRegistration{controllerRegistration},
				}, nil
			})

			err := admissionHandler.Validate(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should deny the object because another resource in the system uses the kind/type combination", func() {
			attrs = admission.NewAttributesRecord(&controllerRegistration, nil, core.Kind("ControllerRegistration").WithVersion("version"), "", controllerRegistration.Name, core.Resource("controllerregistrations").WithVersion("version"), "", admission.Create, false, nil)

			controllerRegistration2 := controllerRegistration.DeepCopy()
			controllerRegistration2.Name = "another-name"

			coreClient.AddReactor("list", "controllerregistrations", func(action testing.Action) (bool, runtime.Object, error) {
				return true, &core.ControllerRegistrationList{
					Items: []core.ControllerRegistration{*controllerRegistration2},
				}, nil
			})

			err := admissionHandler.Validate(attrs, nil)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
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
