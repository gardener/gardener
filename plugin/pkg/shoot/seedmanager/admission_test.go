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

package seedmanager_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/seedmanager"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("seedmanager", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *SeedManager
			gardenInformerFactory gardeninformers.SharedInformerFactory
			seed                  garden.Seed
			shoot                 garden.Shoot

			cloudProfileName = "cloudprofile-1"
			seedName         = "seed-1"
			region           = "europe"

			falseVar = false
			trueVar  = true

			seedBase = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: cloudProfileName,
						Region:  region,
					},
					Visible:   &trueVar,
					Protected: &falseVar,
				},
				Status: garden.SeedStatus{
					Conditions: []garden.Condition{
						{
							Type:   garden.SeedAvailable,
							Status: corev1.ConditionTrue,
						},
					},
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: cloudProfileName,
						Region:  region,
					},
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			seed = seedBase
			shoot = shootBase
		})

		Context("Shoot references a Seed - protection", func() {
			BeforeEach(func() {
				shoot.Spec.Cloud.Seed = &seedName
			})

			It("should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should pass because shoot is not in garden namespace and seed is not protected", func() {
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should fail because shoot is not in garden namespace and seed is protected", func() {
				seed.Spec.Protected = &trueVar

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should pass because shoot is in garden namespace and seed is protected", func() {
				shoot.Namespace = "garden"
				seed.Spec.Protected = &trueVar

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).ToNot(HaveOccurred())
			})

			It("should pass because shoot is in garden namespace and seed is not protected", func() {
				shoot.Namespace = "garden"

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("Shoot does not reference a Seed - find an adequate one", func() {
			BeforeEach(func() {
				shoot.Spec.Cloud.Seed = nil
			})

			It("should find a seed cluster referencing the same profile and region and indicating availability", func() {
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.Cloud.Seed).To(Equal(seedName))
			})

			It("should fail because it cannot find a seed cluster due to invalid region", func() {
				shoot.Spec.Cloud.Region = "another-region"

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
				Expect(shoot.Spec.Cloud.Seed).To(BeNil())
			})

			It("should fail because it cannot find a seed cluster due to invalid profile", func() {
				shoot.Spec.Cloud.Profile = "another-profile"

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
				Expect(shoot.Spec.Cloud.Seed).To(BeNil())
			})

			It("should fail because it cannot find a seed cluster due to unavailability", func() {
				seed.Status.Conditions = []garden.Condition{
					{
						Type:   garden.SeedAvailable,
						Status: corev1.ConditionFalse,
					},
				}

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
				Expect(shoot.Spec.Cloud.Seed).To(BeNil())
			})

			It("should fail because it cannot find a seed cluster due to invisibility", func() {
				seed.Spec.Visible = &falseVar

				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
				Expect(shoot.Spec.Cloud.Seed).To(BeNil())
			})
		})
	})
})
