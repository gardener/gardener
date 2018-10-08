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

package dnshostedzone_test

import (
	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/awsbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/gcpbotanist"
	"github.com/gardener/gardener/pkg/operation/cloudbotanist/openstackbotanist"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/dnshostedzone"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("quotavalidator", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *DNSHostedZone
			kubeInformerFactory   kubeinformers.SharedInformerFactory
			gardenInformerFactory gardeninformers.SharedInformerFactory
			shoot                 garden.Shoot

			cloudProviderSecretName = "my-secret"
			referencedSecretName    = "my-dns-secret"
			secretBindingName       = "my-secret-binding"
			namespace               = "my-namespace"

			cloudProviderSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cloudProviderSecretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}
			secretBinding = garden.SecretBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secretBindingName,
					Namespace: namespace,
				},
				SecretRef: corev1.SecretReference{
					Name:      cloudProviderSecretName,
					Namespace: namespace,
				},
			}
			referencedSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      referencedSecretName,
					Namespace: namespace,
				},
				Data: map[string][]byte{},
			}

			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: namespace,
				},
				Spec: garden.ShootSpec{
					Cloud: garden.Cloud{
						SecretBindingRef: corev1.LocalObjectReference{
							Name: secretBindingName,
						},
					},
				},
			}

			hostedZoneID = "1234"
			domain       = "example.com"

			defaultDomainSecret = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: common.GardenNamespace,
					Labels: map[string]string{
						common.GardenRole: common.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						common.DNSDomain:       domain,
						common.DNSHostedZoneID: hostedZoneID,
						common.DNSProvider:     string(garden.DNSAWSRoute53),
					},
				},
				Data: map[string][]byte{},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

			shootBase.Spec.DNS.Domain = nil
			shootBase.Spec.DNS.HostedZoneID = nil
			shootBase.Spec.DNS.Provider = garden.DNSUnmanaged
			shoot = shootBase
			cloudProviderSecret.Data = map[string][]byte{}
		})

		It("should do nothing because the shoot specifies the 'unmanaged' dns provider", func() {
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs)

			Expect(err).NotTo(HaveOccurred())
		})

		Context("hosted zone id has been specified", func() {
			Context("domain is a default domain", func() {
				It("should reject because the shoot specifies an invalid hosted zone id", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("invalid-id")
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot." + domain)

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should pass because the shoot specifies the correct hosted zone id", func() {
					shoot.Spec.DNS.HostedZoneID = &hostedZoneID
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot." + domain)

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})
			})

			Context("domain is not a default domain", func() {
				It("should reject because the cloud provider secret does not contain valid dns provider credentials (AWS)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should reject because the cloud provider secret does not contain valid dns provider credentials (GCP)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSGoogleCloudDNS
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should reject because the cloud provider secret does not contain valid dns provider credentials (Designate)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSOpenstackDesignate
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should pass because the cloud provider secret does contain valid dns provider credentials (AWS)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")
					cloudProviderSecret.Data = map[string][]byte{
						awsbotanist.AccessKeyID:     nil,
						awsbotanist.SecretAccessKey: nil,
					}

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass because the cloud provider secret does contain valid dns provider credentials (GCP)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSGoogleCloudDNS
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")
					cloudProviderSecret.Data = map[string][]byte{
						gcpbotanist.ServiceAccountJSON: nil,
					}

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should pass because the cloud provider secret does contain valid dns provider credentials (Designate)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSOpenstackDesignate
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")
					cloudProviderSecret.Data = map[string][]byte{
						openstackbotanist.AuthURL:        nil,
						openstackbotanist.DomainName:     nil,
						openstackbotanist.TenantName:     nil,
						openstackbotanist.UserName:       nil,
						openstackbotanist.UserDomainName: nil,
						openstackbotanist.Password:       nil,
					}

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&cloudProviderSecret)
					gardenInformerFactory.Garden().InternalVersion().SecretBindings().Informer().GetStore().Add(&secretBinding)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})

				It("should reject because the referenced secret does not contain valid dns provider credentials", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")
					shoot.Spec.DNS.SecretName = makeStringPointer(referencedSecretName)

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&referencedSecret)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).To(HaveOccurred())
					Expect(apierrors.IsForbidden(err)).To(BeTrue())
				})

				It("should pass because the referenced secret does contain valid dns provider credentials (AWS)", func() {
					shoot.Spec.DNS.HostedZoneID = makeStringPointer("abcd")
					shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
					shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")
					shoot.Spec.DNS.SecretName = makeStringPointer(referencedSecretName)
					referencedSecret.Data = map[string][]byte{
						awsbotanist.AccessKeyID:     nil,
						awsbotanist.SecretAccessKey: nil,
					}

					kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&referencedSecret)
					attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

					err := admissionHandler.Admit(attrs)

					Expect(err).NotTo(HaveOccurred())
				})
			})
		})

		Context("hosted zone id has not been specified", func() {
			It("should reject because no default domain secrets found", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
				shoot.Spec.DNS.Domain = makeStringPointer("my-shoot." + domain)

				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should reject because no default domain matches the specified domain", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
				shoot.Spec.DNS.Domain = makeStringPointer("my-shoot.my-domain.com")

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).To(HaveOccurred())
				Expect(apierrors.IsForbidden(err)).To(BeTrue())
			})

			It("should pass because a default domain has been found", func() {
				shoot.Spec.DNS.Provider = garden.DNSAWSRoute53
				shoot.Spec.DNS.Domain = makeStringPointer("my-shoot." + domain)

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.HostedZoneID).NotTo(BeNil())
				Expect(*shoot.Spec.DNS.HostedZoneID).To(Equal(hostedZoneID))
			})
		})
	})
})

func makeStringPointer(s string) *string {
	c := s
	return &c
}
