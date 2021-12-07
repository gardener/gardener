// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package controller

import (
	"time"

	testutils "github.com/gardener/gardener/landscaper/common/test-utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	"github.com/gardener/gardener/pkg/logger"
	. "github.com/onsi/ginkgo"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"

	. "github.com/onsi/gomega"
)

var _ = Describe("Generate TLS serving certificates", func() {
	var (
		ca            = testutils.GenerateCACertificate("test")
		caCrt         = string(ca.CertificatePEM)
		privatKeyCA   = string(ca.PrivateKeyPEM)
		testOperation operation
	)

	BeforeEach(func() {
		testOperation = operation{
			log: logrus.NewEntry(logger.NewNopLogger()),
			imports: &imports.Imports{
				GardenerAPIServer: imports.GardenerAPIServer{
					ComponentConfiguration: imports.APIServerComponentConfiguration{
						CA: &imports.CA{
							Crt: &caCrt,
						},
						TLS: &imports.TLSServer{
							Validity: &metav1.Duration{Duration: 30 * time.Second},
						},
					},
				},
				GardenerControllerManager: &imports.GardenerControllerManager{
					ComponentConfiguration: &imports.ControllerManagerComponentConfiguration{
						TLS: &imports.TLSServer{
							Validity: &metav1.Duration{Duration: 30 * time.Second},
						},
					},
				},
				GardenerAdmissionController: &imports.GardenerAdmissionController{
					Enabled: true,
					ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
						CA: &imports.CA{
							Crt: &caCrt,
						},
						TLS: &imports.TLSServer{
							Validity: &metav1.Duration{Duration: 30 * time.Second},
						},
					},
				},
			},
		}
	})

	It("should fail - cannot generate a new TLS serving certificate without a CA private key", func() {
		err := testOperation.GenerateAPIServerCertificates(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("attempted to use the CA certificate for the Gardener API Server"))

		err = testOperation.GenerateAdmissionControllerCertificates(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("attempted to use the CA certificate for the Gardener Admission Controller"))

		err = testOperation.GenerateControllerManagerCertificates(nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("TLS serving certificates for the Gardener Controller Manager"))
	})

	It("should generate a new TLS serving certificate", func() {
		testOperation.imports.GardenerAPIServer.ComponentConfiguration.CA.Key = &privatKeyCA
		testOperation.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key = &privatKeyCA

		Expect(testOperation.GenerateAPIServerCertificates(nil)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(validation.ValidateTLS(
			*testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt,
			*testOperation.imports.GardenerAPIServer.ComponentConfiguration.TLS.Key,
			&caCrt,
			field.NewPath(""))).To(HaveLen(0))

		Expect(testOperation.GenerateAdmissionControllerCertificates(nil)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(validation.ValidateTLS(
			*testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt,
			*testOperation.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Key,
			&caCrt,
			field.NewPath(""))).To(HaveLen(0))

		Expect(testOperation.GenerateControllerManagerCertificates(nil)).ToNot(HaveOccurred())
		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt).ToNot(BeNil())
		Expect(testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key).ToNot(BeNil())

		Expect(validation.ValidateTLS(
			*testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt,
			*testOperation.imports.GardenerControllerManager.ComponentConfiguration.TLS.Key,
			&caCrt,
			field.NewPath(""))).To(HaveLen(0))
	})
})
