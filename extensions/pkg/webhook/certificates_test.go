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

package webhook

import (
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Certificates", func() {
	const (
		modeUrl   = "url"
		name      = "provider-test"
		namespace = "test-namespace"
	)

	var emptyStringArray []string

	Describe("#generateNewCAAndServerCert", func() {
		It("should return an valid certificate for '127.0.1.1'", func() {
			_, serverCert, err := generateNewCAAndServerCert(modeUrl, namespace, name, "127.0.1.1")
			Expect(err).NotTo(HaveOccurred())

			Expect(serverCert.Certificate.IPAddresses).To(Equal([]net.IP{net.ParseIP("127.0.1.1")}))
			Expect(serverCert.Certificate.DNSNames).To(Equal(emptyStringArray))
		})
		It("should return an valid certificate for '::1'", func() {
			_, serverCert, err := generateNewCAAndServerCert(modeUrl, namespace, name, "::1")
			Expect(err).NotTo(HaveOccurred())

			Expect(serverCert.Certificate.IPAddresses).To(Equal([]net.IP{net.ParseIP("::1")}))
			Expect(serverCert.Certificate.DNSNames).To(Equal(emptyStringArray))
		})
		It("should return an valid certificate for 'test.invalid'", func() {
			_, serverCert, err := generateNewCAAndServerCert(modeUrl, namespace, name, "test.invalid")
			Expect(err).NotTo(HaveOccurred())

			Expect(serverCert.Certificate.DNSNames).To(Equal([]string{"test.invalid"}))
		})
		It("should return an valid certificate for 'test.invalid:8443'", func() {
			_, serverCert, err := generateNewCAAndServerCert(modeUrl, namespace, name, "test.invalid:8443")
			Expect(err).NotTo(HaveOccurred())

			Expect(serverCert.Certificate.DNSNames).To(Equal([]string{"test.invalid"}))
		})
		It("should return the original url value for an invalid formatted value of 'test.invalid:8443:invalid'", func() {
			_, serverCert, err := generateNewCAAndServerCert(modeUrl, namespace, name, "test.invalid:8443:invalid")
			Expect(err).NotTo(HaveOccurred())

			Expect(serverCert.Certificate.DNSNames).To(Equal([]string{"test.invalid:8443:invalid"}))
		})
	})
})
