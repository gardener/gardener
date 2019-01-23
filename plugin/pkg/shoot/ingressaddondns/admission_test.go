// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package ingressaddondns_test

import (
	"github.com/gardener/gardener/pkg/operation/common"
	. "github.com/gardener/gardener/plugin/pkg/shoot/ingressaddondns"
	. "github.com/onsi/ginkgo"

	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/gardener/gardener/pkg/apis/garden"
	gardeninformers "github.com/gardener/gardener/pkg/client/garden/informers/internalversion"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/admission"
	kubeinformers "k8s.io/client-go/informers"
)

var _ = Describe("ingressaddon", func() {
	var (
		admissionHandler      *IngressAddon
		kubeInformerFactory   kubeinformers.SharedInformerFactory
		gardenInformerFactory gardeninformers.SharedInformerFactory
		shoot                 *garden.Shoot

		shootName      = "shoot"
		namespace      = "project"
		domain         = "shoot.project.example.com"
		internalDomain = "internal.example.com"

		resource = garden.Resource("shoots").WithVersion("version")
		kind     = garden.Kind("Shoot").WithVersion(garden.SchemeGroupVersion.Version)
	)

	BeforeEach(func() {
		admissionHandler, _ = New()
		admissionHandler.AssignReadyFunc(func() bool { return true })
		kubeInformerFactory = kubeinformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetKubeInformerFactory(kubeInformerFactory)
		gardenInformerFactory = gardeninformers.NewSharedInformerFactory(nil, 0)
		admissionHandler.SetInternalGardenInformerFactory(gardenInformerFactory)

		shoot = &garden.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Name:      shootName,
				Namespace: namespace,
			},
			Spec: garden.ShootSpec{
				DNS: garden.DNS{
					Domain: &domain,
				},
			},
		}
	})

	Context("shoot creation", func() {
		operation := admission.Create

		It("should fail because additionalRecords is set during creation", func() {
			shoot.Spec.Addons = &garden.Addons{
				NginxIngress: &garden.NginxIngress{
					IngressDNS: garden.IngressDNS{
						AdditionalRecords: []string{"shoot.internal.example.com"},
					},
				},
			}

			attrs := admission.NewAttributesRecord(shoot, nil, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("additionalRecords is expected to be empty but found"))
		})

		It("should fail because internal domain secret is not availble", func() {
			shoot.Spec.Addons = &garden.Addons{
				NginxIngress: &garden.NginxIngress{},
			}

			attrs := admission.NewAttributesRecord(shoot, nil, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).ToNot(BeNil())
			Expect(err.Error()).To(ContainSubstring("require exactly ONE internal domain secret"))
		})

		It("should pass and got an additional and standard DNS record assigned", func() {
			shoot.Spec.Addons = &garden.Addons{
				NginxIngress: &garden.NginxIngress{},
			}

			internalDomainSecret := corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						common.DNSDomain: internalDomain,
					},
					Labels: map[string]string{
						common.GardenRole: common.GardenRoleInternalDomain,
					},
					Name:      "internalDomain",
					Namespace: "default",
				},
			}
			kubeInformerFactory.Core().V1().Secrets().Informer().GetStore().Add(&internalDomainSecret)

			attrs := admission.NewAttributesRecord(shoot, nil, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(BeNil())
			Expect(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords).To(ConsistOf(HaveSuffix(internalDomain)))
			Expect(shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords).To(ConsistOf(HaveSuffix(utils.GenerateIngressDomain(domain))))
		})

		It("should pass because Nginx Ingress is not defnied", func() {
			shoot.Spec.Addons = &garden.Addons{
				NginxIngress: nil,
			}

			attrs := admission.NewAttributesRecord(shoot, nil, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(BeNil())
		})

		It("should pass because addons are not defnied", func() {
			shoot.Spec.Addons = nil

			attrs := admission.NewAttributesRecord(shoot, nil, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)

			Expect(err).To(BeNil())
		})
	})

	Context("shoot update", func() {
		operation := admission.Update

		var oldShoot *garden.Shoot
		BeforeEach(func() {
			shoot.Spec.Addons = &garden.Addons{
				NginxIngress: &garden.NginxIngress{
					IngressDNS: garden.IngressDNS{
						AdditionalRecords: []string{"a1f4cv." + internalDomain},
					},
				},
			}
			oldShoot = shoot.DeepCopy()
		})

		It("should fail because additionalRecords are modified", func() {
			oldShoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords = []string{internalDomain}

			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).ToNot(BeNil())
			Expect(err).To(PointTo(MatchAllFields(Fields{
				"ErrStatus": MatchFields(IgnoreExtras, Fields{
					"Code":    Equal(int32(403)),
					"Message": ContainSubstring("must not be changed"),
				}),
			})))
		})

		It("should fail because additionalRecords are added", func() {
			oldShoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords = append(oldShoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords, internalDomain)

			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).ToNot(BeNil())
			Expect(err).To(PointTo(MatchAllFields(Fields{
				"ErrStatus": MatchFields(IgnoreExtras, Fields{
					"Code":    Equal(int32(403)),
					"Message": ContainSubstring("must not be changed"),
				}),
			})))
		})

		It("should fail because additionalRecords are added and Nginx Ingress was not defined before", func() {
			oldShoot.Spec.Addons.NginxIngress = nil

			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).ToNot(BeNil())
			Expect(err).To(PointTo(MatchAllFields(Fields{
				"ErrStatus": MatchFields(IgnoreExtras, Fields{
					"Code":    Equal(int32(403)),
					"Message": ContainSubstring("additionalRecords is expected to be empty"),
				}),
			})))
		})

		It("should fail because additionalRecords are added and Addons were not defined before", func() {
			oldShoot.Spec.Addons = nil

			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).ToNot(BeNil())
			Expect(err).To(PointTo(MatchAllFields(Fields{
				"ErrStatus": MatchFields(IgnoreExtras, Fields{
					"Code":    Equal(int32(403)),
					"Message": ContainSubstring("additionalRecords is expected to be empty"),
				}),
			})))
		})

		It("should pass because additionalRecords are unchanged", func() {
			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).To(BeNil())
			Expect(shoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords).To(ConsistOf(oldShoot.Spec.Addons.NginxIngress.IngressDNS.AdditionalRecords[0]))
			Expect(shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords).To(ConsistOf(utils.GenerateIngressDomain(domain)))
		})

		It("should not adjust standard Ingress domain", func() {
			stdDomain := "ing." + domain
			shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords = []string{stdDomain}
			attrs := admission.NewAttributesRecord(shoot, oldShoot, kind, shoot.Namespace, shoot.Name, resource, "", operation, false, nil)
			err := admissionHandler.Admit(attrs)
			Expect(err).To(BeNil())
			Expect(shoot.Spec.Addons.NginxIngress.IngressDNS.StandardRecords[0]).To(Equal(stdDomain))
		})
	})
})
