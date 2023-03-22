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

package exposureclass_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/utils/pointer"

	"github.com/gardener/gardener/pkg/apis/core"
	gardencoreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/exposureclass"
)

var _ = Describe("exposureclass", func() {
	Describe("#Admit", func() {
		var (
			exposureClassName = "test"

			shoot         *core.Shoot
			exposureClass *core.ExposureClass

			attrs            admission.Attributes
			admissionHandler *ExposureClass

			gardenCoreInformerFactory gardencoreinformers.SharedInformerFactory
		)

		BeforeEach(func() {
			gardenCoreInformerFactory = gardencoreinformers.NewSharedInformerFactory(nil, 0)

			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			admissionHandler.SetInternalCoreInformerFactory(gardenCoreInformerFactory)

			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: core.ShootSpec{
					ExposureClassName: &exposureClassName,
				},
			}

			exposureClass = &core.ExposureClass{
				ObjectMeta: metav1.ObjectMeta{
					Name: exposureClassName,
				},
				Scheduling: &core.ExposureClassScheduling{},
			}
		})

		It("should do nothing because the resource is not Shoot", func() {
			attrs = admission.NewAttributesRecord(nil, nil, core.Kind("Test").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("foos").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should do nothing as Shoot has no ExposureClass referenced", func() {
			shoot.Spec.ExposureClassName = nil

			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should fail as referenced ExposureClass was not found", func() {
			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(HaveOccurred())
		})

		It("should do nothing as referenced ExposureClass has no scheduling settings", func() {
			exposureClass.Scheduling = nil
			Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

			attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
		})

		Context("SeedSelector", func() {
			BeforeEach(func() {
				exposureClass.Scheduling.SeedSelector = &core.SeedSelector{}

				shoot.Spec.SeedSelector = &core.SeedSelector{}
			})

			It("should do nothing as ExposureClass has no seed selector", func() {
				exposureClass.Scheduling.SeedSelector = nil
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should unite the matching labels of seed selector from Shoot and ExposureClass", func() {
				shoot.Spec.SeedSelector.MatchLabels = map[string]string{"abc": "abc"}
				exposureClass.Scheduling.SeedSelector.LabelSelector.MatchLabels = map[string]string{"xyz": "xyz"}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SeedSelector.MatchLabels).To(Equal(map[string]string{
					"abc": "abc",
					"xyz": "xyz",
				}))
			})

			It("should fail as seed selector from Shoot and ExposureClass have conflicting labels", func() {
				shoot.Spec.SeedSelector.MatchLabels = map[string]string{"abc": "abc"}
				exposureClass.Scheduling.SeedSelector.LabelSelector.MatchLabels = map[string]string{"abc": "xyz"}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should unite the seed selector expressions from Shoot and Exposureclass", func() {
				shoot.Spec.SeedSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
					Key:      "abc",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"abc", "def"},
				}}
				exposureClass.Scheduling.SeedSelector.LabelSelector.MatchExpressions = []metav1.LabelSelectorRequirement{{
					Key:      "abc",
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{"xyz"},
				}}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SeedSelector.MatchExpressions).To(Equal([]metav1.LabelSelectorRequirement{
					{
						Key:      "abc",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"abc", "def"},
					},
					{
						Key:      "abc",
						Operator: metav1.LabelSelectorOpIn,
						Values:   []string{"xyz"},
					},
				}))
			})

			It("should unite the seedselector provider types from Shoot and ExposureClass", func() {
				shoot.Spec.SeedSelector.ProviderTypes = []string{"aws", "gcp"}
				exposureClass.Scheduling.SeedSelector.ProviderTypes = []string{"gcp"}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.SeedSelector.ProviderTypes).To(Equal([]string{"aws", "gcp"}))
			})
		})

		Context("Tolerations", func() {
			BeforeEach(func() {
				exposureClass.Scheduling.Tolerations = []core.Toleration{{
					Key:   "abc",
					Value: pointer.String("def"),
				}}

				shoot.Spec.Tolerations = []core.Toleration{{
					Key: "xyz",
				}}
			})

			It("should do nothing as ExposureClass has no tolerations", func() {
				exposureClass.Scheduling.Tolerations = []core.Toleration{}
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{{
					Key: "xyz",
				}}))
			})

			It("should unite the tolerations from Shoot and ExposureClass", func() {
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.Tolerations).To(Equal([]core.Toleration{
					{
						Key: "xyz",
					},
					{
						Key:   "abc",
						Value: pointer.String("def"),
					},
				}))
			})

			It("should fail as Shoot and ExposureClass tolerations have conflicting keys", func() {
				shoot.Spec.Tolerations[0].Key = "abc"
				Expect(gardenCoreInformerFactory.Core().InternalVersion().ExposureClasses().Informer().GetStore().Add(exposureClass)).To(Succeed())

				attrs = admission.NewAttributesRecord(shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)
				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
