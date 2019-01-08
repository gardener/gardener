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

package secrets

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"sync"
	"time"

	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

type certType string

const (
	// CACert indicates that the certificate should be a certificate authority.
	CACert certType = "ca"
	// ServerCert indicates that the certificate should have the ExtKeyUsageServerAuth usage.
	ServerCert certType = "server"
	// ClientCert indicates that the certificate should have the ExtKeyUsageClientAuth usage.
	ClientCert certType = "client"
	// ServerClientCert indicates that the certificate should have both the ExtKeyUsageServerAuth and ExtKeyUsageClientAuth usage.
	ServerClientCert certType = "both"

	// DataKeyCertificate is the key in a secret data holding the certificate.
	DataKeyCertificate = "tls.crt"
	// DataKeyPrivateKey is the key in a secret data holding the private key.
	DataKeyPrivateKey = "tls.key"
	// DataKeyCertificateCA is the key in a secret data holding the CA certificate.
	DataKeyCertificateCA = "ca.crt"
	// DataKeyPrivateKeyCA is the key in a secret data holding the CA private key.
	DataKeyPrivateKeyCA = "ca.key"
)

// CertificateSecretConfig contains the specification a to-be-generated CA, server, or client certificate.
// It always contains a 2048-bit RSA private key.
type CertificateSecretConfig struct {
	Name string

	CommonName   string
	Organization []string
	DNSNames     []string
	IPAddresses  []net.IP

	CertType  certType
	SigningCA *Certificate
}

// Certificate contains the private key, and the certificate. It does also contain the CA certificate
// in case it is no CA. Otherwise, the <CA> field is nil.
type Certificate struct {
	Name string

	CA *Certificate

	PrivateKey    *rsa.PrivateKey
	PrivateKeyPEM []byte

	Certificate    *x509.Certificate
	CertificatePEM []byte
}

// GetName returns the name of the secret.
func (s *CertificateSecretConfig) GetName() string {
	return s.Name
}

// Generate computes a CA, server, or client certificate based on the configuration.
func (s *CertificateSecretConfig) Generate() (Interface, error) {
	var certificate = s.generateCertificateTemplate()

	privateKey, err := generateRSAPrivateKey(2048)
	if err != nil {
		return nil, err
	}

	var (
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

	return &Certificate{
		Name: s.Name,

		CA: s.SigningCA,

		PrivateKey:    privateKey,
		PrivateKeyPEM: utils.EncodePrivateKey(privateKey),

		Certificate:    certificate,
		CertificatePEM: certificatePEM,
	}, nil
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
		// The certificate is not a CA certificate, so we add the signing CA certificate to it and use different
		// keys in the secret data.
		data[DataKeyPrivateKey] = c.PrivateKeyPEM
		data[DataKeyCertificate] = c.CertificatePEM
		data[DataKeyCertificateCA] = c.CA.CertificatePEM
	}

	return data
}

