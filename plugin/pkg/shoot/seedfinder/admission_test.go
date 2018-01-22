// Copyright 2018 The Gardener Authors.
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

package seedfinder_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/seedfinder"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("seedfinder", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *Finder
			gardenInformerFactory gardeninformers.SharedInformerFactory
			seed                  garden.Seed
			shoot                 garden.Shoot

			cloudProfileName = "cloudprofile-1"
			seedName         = "seed-1"
			region           = "europe"

			seedBase = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: cloudProfileName,
						Region:  region,
					},
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
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			seed = seedBase
			shoot = shootBase
		})

		It("should do nothing because the shoot already references a seed", func() {
			shoot.Spec.Cloud.Seed = &seedName

			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shoot.Spec.Cloud.Seed).To(Equal(seedName))
		})

		It("should find a seed cluster referencing the same profile and region and indicating availability", func() {
			shoot.Spec.Cloud.Seed = nil

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shoot.Spec.Cloud.Seed).To(Equal(seedName))
		})

		It("should fail because it cannot find a seed cluster due to invalid region", func() {
			shoot.Spec.Cloud.Seed = nil
			shoot.Spec.Cloud.Region = "another-region"

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
			Expect(shoot.Spec.Cloud.Seed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to invalid profile", func() {
			shoot.Spec.Cloud.Seed = nil
			shoot.Spec.Cloud.Profile = "another-profile"

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
			Expect(shoot.Spec.Cloud.Seed).To(BeNil())
		})

		It("should fail because it cannot find a seed cluster due to unavailability", func() {
			shoot.Spec.Cloud.Seed = nil
			seed.Status.Conditions = []garden.Condition{
				{
					Type:   garden.SeedAvailable,
					Status: corev1.ConditionFalse,
				},
			}

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
			Expect(shoot.Spec.Cloud.Seed).To(BeNil())
		})
	})
})
