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

package testutils

import (
	"net"
	"time"

	"github.com/gardener/gardener/pkg/utils/secrets"
	. "github.com/onsi/gomega"
)

const x509CommonName = "gardener.cloud:system:seed:test"

var (
	x509Organization = []string{"gardener.cloud:system:seeds"}
	x509DnsNames     = []string{"my.alternative.apiserver.domain"}
	x509IpAddresses  = []net.IP{net.ParseIP("100.64.0.10").To4()}
)

// GenerateTLSServingCertificate is a common utility function for validation tests to generate
// a TLS serving certificate
func GenerateTLSServingCertificate(ca *secrets.Certificate) secrets.Certificate {
	return GenerateCertificate(10*time.Hour, secrets.ServerCert, ca)
}

// GenerateTLSServingCertificateWithValidity is a common utility function for validation tests to generate
// a TLS serving certificate with a custom validity
func GenerateTLSServingCertificateWithValidity(ca *secrets.Certificate, validity time.Duration) secrets.Certificate {
	return GenerateCertificate(validity, secrets.ServerCert, ca)
}

// GenerateClientCertificate is a common utility function for validation tests to generate
// a client certificate
func GenerateClientCertificate(ca *secrets.Certificate) secrets.Certificate {
	return GenerateCertificate(10*time.Hour, secrets.ClientCert, ca)
}

// GenerateClientCertificateWithValidity is a common utility function for validation tests to generate
// a client certificate with a custom validity
func GenerateClientCertificateWithValidity(ca *secrets.Certificate, validity time.Duration) secrets.Certificate {
	return GenerateCertificate(validity, secrets.ClientCert, ca)
}

// GenerateCertificate is a common utility function for validation tests to generate
// a certificate with default configuration
func GenerateCertificate(validity time.Duration, certType secrets.CertType, ca *secrets.Certificate) secrets.Certificate {
	caCertConfig := &secrets.CertificateSecretConfig{
		Name:         "test",
		CommonName:   x509CommonName,
		Organization: x509Organization,
		DNSNames:     x509DnsNames,
		IPAddresses:  x509IpAddresses,
		CertType:     certType,
		Validity:     &validity,
	}

	if ca != nil {
		caCertConfig.SigningCA = ca
	}

	cert, err := caCertConfig.GenerateCertificate()
	Expect(err).ToNot(HaveOccurred())
	return *cert
}

// GenerateCACertificate is a common utility function for validation tests to generate a CA certificate
func GenerateCACertificate(commonName string) secrets.Certificate {
	return generateCACertificate(commonName, 10*time.Hour)
}

// GenerateCACertificateWithValidity is a common utility function for validation tests to generate a CA certificate
// with a custom validity
func GenerateCACertificateWithValidity(commonName string, validity time.Duration) secrets.Certificate {
	return generateCACertificate(commonName, validity)
}

func generateCACertificate(commonName string, validity time.Duration) secrets.Certificate {
	caCertConfig := &secrets.CertificateSecretConfig{
		Name:         commonName,
		CommonName:   commonName,
		Organization: x509Organization,
		DNSNames:     x509DnsNames,
		IPAddresses:  x509IpAddresses,
		CertType:     secrets.CACert,
		Validity:     &validity,
	}
	cert, err := caCertConfig.GenerateCertificate()
	Expect(err).ToNot(HaveOccurred())
	return *cert
}
