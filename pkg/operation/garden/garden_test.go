// SPDX-FileCopyrightText: 2018 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package garden_test

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/operation/common"
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
				includeZones = []string{"a", "b"}
				excludeZones = []string{"c", "d"}

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider:     provider,
							common.DNSDomain:       domain,
							common.DNSIncludeZones: strings.Join(includeZones, ","),
							common.DNSExcludeZones: strings.Join(excludeZones, ","),
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
			Expect(defaultDomains).To(Equal([]*Domain{
				{
					Domain:       domain,
					Provider:     provider,
					SecretData:   data,
					IncludeZones: includeZones,
					ExcludeZones: excludeZones,
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
				includeZones = []string{"a", "b"}
				excludeZones = []string{"c", "d"}

				secret = &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							common.DNSProvider:     provider,
							common.DNSDomain:       domain,
							common.DNSIncludeZones: strings.Join(includeZones, ","),
							common.DNSExcludeZones: strings.Join(excludeZones, ","),
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
			Expect(internalDomain).To(Equal(&Domain{
				Domain:       domain,
				Provider:     provider,
				SecretData:   data,
				IncludeZones: includeZones,
				ExcludeZones: excludeZones,
			}))
		})

		It("should return an error due to incomplete secrets map", func() {
			_, err := GetInternalDomain(map[string]*corev1.Secret{})

			Expect(err).NotTo(HaveOccurred())
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
		defaultDomain           = &Domain{
			Domain:     "bar.com",
			Provider:   defaultDomainProvider,
			SecretData: defaultDomainSecretData,
		}
	)

	DescribeTable("#DomainIsDefaultDomain",
		func(domain string, defaultDomains []*Domain, expected gomegatypes.GomegaMatcher) {
			Expect(DomainIsDefaultDomain(domain, defaultDomains)).To(expected)
		},

		Entry("no default domain", "foo.bar.com", nil, BeNil()),
		Entry("default domain", "foo.bar.com", []*Domain{defaultDomain}, Equal(defaultDomain)),
		Entry("no default domain but with same suffix", "foo.foobar.com", []*Domain{defaultDomain}, BeNil()),
	)
})
