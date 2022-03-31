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
	"context"
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/client/kubernetes/fake"
	. "github.com/gardener/gardener/pkg/operation/garden"
	gutil "github.com/gardener/gardener/pkg/utils/gardener"
	secretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager"
	fakesecretsmanager "github.com/gardener/gardener/pkg/utils/secrets/manager/fake"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"
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
							gutil.DNSProvider:     provider,
							gutil.DNSDomain:       domain,
							gutil.DNSIncludeZones: strings.Join(includeZones, ","),
							gutil.DNSExcludeZones: strings.Join(excludeZones, ","),
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					fmt.Sprintf("%s-%s", constants.GardenRoleDefaultDomain, domain): secret,
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
				fmt.Sprintf("%s-%s", constants.GardenRoleDefaultDomain, "nip"): {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							gutil.DNSProvider: "aws",
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
							gutil.DNSProvider:     provider,
							gutil.DNSDomain:       domain,
							gutil.DNSIncludeZones: strings.Join(includeZones, ","),
							gutil.DNSExcludeZones: strings.Join(excludeZones, ","),
						},
					},
					Data: data,
				}
				secrets = map[string]*corev1.Secret{
					constants.GardenRoleInternalDomain: secret,
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
				constants.GardenRoleInternalDomain: {
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							gutil.DNSProvider: "aws",
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

	Describe("#BootstrapCluster", func() {
		var (
			fakeGardenClient client.Client
			k8sGardenClient  kubernetes.Interface
			sm               secretsmanager.Interface

			ctx       = context.TODO()
			namespace = "garden"
		)

		BeforeEach(func() {
			fakeGardenClient = fakeclient.NewClientBuilder().WithScheme(kubernetes.GardenScheme).Build()
			k8sGardenClient = fake.NewClientSetBuilder().WithClient(fakeGardenClient).WithVersion("1.17.4").Build()
			sm = fakesecretsmanager.New(fakeGardenClient, namespace)
		})

		It("should return an error because the garden version cannot be parsed", func() {
			k8sGardenClient = fake.NewClientSetBuilder().WithClient(fakeGardenClient).WithVersion("").Build()

			Expect(BootstrapCluster(ctx, k8sGardenClient, sm)).To(MatchError(ContainSubstring("Invalid Semantic Version")))
		})

		It("should return an error because the garden version is too low", func() {
			k8sGardenClient = fake.NewClientSetBuilder().WithClient(fakeGardenClient).WithVersion("1.16.5").Build()

			Expect(BootstrapCluster(ctx, k8sGardenClient, sm)).To(MatchError(ContainSubstring("the Kubernetes version of the Garden cluster must be at least 1.17")))
		})

		It("should generate a global monitoring secret because none exists yet", func() {
			Expect(BootstrapCluster(ctx, k8sGardenClient, sm)).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeGardenClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{"gardener.cloud/role": "global-monitoring"})).To(Succeed())
			validateGlobalMonitoringSecret(secretList)
		})

		It("should generate a global monitoring secret because legacy secret exists", func() {
			legacySecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "monitoring-ingress-credentials",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "global-monitoring",
					},
				},
			}
			Expect(fakeGardenClient.Create(ctx, legacySecret)).To(Succeed())

			Expect(BootstrapCluster(ctx, k8sGardenClient, sm)).To(Succeed())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(legacySecret), &corev1.Secret{})).To(BeNotFoundError())

			secretList := &corev1.SecretList{}
			Expect(fakeGardenClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{"gardener.cloud/role": "global-monitoring"})).To(Succeed())
			validateGlobalMonitoringSecret(secretList)
		})

		It("should not generate a global monitoring secret because it already exists", func() {
			customSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "self-managed-secret",
					Namespace: namespace,
					Labels: map[string]string{
						"gardener.cloud/role": "global-monitoring",
					},
				},
			}
			Expect(fakeGardenClient.Create(ctx, customSecret)).To(Succeed())

			Expect(BootstrapCluster(ctx, k8sGardenClient, sm)).To(Succeed())

			Expect(fakeGardenClient.Get(ctx, client.ObjectKeyFromObject(customSecret), &corev1.Secret{})).To(Succeed())

			secretList := &corev1.SecretList{}
			Expect(fakeGardenClient.List(ctx, secretList, client.InNamespace(namespace), client.MatchingLabels{
				"name":             "observability-ingress",
				"managed-by":       "secretsmanager",
				"manager-identity": "fake",
			})).To(Succeed())
			Expect(secretList.Items).To(BeEmpty())
		})
	})
})

func validateGlobalMonitoringSecret(secretList *corev1.SecretList) {
	Expect(secretList.Items).To(HaveLen(1))
	Expect(secretList.Items[0].Name).To(HavePrefix("observability-ingress-"))
	Expect(secretList.Items[0].Labels).To(And(
		HaveKeyWithValue("name", "observability-ingress"),
		HaveKeyWithValue("managed-by", "secrets-manager"),
		HaveKeyWithValue("manager-identity", "fake"),
	))
}
