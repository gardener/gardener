// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package certificates

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/extensions/pkg/webhook"
	secretsutils "github.com/gardener/gardener/pkg/utils/secrets"
)

// GenerateUnmanagedCertificates generates a one-off CA and server cert for a webhook server. The server certificate and
// key are written to certDir. This is useful for local development.
func GenerateUnmanagedCertificates(providerName, certDir, mode, url string) ([]byte, error) {
	caConfig := getWebhookCAConfig(providerName)
	// we want to use a long validity here, because we don't auto-renew certificates
	caConfig.Validity = ptr.To(10 * 365 * 24 * time.Hour) // 10y

	caCert, err := caConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	serverConfig := getWebhookServerCertConfig(providerName, "", providerName, mode, url)
	serverConfig.SigningCA = caCert

	serverCert, err := serverConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	return caCert.CertificatePEM, writeCertificatesToDisk(certDir, serverCert.CertificatePEM, serverCert.PrivateKeyPEM)
}

var caCertificateValidity = 30 * 24 * time.Hour // 30d

func getWebhookCAConfig(name string) *secretsutils.CertificateSecretConfig {
	return &secretsutils.CertificateSecretConfig{
		Name:       name,
		CommonName: name,
		CertType:   secretsutils.CACert,
		Validity:   &caCertificateValidity,
	}
}

func getWebhookServerCertConfig(name, namespace, componentName, mode, url string) *secretsutils.CertificateSecretConfig {
	var (
		dnsNames    []string
		ipAddresses []net.IP

		serverName = url
	)

	if host, _, err := net.SplitHostPort(url); err == nil {
		serverName = host
	}

	switch mode {
	case webhook.ModeURL:
		if addr := net.ParseIP(serverName); addr != nil {
			ipAddresses = []net.IP{addr}
		} else {
			dnsNames = []string{serverName}
		}

	case webhook.ModeService:
		dnsNames = []string{webhook.PrefixedName(componentName)}
		if namespace != "" {
			dnsNames = append(dnsNames,
				fmt.Sprintf("%s.%s", webhook.PrefixedName(componentName), namespace),
				fmt.Sprintf("%s.%s.svc", webhook.PrefixedName(componentName), namespace),
			)
		}
	}

	return &secretsutils.CertificateSecretConfig{
		Name:                        name,
		CommonName:                  componentName,
		DNSNames:                    dnsNames,
		IPAddresses:                 ipAddresses,
		CertType:                    secretsutils.ServerCert,
		SkipPublishingCACertificate: true,
	}
}

func writeCertificatesToDisk(certDir string, serverCert, serverKey []byte) error {
	var (
		serverKeyPath  = filepath.Join(certDir, secretsutils.DataKeyPrivateKey)
		serverCertPath = filepath.Join(certDir, secretsutils.DataKeyCertificate)
	)

	if err := os.MkdirAll(certDir, 0750); err != nil {
		return err
	}
	if err := os.WriteFile(serverKeyPath, serverKey, 0600); err != nil {
		return err
	}
	return os.WriteFile(serverCertPath, serverCert, 0600)
}
