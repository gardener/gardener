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

package quotavalidator_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/gardener/gardener/plugin/pkg/shoot/quotavalidator"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
)

var _ = Describe("quotavalidator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *RejectShootIfQuotaExceeded
			gardenInformerFactory gardeninformers.SharedInformerFactory
			shoot                 garden.Shoot
			crossSB               garden.CrossSecretBinding
			quota                 garden.Quota
			cloudProfile          garden.CloudProfile

			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-shoot",
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: "profile",
						SecretBindingRef: corev1.ObjectReference{
							Kind: "CrossSecretBinding",
							Name: "test-crossSB",
						},
						GCP: &garden.GCPCloud{},
					},
				},
			}
			cloudProfileBase = garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: "profile",
				},
				Spec: garden.CloudProfileSpec{
					GCP: &garden.GCPProfile{
						Constraints: garden.GCPConstraints{
							MachineTypes: []garden.MachineType{
								{
									Name:   "n1-standard-2",
									CPUs:   2,
									GPUs:   0,
									Memory: resource.MustParse("7500Mi"),
								},
							},
							VolumeTypes: []garden.VolumeType{
								{
									Name:  "pd-standard",
									Class: "standard",
								},
							},
						},
					},
				},
			}
			crossSBBase = garden.CrossSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-namespace",
					Name:      "test-crossSB",
				},
				SecretRef: garden.CrossReference{
					Namespace: "test-trial-namespace",
					Name:      "test-secret",
				},
			}
			quotaBase = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
			}
		)

		BeforeSuite(func() {
			logger.Logger = logger.NewLogger("")
		})

		BeforeEach(func() {
			shoot = shootBase
			cloudProfile = cloudProfileBase
			crossSB = crossSBBase
			quota = quotaBase

			admissionHandler, _ = New()
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)
		})

		It("should allow: empty shoot (without CrossSecretBinding)", func() {
			shoot = garden.Shoot{}
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow: shoot with private secret binding", func() {
			// set shoot spec
			shoot.Spec.Cloud.SecretBindingRef.Kind = "PrivateSecretBinding"
			shoot.Spec.Cloud.SecretBindingRef.Name = "test-privateSB"

			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow: shoot with cross secret binding without quotas", func() {
			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should bring error: shoot with cross secret binding having quota without namespace", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Name: "test-quota",
				},
			}

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInternalError(err)).To(BeTrue())
		})

		It("should bring error: shoot with cross secret binding having quota that doesn't exist", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "not-existing",
					Name:      "test-quota",
				},
			}

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInternalError(err)).To(BeTrue())
		})

		It("should bring error: shoot with cross secret binding having quota with invalid scope", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
			}
			quota.Spec.Scope = "invalid"

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInternalError(err)).To(BeTrue())
		})

		It("should bring error: shoot with cross secret binding having quotas from different namespaces", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
				{
					Namespace: "other-namespace",
					Name:      "test-quota",
				},
			}
			quota2 := quotaBase
			quota2.ObjectMeta.Namespace = "other-namespace"
			quota2.Spec.Scope = "secret"
			quota.Spec.Scope = "secret"

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota2)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsInternalError(err)).To(BeTrue())
		})

		It("should allow: shoot with cross secret binding having quota with no metrics", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
			}
			quota.Spec.Scope = "secret"

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow: shoot with cross secret binding having quota not exceeding limits", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
			}
			// set high enough quota
			quota.Spec.Scope = "secret"
			quota.Spec.Metrics = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("4"),
			}
			// set worker at shoot
			shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
				{
					Worker: garden.Worker{
						Name:          "test-worker",
						MachineType:   "n1-standard-2",
						AutoScalerMax: 2,
					},
					VolumeType: "pd-standard",
					VolumeSize: "20Gi",
				},
			}

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		It("should bring error: shoot with cross secret binding having quota exceeding limits", func() {
			// set cross SB
			crossSB.Quotas = []garden.CrossReference{
				{
					Namespace: "test-trial-namespace",
					Name:      "test-quota",
				},
			}
			// set high enough quota
			quota.Spec.Scope = "secret"
			quota.Spec.Metrics = corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("1"),
			}
			// set worker at shoot
			shoot.Spec.Cloud.GCP.Workers = []garden.GCPWorker{
				{
					Worker: garden.Worker{
						Name:          "test-worker",
						MachineType:   "n1-standard-2",
						AutoScalerMax: 2,
					},
					VolumeType: "pd-standard",
					VolumeSize: "20Gi",
				},
			}

			gardenInformerFactory.Garden().InternalVersion().CrossSecretBindings().Informer().GetStore().Add(&crossSB)
			gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)
			gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(HaveOccurred())
			Expect(apierrors.IsForbidden(err)).To(BeTrue())
		})
	})
})
