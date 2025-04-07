// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package secrets

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/gardener/gardener/pkg/utils"
)

// CertType is a string alias for certificate types.
type CertType string

const (
	// CACert indicates that the certificate should be a certificate authority.
	CACert CertType = "ca"
	// ServerCert indicates that the certificate should have the ExtKeyUsageServerAuth usage.
	ServerCert CertType = "server"
	// ClientCert indicates that the certificate should have the ExtKeyUsageClientAuth usage.
	ClientCert CertType = "client"
	// ServerClientCert indicates that the certificate should have both the ExtKeyUsageServerAuth and ExtKeyUsageClientAuth usage.
	ServerClientCert CertType = "both"

	// DataKeyCertificate is the key in a secret data holding the certificate.
	DataKeyCertificate = "tls.crt"
	// DataKeyPrivateKey is the key in a secret data holding the private key.
	DataKeyPrivateKey = "tls.key"
	// DataKeyCertificateCA is the key in a secret data or config map data holding the CA certificate.
	DataKeyCertificateCA = "ca.crt"
	// DataKeyPrivateKeyCA is the key in a secret data holding the CA private key.
	DataKeyPrivateKeyCA = "ca.key"
)

const (
	// PKCS1 certificate format
	PKCS1 = iota
	// PKCS8 certificate format
	PKCS8
)

const (
	// allowedClockSkew is the offset to allow with regards to certificate creation/usage difference.
	allowedClockSkew = 1 * time.Minute
)

// CertificateSecretConfig contains the specification a to-be-generated CA, server, or client certificate.
// It always contains a 3072-bit RSA private key.
type CertificateSecretConfig struct {
	Name string

	CommonName   string
	Organization []string
	DNSNames     []string
	IPAddresses  []net.IP

	CertType  CertType
	SigningCA *Certificate
	PKCS      int

	Validity                          *time.Duration
	SkipPublishingCACertificate       bool
	IncludeCACertificateInServerChain bool
}

// Certificate contains the private key, and the certificate. It does also contain the CA certificate
// in case it is no CA. Otherwise, the <CA> field is nil.
type Certificate struct {
	Name string

	CA                                *Certificate
	CertType                          CertType
	SkipPublishingCACertificate       bool
	IncludeCACertificateInServerChain bool

	PrivateKey    *rsa.PrivateKey
	PrivateKeyPEM []byte

	Certificate    *x509.Certificate
	CertificatePEM []byte
}

// GetName returns the name of the secret.
func (s *CertificateSecretConfig) GetName() string {
	return s.Name
}

// Generate implements ConfigInterface.
func (s *CertificateSecretConfig) Generate() (DataInterface, error) {
	return s.GenerateCertificate()
}

// GenerateCertificate is the same as Generate but returns a *Certificate instead of the DataInterface.
func (s *CertificateSecretConfig) GenerateCertificate() (*Certificate, error) {
	certificateObj := &Certificate{
		Name:                              s.Name,
		CA:                                s.SigningCA,
		CertType:                          s.CertType,
		SkipPublishingCACertificate:       s.SkipPublishingCACertificate,
		IncludeCACertificateInServerChain: s.IncludeCACertificateInServerChain,
	}

	// If no cert type is given then we only return a certificate object that contains the CA.
	if s.CertType != "" {
		privateKey, err := GenerateKey(rand.Reader, 3072)
		if err != nil {
			return nil, err
		}

		var (
			certificate       = s.generateCertificateTemplate()
			certificateSigner = certificate
			privateKeySigner  = privateKey
		)

		if s.SigningCA != nil {
			certificateSigner = s.SigningCA.Certificate
			privateKeySigner = s.SigningCA.PrivateKey
		}

		certificatePEM, err := signCertificate(certificate, privateKey, certificateSigner, privateKeySigner)
		if err != nil {
			return nil, err
		}

		var pk []byte
		switch s.PKCS {
		case PKCS1:
			pk = utils.EncodePrivateKey(privateKey)
		case PKCS8:
			pk, err = utils.EncodePrivateKeyInPKCS8(privateKey)

			if err != nil {
				return nil, err
			}
		}

		certificateObj.PrivateKey = privateKey
		certificateObj.PrivateKeyPEM = pk
		certificateObj.Certificate = certificate
		certificateObj.CertificatePEM = certificatePEM
	}

	return certificateObj, nil
}

// SecretData computes the data map which can be used in a Kubernetes secret.
func (c *Certificate) SecretData() map[string][]byte {
	data := map[string][]byte{}

	switch {
	case c.CA == nil:
		// The certificate is a CA certificate itself, so we use different keys in the secret data (for backwards-
		// compatibility).
		data[DataKeyCertificateCA] = c.CertificatePEM
		data[DataKeyPrivateKeyCA] = c.PrivateKeyPEM

	case c.CA != nil:
		cert := c.CertificatePEM
		if c.CertType == ServerCert && c.IncludeCACertificateInServerChain {
			cert = make([]byte, 0, len(c.CA.CertificatePEM)+len(c.CertificatePEM))
			cert = append(cert, c.CertificatePEM...)
			cert = append(cert, c.CA.CertificatePEM...)
		}

		data[DataKeyCertificate] = cert
		data[DataKeyPrivateKey] = c.PrivateKeyPEM
		if !c.SkipPublishingCACertificate {
			data[DataKeyCertificateCA] = c.CA.CertificatePEM
		}
	}

	return data
}

