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

package utils

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"time"

	"golang.org/x/crypto/ssh"
)

// Secret is a struct which contains a name and is used to be inherited from for more advanced secrets.
// * DoNotApply is a boolean value which can be used to prevent creating the Secret in the Seed cluster.
//   This can be useful to generate secrets which will be used in the Shoot cluster (whose API server
//   might not be available yet).
type Secret struct {
	Name       string
	DoNotApply bool
}

// TLSSecret is a struct which inherits from Secret (i.e., it gets a name) and which allows specifying the
// required properties for the to-be-created certificate. It always contains a 2048-bit RSA private key
// and can be either a server of a client certificate.
// * CommonName is the common name used in the certificate.
// * Organization is a list of organizations used in the certificate.
// * DNSNames is a list of DNS names for the Subject Alternate Names list.
// * IPAddresses is a list of IP addresses for the Subject Alternate Names list.
// * CertType specifies the usages of the certificate (server, client, both).
type TLSSecret struct {
	Secret
	CommonName   string
	Organization []string
	DNSNames     []string
	IPAddresses  []net.IP
	CertType     certType
	WantsCA      bool
}

type certType string

const (
	// ServerCert indicates that the certificate should have the ExtKeyUsageServerAuth usage.
	ServerCert certType = "server"

	// ClientCert indicates that the certificate should have the ExtKeyUsageClientAuth usage.
	ClientCert certType = "client"

	// ServerClientCert indicates that the certificate should have both the ExtKeyUsageServerAuth and ExtKeyUsageClientAuth usage.
	ServerClientCert certType = "both"
)

// GenerateCertificate takes a TLSSecret object, the CA certificate template and the CA private key, and it
// generates a new 2048-bit RSA private key along with a X509 certificate which will be signed by the given
// CA. The private key as well as the certificate will be returned PEM-encoded.
func GenerateCertificate(secret TLSSecret, CACertificateTemplate *x509.Certificate, CAPrivateKey *rsa.PrivateKey) ([]byte, []byte, error) {
	privateKey, err := GenerateRSAPrivateKey(2048)
	if err != nil {
		return nil, nil, err
	}
	privateKeyPEM := EncodePrivateKey(privateKey)
	certificateTemplate := GenerateCertificateTemplate(secret.CommonName, secret.Organization, secret.DNSNames, secret.IPAddresses, false, secret.CertType)
	certificatePEM, err := SignCertificate(certificateTemplate, CACertificateTemplate, privateKey, CAPrivateKey)
	if err != nil {
		return nil, nil, err
	}
	return privateKeyPEM, certificatePEM, nil
}

// GenerateCA generates a Certificate Authority and returns its private key, certificate template and PEM certificate which can be used to sign other x509 certificates
func GenerateCA() (*rsa.PrivateKey, *x509.Certificate, []byte, error) {
	CAPrivateKey, err := GenerateRSAPrivateKey(2048)
	if err != nil {
		return nil, nil, nil, err
	}
	CACertificateTemplate := GenerateCertificateTemplate("kubernetes", nil, nil, nil, true, "")
	CACertificatePEM, err := SignCertificate(CACertificateTemplate, CACertificateTemplate, CAPrivateKey, CAPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}
	return CAPrivateKey, CACertificateTemplate, CACertificatePEM, nil
}

// LoadCA takes a byte slice representation of a Certificate Authority and returns its private key, certificate template and PEM certificate which can be used to sign other x509 certificaates
func LoadCA(CAKey, CACert []byte) (*rsa.PrivateKey, *x509.Certificate, []byte, error) {
	CAPrivateKey, err := DecodePrivateKey(CAKey)
	if err != nil {
		return nil, nil, nil, err
	}
	CACertificateTemplate, err := DecodeCertificate(CACert)
	if err != nil {
		return nil, nil, nil, err
	}
	CACertificatePEM, err := SignCertificate(CACertificateTemplate, CACertificateTemplate, CAPrivateKey, CAPrivateKey)
	if err != nil {
		return nil, nil, nil, err
	}

	return CAPrivateKey, CACertificateTemplate, CACertificatePEM, nil
}

// GenerateRSAPrivateKey generates a RSA private for the given number of <bits>.
func GenerateRSAPrivateKey(bits int) (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, bits)
}

// GenerateRSAPublicKey takes a RSA private key <privateKey> and generates the corresponding public key.
// It serializes the public key for inclusion in an OpenSSH `authorized_keys` file and it trims the new-
// line at the end.
func GenerateRSAPublicKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	pubKey, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}
	publicKey := ssh.MarshalAuthorizedKey(pubKey)
	return bytes.Trim(publicKey, "\x0a"), nil
}

// GenerateCertificateTemplate creates a X509 Certificate object based on the provided information regarding
// common name, organization, SANs (DNS names and IP addresses). It can create a server or a client certificate
// or both, depending on the <certType> value. If <isCACert> is true, then a CA certificate is being created.
// The certificates a valid for 10 years.
func GenerateCertificateTemplate(commonName string, organization, dnsNames []string, ipAddresses []net.IP, isCA bool, certType certType) *x509.Certificate {
	serialNumber, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	template := &x509.Certificate{
		IsCA: isCA,
		BasicConstraintsValid: true,
		SerialNumber:          serialNumber,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // + 10 years
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		Subject: pkix.Name{
			CommonName:   commonName,
			Organization: organization,
		},
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
	}
	if isCA {
		template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	} else {
		switch certType {
		case ServerCert:
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
		case ClientCert:
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
		case ServerClientCert:
			template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
		}
	}
	return template
}

// SignCertificate takes a <certificateTemplate> and a <certificateTemplateSigner> which is used to sign
// the first. It also requires the corresponding private keys of both certificates. The created certificate
// is returned as byte slice.
func SignCertificate(certificateTemplate, certificateTemplateSigner *x509.Certificate, privateKey, privateKeySigner *rsa.PrivateKey) ([]byte, error) {
	certificate, err := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplateSigner, &privateKey.PublicKey, privateKeySigner)
	if err != nil {
		return nil, err
	}
	return EncodeCertificate(certificate), nil
}

// GenerateBasicAuthData computes a username/password keypair. It uses "admin" as username and generates a
// random password of length 32.
func GenerateBasicAuthData() (string, string, error) {
	password, err := GenerateRandomString(32)
	return "admin", password, err
}
