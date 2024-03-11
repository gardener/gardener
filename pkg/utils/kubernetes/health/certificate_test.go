// Copyright 2024 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package health_test

import (
	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
)

var _ = Describe("Cert Management", func() {
	Describe("Certificate", func() {
		var certificate *certv1alpha1.Certificate

		BeforeEach(func() {
			certificate = &certv1alpha1.Certificate{}
		})

		Describe("#CheckCertificate", func() {
			It("should return no error because certificate is ready", func() {
				certificate.Status.State = "Ready"
				certificate.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: "True"},
				}

				Expect(health.CheckCertificate(certificate)).ToNot(HaveOccurred())
			})

			It("should return an error because state is not ready", func() {
				certificate.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: "True"},
				}

				Expect(health.CheckCertificate(certificate)).To(MatchError(`certificate state is "" ("Ready" expected)`))
			})

			It("should return an error because condition is not ready", func() {
				certificate.Status.Conditions = []metav1.Condition{
					{Type: "Ready", Status: "False", Reason: "SomeReason", Message: "Some message"},
				}

				Expect(health.CheckCertificate(certificate)).To(MatchError(`condition "Ready" has invalid status False (expected True) due to SomeReason: Some message`))
			})
		})

		Describe("#IsCertificateProgressing", func() {
			It("should return false because certificate is rolled out", func() {
				result, reason := health.IsCertificateProgressing(certificate)
				Expect(result).To(BeFalse())
				Expect(reason).To(Equal("Certificate is fully rolled out"))
			})

			It("should return true because observed generation is outdated", func() {
				certificate.Generation = 10
				certificate.Status.ObservedGeneration = 1

				result, reason := health.IsCertificateProgressing(certificate)
				Expect(result).To(BeTrue())
				Expect(reason).To(Equal(`observed generation outdated (1/10)`))
			})
		})
	})

	Describe("Issuer", func() {
		var issuer *certv1alpha1.Issuer

		BeforeEach(func() {
			issuer = &certv1alpha1.Issuer{}
		})

		Describe("#CheckCertificateIssuer", func() {
			It("should return no error because issuer is ready", func() {
				issuer.Status.State = "Ready"

				Expect(health.CheckCertificateIssuer(issuer)).ToNot(HaveOccurred())
			})

			It("should return an error because issuer is not ready", func() {
				Expect(health.CheckCertificateIssuer(issuer)).To(MatchError(`issuer state is "" ("Ready" expected)`))
			})
		})

		Describe("#IsCertificateIssuerProgressing", func() {
			It("should return true because issuer is rolled out", func() {
				result, reason := health.IsCertificateIssuerProgressing(issuer)
				Expect(result).To(BeFalse())
				Expect(reason).To(Equal(`Issuer is fully rolled out`))
			})

			It("should return true because observed generation is outdated", func() {
				issuer.Generation = 10
				issuer.Status.ObservedGeneration = 1

				result, reason := health.IsCertificateIssuerProgressing(issuer)
				Expect(result).To(BeTrue())
				Expect(reason).To(Equal(`observed generation outdated (1/10)`))
			})
		})
	})
})
