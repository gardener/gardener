// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/plugin/pkg/project/validator"
)

var _ = Describe("Admission", func() {
	Describe("#Admit", func() {
		var (
			err              error
			project          core.Project
			admissionHandler admission.ValidationInterface

			namespaceName = "garden-my-project"
			projectName   = "my-project"
			projectBase   = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name:      projectName,
					Namespace: namespaceName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, err = New()
			Expect(err).NotTo(HaveOccurred())

			project = projectBase
		})

		It("should allow creating the project (namespace nil)", func() {
			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})

		It("should allow creating the project(namespace non-nil)", func() {
			project.Spec.Namespace = &namespaceName

			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})

		It("should allow creating the project (namespace is 'garden')", func() {
			project.Spec.Namespace = pointer.String(v1beta1constants.GardenNamespace)

			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(Succeed())
		})

		It("should prevent creating the project because namespace prefix is missing", func() {
			project.Spec.Namespace = pointer.String("foo")

			attrs := admission.NewAttributesRecord(&project, nil, core.Kind("Project").WithVersion("version"), "", project.Name, core.Resource("projects").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			Expect(admissionHandler.Validate(context.TODO(), attrs, nil)).To(MatchError(ContainSubstring(".spec.namespace must start with garden-")))
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

	Describe("#New", func() {
		It("should only handle CREATE operations", func() {
			dr, err := New()
			Expect(err).ToNot(HaveOccurred())
			Expect(dr.Handles(admission.Create)).To(BeTrue())
			Expect(dr.Handles(admission.Update)).To(BeFalse())
			Expect(dr.Handles(admission.Connect)).To(BeFalse())
			Expect(dr.Handles(admission.Delete)).To(BeFalse())
		})
	})
})
