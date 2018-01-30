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

package seedprotector_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/shoot/seedprotector"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("seedprotector", func() {
	Describe("#Admit", func() {
		var (
			admissonHandler       *Protector
			gardenInformerFactory gardeninformers.SharedInformerFactory
			seed                  garden.Seed
			shoot                 garden.Shoot

			seedName = "seed-1"
			falseVar = false
			trueVar  = true

			seedBase = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Protected: &falseVar,
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Seed: &seedName,
					},
				},
			}
		)

		BeforeEach(func() {
			admissonHandler, _ = New()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissonHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			seed = seedBase
			shoot = shootBase
		})

		It("should pass because no Seed is specified in shoot.Spec.Cloud.Seed", func() {
			shoot.Spec.Cloud.Seed = nil

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should pass because the Seed specified in shoot manifest is not protected and shoot is not in garden namespace", func() {
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should pass because shoot is not in garden namespace and seed is not protected", func() {
			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should fail because shoot is not in garden namespace and seed is protected", func() {
			seed.Spec.Protected = &trueVar

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})

		It("should pass because shoot is in garden namespace and seed is protected", func() {
			shoot.Namespace = "garden"
			seed.Spec.Protected = &trueVar

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).ToNot(HaveOccurred())
		})

		It("should pass because shoot is in garden namespace and seed is not protected", func() {
			shoot.Namespace = "garden"

			gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissonHandler.Admit(attrs)

			Expect(err).ToNot(HaveOccurred())
		})
	})
})