// LoadCertificate takes a byte slice representation of a certificate and the corresponding private key, and returns its de-serialized private
// key, certificate template and PEM certificate which can be used to sign other x509 certificates.
func LoadCertificate(name string, privateKeyPEM, certificatePEM []byte) (Interface, error) {
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
	var (
		serialNumber, _ = rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
		now             = time.Now()
		isCA            = s.CertType == CACert

		template = &x509.Certificate{
			BasicConstraintsValid: true,
			IsCA:                  isCA,
			SerialNumber:          serialNumber,
			NotBefore:             now,
			NotAfter:              now.AddDate(10, 0, 0), // + 10 years
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

func generateCA(k8sClusterClient kubernetes.Interface, config *CertificateSecretConfig, namespace string) (*corev1.Secret, Interface, error) {
	certificate, err := config.Generate()
	if err != nil {
		return nil, nil, err
	}

	secret, err := k8sClusterClient.CreateSecret(namespace, config.GetName(), corev1.SecretTypeOpaque, certificate.SecretData(), false)
	if err != nil {
		return nil, nil, err
	}
	return secret, certificate, nil
}

func loadCA(name string, existingSecret *corev1.Secret) (*corev1.Secret, Interface, error) {
	certificate, err := LoadCertificate(name, existingSecret.Data[DataKeyPrivateKeyCA], existingSecret.Data[DataKeyCertificateCA])
	if err != nil {
		return nil, nil, err
	}
	return existingSecret, certificate, nil
}

// GenerateCertificateAuthorities get a map of wanted cerificated and check If they exist in the existingSecretsMap based on the keys in the map. If they exist it get only the certificate from the corresponding
// existing secret and makes a certificate Interface from the existing secret. If there is no existing secret contaning the wanted certificate, we make one certificate and with it we deploy in K8s cluster
// a secret with that  certificate and then return the newly existing secret. The function returns a map of secrets contaning the wanted CA, a map with the wanted CA certificate and an error.
func GenerateCertificateAuthorities(k8sClusterClient kubernetes.Interface, existingSecretsMap map[string]*corev1.Secret, wantedCertificateAuthorities map[string]*CertificateSecretConfig, namespace string) (map[string]*corev1.Secret, map[string]*Certificate, error) {
	type caOutput struct {
		secret      *corev1.Secret
		certificate Interface
		err         error
	}

	var (
		certificateAuthorities = map[string]*Certificate{}
		generatedSecrets       = map[string]*corev1.Secret{}
		results                = make(chan *caOutput)
		wg                     sync.WaitGroup
		errorList              = []error{}
	)

	for name, config := range wantedCertificateAuthorities {
		wg.Add(1)

		if existingSecret, ok := existingSecretsMap[name]; !ok {
			go func(config *CertificateSecretConfig) {
				defer wg.Done()
				secret, certificate, err := generateCA(k8sClusterClient, config, namespace)
				results <- &caOutput{secret, certificate, err}
			}(config)
		} else {
			go func(name string, existingSecret *corev1.Secret) {
				defer wg.Done()
				secret, certificate, err := loadCA(name, existingSecret)
				results <- &caOutput{secret, certificate, err}
			}(name, existingSecret)
		}
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for out := range results {
		if out.err != nil {
			errorList = append(errorList, out.err)
			continue
		}
		generatedSecrets[out.secret.Name] = out.secret
		certificateAuthorities[out.secret.Name] = out.certificate.(*Certificate)
	}

	// Wait and check wether an error occurred during the parallel processing of the Secret creation.
	if len(errorList) > 0 {
		return nil, nil, fmt.Errorf("Errors occurred during certificate authority generation: %+v", errorList)
	}

	return generatedSecrets, certificateAuthorities, nil
}

// GenerateClusterSecrets try to deploy in the k8s cluster each secret in the wantedSecretsList. If the secret already exist it jumps to the next one.
// The function returns a map with all of the successfuly deployed wanted secrets plus those alredy deployed(only from the wantedSecretsList)
func GenerateClusterSecrets(k8sClusterClient kubernetes.Interface, existingSecretsMap map[string]*corev1.Secret, wantedSecretsList []ConfigInterface, namespace string) (map[string]*corev1.Secret, error) {
	type secretOutput struct {
		secret *corev1.Secret
		err    error
	}

	var (
		results                = make(chan *secretOutput)
		deployedClusterSecrets = map[string]*corev1.Secret{}
		wg                     sync.WaitGroup
		errorList              = []error{}
	)

	for _, s := range wantedSecretsList {
		name := s.GetName()

		if existingSecret, ok := existingSecretsMap[name]; ok {
			deployedClusterSecrets[name] = existingSecret
			continue
		}

		wg.Add(1)
		go func(s ConfigInterface) {
			defer wg.Done()

			obj, err := s.Generate()
			if err != nil {
				results <- &secretOutput{err: err}
				return
			}

			secretType := corev1.SecretTypeOpaque
			if _, isTLSSecret := s.(*CertificateSecretConfig); isTLSSecret {
				secretType = corev1.SecretTypeTLS
			}

			secret, err := k8sClusterClient.CreateSecret(namespace, s.GetName(), secretType, obj.SecretData(), false)
			results <- &secretOutput{secret: secret, err: err}
		}(s)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for out := range results {
		if out.err != nil {
			errorList = append(errorList, out.err)
			continue
		}

		deployedClusterSecrets[out.secret.Name] = out.secret
	}

	// Wait and check wether an error occurred during the parallel processing of the Secret creation.
	if len(errorList) > 0 {
		return deployedClusterSecrets, fmt.Errorf("Errors occurred during shoot secrets generation: %+v", errorList)
	}

	return deployedClusterSecrets, nil
}
