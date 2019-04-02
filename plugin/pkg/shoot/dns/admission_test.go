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
	"fmt"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/dns"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
)

var _ = Describe("dns", func() {
	Describe("#Admit", func() {
		var (
			admissionHandler      *DNS
			kubeInformerFactory   kubeinformers.SharedInformerFactory
			gardenInformerFactory gardeninformers.SharedInformerFactory
			shoot                 garden.Shoot

			namespace   = "my-namespace"
			projectName = "my-project"
			shootName   = "shoot"

			domain   = "example.com"
			provider = garden.DNSUnmanaged

			project = garden.Project{
				ObjectMeta: metav1.ObjectMeta{
					Name: projectName,
				},
				Spec: garden.ProjectSpec{
					Namespace: &namespace,
				},
			}

			defaultDomainProvider = "my-dns-provider"
			defaultDomainSecret   = corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secret-1",
					Namespace: common.GardenNamespace,
					Labels: map[string]string{
						common.GardenRole: common.GardenRoleDefaultDomain,
					},
					Annotations: map[string]string{
						common.DNSDomain:   domain,
						common.DNSProvider: defaultDomainProvider,
					},
				},
			}

			shootBase = garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      shootName,
					Namespace: namespace,
				},
				Spec: garden.ShootSpec{},
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
			shootBase.Spec.DNS.Provider = &provider
			shoot = shootBase
		})

		It("should do nothing because the shoot specifies the 'unmanaged' dns provider", func() {
			shootBefore := shoot.DeepCopy()
			attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

			err := admissionHandler.Admit(attrs, nil)

			Expect(err).NotTo(HaveOccurred())
			Expect(shoot).To(Equal(*shootBefore))
		})

		Context("provider is not 'unmanaged'", func() {
			BeforeEach(func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Provider = nil
			})

			It("should pass because a default domain was generated for the shoot (no domain)", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Provider).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(fmt.Sprintf("%s.%s.%s", shootName, projectName, domain)))
			})

			It("should pass because a default domain was generated for the shoot (with domain)", func() {
				shootDomain := fmt.Sprintf("my-shoot.%s", domain)
				shoot.Spec.DNS.Domain = &shootDomain

				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).NotTo(HaveOccurred())
				Expect(shoot.Spec.DNS.Provider).To(BeNil())
				Expect(*shoot.Spec.DNS.Domain).To(Equal(shootDomain))
			})

			It("should reject because no domain was configured for the shoot and project is missing", func() {
				kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&defaultDomainSecret)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(MatchError(apierrors.NewBadRequest("shoot domain field .spec.dns.domain must be set if provider != unmanaged")))
			})

			It("should reject because no domain was configured for the shoot and default domain secret is missing", func() {
				gardenInformerFactory.Garden().InternalVersion().Projects().Informer().GetStore().Add(&project)
				attrs := admission.NewAttributesRecord(&shoot, nil, garden.Kind("Shoot").WithVersion("version"), shoot.Namespace, shoot.Name, garden.Resource("shoots").WithVersion("version"), "", admission.Create, false, nil)

				err := admissionHandler.Admit(attrs, nil)

				Expect(err).To(MatchError(apierrors.NewBadRequest("shoot domain field .spec.dns.domain must be set if provider != unmanaged")))
			})
		})
	})
})
