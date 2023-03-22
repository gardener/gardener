// Copyright 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package gardener_test

import (
	"context"
	"crypto/x509"
	"crypto/x509/pkix"
	"net"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	fakeclient "sigs.k8s.io/controller-runtime/pkg/client/fake"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/utils/gardener"
)

var _ = Describe("utils", func() {
	DescribeTable("#ComputeGardenNamespace",
		func(name, expected string) {
			Expect(ComputeGardenNamespace(name)).To(Equal(expected))
		},

		Entry("empty name", "", "seed-"),
		Entry("garden", "garden", "seed-garden"),
		Entry("dash", "-", "seed--"),
		Entry("garden prefixed with dash", "-garden", "seed--garden"),
	)

	DescribeTable("#ComputeSeedName",
		func(name, expected string) {
			Expect(ComputeSeedName(name)).To(Equal(expected))
		},

		Entry("expect error with empty name", "", ""),
		Entry("expect error with garden name", "garden", ""),
		Entry("expect error with dash", "-", ""),
		Entry("expect success with empty name", "seed-", ""),
		Entry("expect success with dash name", "seed--", "-"),
		Entry("expect success with duplicated prefix", "seed-seed-", "seed-"),
		Entry("expect success with duplicated prefix", "seed-seed-a", "seed-a"),
		Entry("expect success with garden name", "seed-garden", "garden"),
	)

	DescribeTable("#IsSeedClientCert",
		func(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, expectedStatus bool, expectedReason gomegatypes.GomegaMatcher) {
			status, reason := IsSeedClientCert(x509cr, usages)
			Expect(status).To(Equal(expectedStatus))
			Expect(reason).To(expectedReason)
		},

		Entry("org does not match", &x509.CertificateRequest{}, nil, false, ContainSubstring("organization")),
		Entry("dns names given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, DNSNames: []string{"foo"}}, nil, false, ContainSubstring("DNSNames")),
		Entry("email addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, EmailAddresses: []string{"foo"}}, nil, false, ContainSubstring("EmailAddresses")),
		Entry("ip addresses given", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}, IPAddresses: []net.IP{{}}}, nil, false, ContainSubstring("IPAddresses")),
		Entry("key usages do not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, nil, false, ContainSubstring("key usages")),
		Entry("common name does not match", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, false, ContainSubstring("CommonName")),
		Entry("everything matches", &x509.CertificateRequest{Subject: pkix.Name{Organization: []string{"gardener.cloud:system:seeds"}, CommonName: "gardener.cloud:system:seed:foo"}}, []certificatesv1.KeyUsage{certificatesv1.UsageKeyEncipherment, certificatesv1.UsageDigitalSignature, certificatesv1.UsageClientAuth}, true, Equal("")),
	)

	Describe("#ComputeNginxIngressClassForSeed", func() {
		var (
			seed              *gardencorev1beta1.Seed
			kubernetesVersion *string
		)

		BeforeEach(func() {
			seed = &gardencorev1beta1.Seed{}
			kubernetesVersion = pointer.String("1.20.3")
		})

		It("should return an error because kubernetes version is nil", func() {
			class, err := ComputeNginxIngressClassForSeed(seed, nil)
			Expect(class).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("kubernetes version is missing for seed")))
		})

		It("should return an error because kubernetes version cannot be parsed", func() {
			class, err := ComputeNginxIngressClassForSeed(seed, pointer.String("foo"))
			Expect(class).To(BeEmpty())
			Expect(err).To(MatchError(ContainSubstring("Invalid Semantic Version")))
		})

		Context("when seed does not want managed ingress", func() {
			It("should return 'nginx'", func() {
				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx"))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when seed wants managed ingress", func() {
			BeforeEach(func() {
				seed.Spec.DNS.Provider = &gardencorev1beta1.SeedDNSProvider{}
				seed.Spec.Ingress = &gardencorev1beta1.Ingress{Controller: gardencorev1beta1.IngressController{Kind: v1beta1constants.IngressKindNginx}}
			})

			It("should return 'nginx-gardener' when kubernetes version < 1.22", func() {
				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx-gardener"))
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return 'nginx-ingress-gardener' when kubernetes version >= 1.22", func() {
				kubernetesVersion = pointer.String("1.22.0")

				class, err := ComputeNginxIngressClassForSeed(seed, kubernetesVersion)
				Expect(class).To(Equal("nginx-ingress-gardener"))
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("#GetWilcardCertificate", func() {
		var (
			ctx        = context.TODO()
			fakeClient client.Client
			secret     *corev1.Secret
		)

		BeforeEach(func() {
			fakeClient = fakeclient.NewClientBuilder().WithScheme(kubernetesscheme.Scheme).Build()

			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "secret-",
					Namespace:    "garden",
					Labels:       map[string]string{"gardener.cloud/role": "controlplane-cert"},
				},
			}
		})

		It("should return an error because there are more than one wildcard certificates", func() {
			secret2 := secret.DeepCopy()
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())
			Expect(fakeClient.Create(ctx, secret2)).To(Succeed())

			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(BeNil())
			Expect(err).To(MatchError(ContainSubstring("misconfigured seed cluster: not possible to provide more than one secret with annotation")))
		})

		It("should return the wildcard certificate secret", func() {
			Expect(fakeClient.Create(ctx, secret)).To(Succeed())

			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(Equal(secret))
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return nil because there is no wildcard certificate secret", func() {
			result, err := GetWildcardCertificate(ctx, fakeClient)
			Expect(result).To(BeNil())
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