// LoadCertificate takes a byte slice representation of a certificate and the corresponding private key, and returns its de-serialized private
// key, certificate template and PEM certificate which can be used to sign other x509 certificates.
func LoadCertificate(name string, privateKeyPEM, certificatePEM []byte) (*Certificate, error) {
	privateKey, err := utils.DecodePrivateKey(privateKeyPEM)
	if err != nil {
		return nil, err
	}
	certificate, err := utils.DecodeCertificate(certificatePEM)
	if err != nil {
		return nil, err
	}

	return &Certificate{
		Name: name,

		PrivateKey:    privateKey,
		PrivateKeyPEM: privateKeyPEM,

		Certificate:    certificate,
		CertificatePEM: certificatePEM,
	}, nil
}

// generateCertificateTemplate creates a X509 Certificate object based on the provided information regarding
// common name, organization, SANs (DNS names and IP addresses). It can create a server or a client certificate
// or both, depending on the <certType> value. If <isCACert> is true, then a CA certificate is being created.
// The certificates a valid for 10 years.
func (s *CertificateSecretConfig) generateCertificateTemplate() *x509.Certificate {
	now := Clock.Now()

	expiration := now.AddDate(10, 0, 0) // + 10 years
	if s.Validity != nil {
		expiration = now.Add(*s.Validity)
	}

	var (
		serialNumber, _ = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		isCA            = s.CertType == CACert

		template = &x509.Certificate{
			BasicConstraintsValid: true,
			IsCA:                  isCA,
			SerialNumber:          serialNumber,
			NotBefore:             AdjustToClockSkew(now),
			NotAfter:              expiration,
			KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
			Subject: pkix.Name{
				CommonName:   s.CommonName,
				Organization: s.Organization,
			},
			DNSNames:    s.DNSNames,
			IPAddresses: s.IPAddresses,
		}
	)

	switch s.CertType {
	case CACert:
		template.KeyUsage |= x509.KeyUsageCertSign | x509.KeyUsageCRLSign
	case ServerCert:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}
	case ClientCert:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}
	case ServerClientCert:
		template.ExtKeyUsage = []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}
	}

	return template
}

// SignCertificate takes a <certificateTemplate> and a <certificateTemplateSigner> which is used to sign
// the first. It also requires the corresponding private keys of both certificates. The created certificate
// is returned as byte slice.
func signCertificate(certificateTemplate *x509.Certificate, privateKey *rsa.PrivateKey, certificateTemplateSigner *x509.Certificate, privateKeySigner *rsa.PrivateKey) ([]byte, error) {
	certificate, err := x509.CreateCertificate(rand.Reader, certificateTemplate, certificateTemplateSigner, &privateKey.PublicKey, privateKeySigner)
	if err != nil {
		return nil, err
	}
	return utils.EncodeCertificate(certificate), nil
}

// TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern is a constant for the pattern used when creating a temporary
// directory for self-generated certificates.
const TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern = "self-generated-server-certificates-"

// SelfGenerateTLSServerCertificate generates a new CA certificate and signs a server certificate with it. It'll store
// the generated CA + server certificate bytes into a temporary directory with the default filenames, e.g. `DataKeyCertificateCA`.
// The function will return the *Certificate object as well as the path of the temporary directory where the
// certificates are stored.
func SelfGenerateTLSServerCertificate(name string, dnsNames []string, ips []net.IP) (cert *Certificate, ca *Certificate, dir string, rErr error) {
	tempDir, err := os.MkdirTemp("", TemporaryDirectoryForSelfGeneratedTLSCertificatesPattern)
	if err != nil {
		return nil, nil, "", err
	}

	caCertificate, err := (&CertificateSecretConfig{
		Name:       name,
		CommonName: name,
		CertType:   CACert,
	}).GenerateCertificate()
	if err != nil {
		return nil, nil, "", err
	}
	caCertificateData := caCertificate.SecretData()

	if err := os.WriteFile(filepath.Join(tempDir, DataKeyCertificateCA), caCertificateData[DataKeyCertificateCA], 0600); err != nil {
		return nil, nil, "", err
	}
	if err := os.WriteFile(filepath.Join(tempDir, DataKeyPrivateKeyCA), caCertificateData[DataKeyPrivateKeyCA], 0600); err != nil {
		return nil, nil, "", err
	}

	certificate, err := (&CertificateSecretConfig{
		Name:        name,
		CommonName:  name,
		DNSNames:    dnsNames,
		IPAddresses: ips,
		CertType:    ServerCert,
		SigningCA:   caCertificate,
	}).GenerateCertificate()
	if err != nil {
		return nil, nil, "", err
	}
	certificateData := certificate.SecretData()

	if err := os.WriteFile(filepath.Join(tempDir, DataKeyCertificate), certificateData[DataKeyCertificate], 0600); err != nil {
		return nil, nil, "", err
	}
	if err := os.WriteFile(filepath.Join(tempDir, DataKeyPrivateKey), certificateData[DataKeyPrivateKey], 0600); err != nil {
		return nil, nil, "", err
	}

	return certificate, caCertificate, tempDir, nil
}

// AdjustToClockSkew adjusts the given time by the maximum allowed clock skew as clock skew can cause non-trivial errors.
func AdjustToClockSkew(t time.Time) time.Time {
	return t.Add(-1 * allowedClockSkew)
}
