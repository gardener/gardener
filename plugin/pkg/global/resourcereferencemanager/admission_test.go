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

package resourcereferencemanager_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	. "github.com/gardener/gardener/plugin/pkg/global/resourcereferencemanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("resourcereferencemanager", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *ReferenceManager
			kubeInformerFactory   kubeinformers.SharedInformerFactory
			gardenInformerFactory gardeninformers.SharedInformerFactory

			shoot garden.Shoot

			namespace        = "default"
			cloudProfileName = "profile-1"
			seedName         = "seed-1"
			bindingName      = "binding-1"
			quotaName        = "quota-1"
			secretName       = "secret-1"
			shootName        = "shoot-1"

			secret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretName,
					Namespace: namespace,
				},
			}

			cloudProfile = garden.CloudProfile{
				ObjectMeta: metav1.ObjectMeta{
					Name: cloudProfileName,
				},
			}
			seed = garden.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
				Spec: garden.SeedSpec{
					Cloud: garden.SeedCloud{
						Profile: cloudProfileName,
					},
					SecretRef: corev1.ObjectReference{
						Name:      secretName,
						Namespace: namespace,
					},
				},
			}
			quota = garden.Quota{
				ObjectMeta: metav1.ObjectMeta{
					Name:      quotaName,
					Namespace: namespace,
				},
			}
			privateSecretBinding = garden.PrivateSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: namespace,
				},
				SecretRef: corev1.LocalObjectReference{
					Name: secretName,
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName,
						Namespace: namespace,
					},
				},
			}
			crossSecretBinding = garden.CrossSecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      bindingName,
					Namespace: namespace,
				},
				SecretRef: corev1.ObjectReference{
					Name:      secretName,
					Namespace: namespace,
				},
				Quotas: []corev1.ObjectReference{
					{
						Name:      quotaName,
						Namespace: namespace,
					},
				},
			}
			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						Profile: cloudProfileName,
						Seed:    &seedName,
						SecretBindingRef: corev1.ObjectReference{
							Kind: "PrivateSecretBinding",
							Name: bindingName,
						},
					},
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			shoot = shootBase
		})

		Context("tests for PrivateSecretBinding objects", func() {
			It("should accept because all referenced objects have been found", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				attrs := admission.NewAttributesRecord(&privateSecretBinding, nil, garden.Kind("PrivateSecretBinding").WithVersion("version"), privateSecretBinding.Namespace, privateSecretBinding.Name, garden.Resource("privatesecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				attrs := admission.NewAttributesRecord(&privateSecretBinding, nil, garden.Kind("PrivateSecretBinding").WithVersion("version"), privateSecretBinding.Namespace, privateSecretBinding.Name, garden.Resource("privatesecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because one of the referenced quotas does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				attrs := admission.NewAttributesRecord(&privateSecretBinding, nil, garden.Kind("PrivateSecretBinding").WithVersion("version"), privateSecretBinding.Namespace, privateSecretBinding.Name, garden.Resource("privatesecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for CrossSecretBinding objects", func() {
			It("should accept because all referenced objects have been found", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				attrs := admission.NewAttributesRecord(&crossSecretBinding, nil, garden.Kind("CrossSecretBinding").WithVersion("version"), crossSecretBinding.Namespace, crossSecretBinding.Name, garden.Resource("crosssecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().Quotas().Informer().GetStore().Add(&quota)

				attrs := admission.NewAttributesRecord(&crossSecretBinding, nil, garden.Kind("CrossSecretBinding").WithVersion("version"), crossSecretBinding.Namespace, crossSecretBinding.Name, garden.Resource("crosssecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because one of the referenced quotas does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				attrs := admission.NewAttributesRecord(&crossSecretBinding, nil, garden.Kind("CrossSecretBinding").WithVersion("version"), crossSecretBinding.Namespace, crossSecretBinding.Name, garden.Resource("crosssecretbindings").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Seed objects", func() {
			It("should accept because all referenced objects have been found", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&secret)

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)

				attrs := admission.NewAttributesRecord(&seed, nil, garden.Kind("Seed").WithVersion("version"), "", seed.Name, garden.Resource("seeds").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})

		Context("tests for Shoot objects", func() {
			It("should accept because all referenced objects have been found", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().PrivateSecretBindings().Informer().GetStore().Add(&privateSecretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject because the referenced cloud profile does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				gardenInformerFactory.Garden().InternalVersion().PrivateSecretBindings().Informer().GetStore().Add(&privateSecretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced seed does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().PrivateSecretBindings().Informer().GetStore().Add(&privateSecretBinding)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced private secret binding does not exist", func() {
				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced cross secret binding does not exist", func() {
				shoot.Spec.Cloud.SecretBindingRef = corev1.ObjectReference{
					Kind: "CrossSecretBinding",
					Name: bindingName,
				}

				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because the referenced secret binding kind is unknown", func() {
				shoot.Spec.Cloud.SecretBindingRef = corev1.ObjectReference{
					Kind: "doesnotexist",
				}

				gardenInformerFactory.Garden().InternalVersion().CloudProfiles().Informer().GetStore().Add(&cloudProfile)
				gardenInformerFactory.Garden().InternalVersion().Seeds().Informer().GetStore().Add(&seed)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
			})
		})
	})
})
