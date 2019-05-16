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

package garden_test

import (
	"fmt"

	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/operation/garden"
	. "github.com/gardener/gardener/pkg/operation/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Garden", func() {
	Describe("#GetDefaultDomains", func() {
		It("should return all default domain", func() {
			var (
				provider = "aws"
				domain   = "nip.io"
				data     = map[string][]byte{
					"foo": []byte("bar"),
				}
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider: provider,
							common.DNSDomain:   domain,
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					fmt.Sprintf("%s-%s", common.GardenRoleDefaultDomain, domain): secret,
				}
			)

			defaultDomains, err := GetDefaultDomains(secrets)

			Expect(err).NotTo(HaveOccurred())
			Expect(defaultDomains).To(Equal([]*DefaultDomain{
				{
					Domain:     domain,
					Provider:   provider,
					SecretData: data,
				},
			}))
		})

		It("should return an error", func() {
			secrets := map[string]*corev1.Secret{
				fmt.Sprintf("%s-%s", common.GardenRoleDefaultDomain, "nip"): {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider: "aws",
						},
					},
				},
			}

			_, err := GetDefaultDomains(secrets)

			Expect(err).To(HaveOccurred())
		})
	})

	Describe("#GetInternalDomain", func() {
		It("should return the internal domain", func() {
			var (
				provider = "aws"
				domain   = "nip.io"
				data     = map[string][]byte{
					"foo": []byte("bar"),
				}
				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider: provider,
							common.DNSDomain:   domain,
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					common.GardenRoleInternalDomain: secret,
				}
			)

			internalDomain, err := GetInternalDomain(secrets)

			Expect(err).NotTo(HaveOccurred())
			Expect(internalDomain).To(Equal(&InternalDomain{
				Domain:     domain,
				Provider:   provider,
				SecretData: data,
			}))
		})

		It("should return an error due to incomplete secrets map", func() {
			_, err := GetInternalDomain(map[string]*corev1.Secret{})

			Expect(err).To(HaveOccurred())
		})

		It("should return an error", func() {
			secrets := map[string]*corev1.Secret{
				common.GardenRoleInternalDomain: {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider: "aws",
						},
					},
				},
			}

			_, err := GetInternalDomain(secrets)

			Expect(err).To(HaveOccurred())
		})
	})

	var (
		defaultDomainProvider   = "default-domain-provider"
		defaultDomainSecretData = map[string][]byte{"default": []byte("domain")}
		defaultDomain           = &garden.DefaultDomain{
			Domain:     "bar.com",
			Provider:   defaultDomainProvider,
			SecretData: defaultDomainSecretData,
		}
	)

	DescribeTable("#DomainIsDefaultDomain",
		func(domain string, defaultDomains []*garden.DefaultDomain, expected gomegatypes.GomegaMatcher) {
			Expect(DomainIsDefaultDomain(domain, defaultDomains)).To(expected)
		},

		Entry("no default domain", "foo.bar.com", nil, BeNil()),
		Entry("default domain", "foo.bar.com", []*garden.DefaultDomain{defaultDomain}, Equal(defaultDomain)),
	)
})
