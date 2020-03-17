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

package dns_test

import (
	"context"
	"fmt"
	"net/http"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	coreinformers "github.com/gardener/gardener/pkg/client/core/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/utils/pointer"
)

var _ = Describe("dns", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler    *DNS
			kubeInformerFactory kubeinformers.SharedInformerFactory
			coreInformerFactory coreinformers.SharedInformerFactory

			seed  core.Seed
			shoot core.Shoot

			namespace   = "my-namespace"
			projectName = "my-project"
			seedName    = "my-seed"
			shootName   = "shoot"

			domain   = "example.com"
			provider = core.DNSUnmanaged

			project = core.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: core.ProjectSpec{
					Namespace: &namespace,
				},
			}

			defaultDomainProvider = "my-dns-provider"
			defaultDomainSecret   = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: v1beta1constants.GardenNamespace,
					Labels: map[string]string{
						v1beta1constants.GardenRole: common.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						common.DNSDomain:   domain,
						common.DNSProvider: defaultDomainProvider,
					},
				},
			}

			seedBase = core.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Name: seedName,
				},
			}

			shootBase = core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: core.ShootSpec{
					DNS:      &core.DNS{},
					SeedName: &seedName,
				},
			}
		)

		BeforeEach(func() {
			admissionHandler, _ = New()
			admissionHandler.AssignReadyFunc(func() bool { return true })
			kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
			coreInformerFactory = coreinformers.NewSharedInformerFactory(nil, 0)
			admissionHandler.SetInternalCoreInformerFactory(coreInformerFactory)

			shootBase.Spec.DNS.Domain = nil
			shootBase.Spec.DNS.Providers = []core.DNSProvider{
				{
					Type: &provider,
				},
			}
			shoot = shootBase
			seed = seedBase
		})

		It("should do nothing because the shoot status is updated", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "status", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (create)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the shoot does not specify a seed (update)", func() {
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.SeedName = nil
			shootBefore := shootCopy.DeepCopy()

			attrs := admission.NewAttributesRecord(shootCopy, shootCopy, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should do nothing because the seed disables DNS", func() {
			seedCopy := seed.DeepCopy()
			seedCopy.Spec.Taints = append(seedCopy.Spec.Taints, core.SeedTaint{Key: core.SeedTaintDisableDNS})
			shootCopy := shoot.DeepCopy()
			shootCopy.Spec.DNS = nil
			shootBefore := shootCopy.DeepCopy()

			coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(seedCopy)
			attrs := admission.NewAttributesRecord(shootCopy, nil, core.Kind("Shoot").WithVersion("version"), shootCopy.Namespace, shootCopy.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(*shootCopy).To(Equal(*shootBefore))
		})

		It("should throw an error because the seed disables DNS but shoot specifies a dns section", func() {
			seedCopy := seed.DeepCopy()
			seedCopy.Spec.Taints = append(seedCopy.Spec.Taints, core.SeedTaint{Key: core.SeedTaintDisableDNS})

			coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(seedCopy)
			attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).To(MatchError(apierrors.NewBadRequest("shoot's .spec.dns section must be nil if seed with disabled DNS is chosen")))
		})

		It("should set the 'unmanaged' dns provider as the primary one", func() {
			shootBefore := shoot.DeepCopy()
			shootBefore.Spec.DNS.Providers[0].Primary = pointer.BoolPtr(true)

			coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
			attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

			err := admissionHandler.Admit(context.TODO(), attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).To(Equal(*shootBefore))
		})

		Context("provider is not 'unmanaged'", func() {
			var (
				providerType = "provider"
				secretName   = "secret"
			)

			BeforeEach(func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Providers = nil
			})

			It("should pass because no default domain was generated for the shoot (with domain)", func() {
				var (
					shootDomain  = "my-shoot.my-private-domain.com"
					providerType = "provider"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
				}

				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":    Equal(pointer.StringPtr(providerType)),
					"Primary": Equal(pointer.BoolPtr(true)),
				})))
			})

			It("should set the correct primary DNS provider", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type":    Equal(pointer.StringPtr(providerType)),
						"Primary": Equal(pointer.BoolPtr(true)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":       Equal(pointer.StringPtr(providerType)),
						"Primary":    BeNil(),
						"SecretName": Equal(pointer.StringPtr(secretName)),
					}),
				))
			})

			It("should re-set the correct primary DNS provider on updates", func() {
				var (
					shootDomain = "my-shoot.my-private-domain.com"
				)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: &providerType,
					},
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.DNS.Providers[1].Primary = pointer.BoolPtr(true)

				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(
					MatchFields(IgnoreExtras, Fields{
						"Type": Equal(pointer.StringPtr(providerType)),
					}),
					MatchFields(IgnoreExtras, Fields{
						"Type":       Equal(pointer.StringPtr(providerType)),
						"Primary":    Equal(pointer.BoolPtr(true)),
						"SecretName": Equal(pointer.StringPtr(secretName)),
					}),
				))
			})

			It("should pass because a default domain was generated for the shoot (no domain)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should not set a primary provider because a default domain was generated for the shoot (no domain)", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
					},
				}

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
				Expect(shoot.Spec.DNS.Providers).To(ConsistOf(MatchFields(IgnoreExtras, Fields{
					"Type":       Equal(pointer.StringPtr(providerType)),
					"Primary":    BeNil(),
					"SecretName": Equal(pointer.StringPtr(secretName)),
				})))
			})

			It("should forbid setting a primary provider because a default domain was generated for the shoot (no domain)", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
						Primary:    pointer.BoolPtr(true),
					},
				}

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code":    Equal(int32(http.StatusBadRequest)),
						"Message": Equal("primary dns provider must not be set when default domain is used"),
					}),
				})))
			})

			It("should forbid setting a primary provider because a default domain was manually configured for the shoot", func() {
				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       &providerType,
						SecretName: &secretName,
						Primary:    pointer.BoolPtr(true),
					},
				}

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(PointTo(MatchFields(IgnoreExtras, Fields{
					"ErrStatus": MatchFields(IgnoreExtras, Fields{
						"Code":    Equal(int32(http.StatusBadRequest)),
						"Message": Equal("primary dns provider must not be set when default domain is used"),
					}),
				})))
			})

			It("should pass because the default domain was allowed for the shoot (with domain)", func() {
				shootDomain := fmt.Sprintf("%s.%s.%s", shoot.Name, project.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Providers).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
			})

			It("should reject because a default domain was already used for the shoot but is invalid (with domain)", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should reject because a default domain was already used for the shoot but is invalid (with domain) when seed is assigned", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, oldShoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(HaveOccurred())
			})

			It("should not reject shoots using a non compliant default domain on updates", func() {
				shootDomain := fmt.Sprintf("%s.other-project.%s", shoot.Name, domain)
				shoot.Spec.DNS.Domain = &shootDomain

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, &shoot, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Update, &metav1.UpdateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(Not(HaveOccurred()))
			})

			It("should reject because no domain was configured for the shoot and project is missing", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(apierrors.NewInternalError(fmt.Errorf("no project found for namespace %q", shoot.Namespace))))
			})

			It("should reject because no domain was configured for the shoot and default domain secret is missing", func() {
				coreInformerFactory.Core().InternalVersion().Projects().Informer().GetStore().Add(&project)
				coreInformerFactory.Core().InternalVersion().Seeds().Informer().GetStore().Add(&seed)
				attrs := admission.NewAttributesRecord(&shoot, nil, core.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, core.Resource("shoots").WithVersion("version"), "", admission.Create, &metav1.CreateOptions{}, false, nil)

				err := admissionHandler.Admit(context.TODO(), attrs, nil)

				Expect(err).To(MatchError(apierrors.NewBadRequest("shoot domain field .spec.dns.domain must be set if provider != unmanaged and assigned to a seed which does not disable DNS")))
			})
		})
	})
})
