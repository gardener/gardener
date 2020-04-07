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
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// ModeService is a constant for the webhook mode indicating that the controller is running inside of the Kubernetes cluster it
	// is serving.
	ModeService = "service"
	// ModeURL is a constant for the webhook mode indicating that the controller is running outside of the Kubernetes cluster it
	// is serving. If this is set then a URL is required for configuration.
	ModeURL = "url"
	// ModeURLWithServiceName is a constant for the webhook mode indicating that the controller is running outside of the Kubernetes cluster it
	// is serving but in the same cluster like the kube-apiserver. If this is set then a URL is required for configuration.
	ModeURLWithServiceName = "url-service"

	certSecretName = "gardener-extension-webhook-cert"
)

// GenerateCertificates generates the certificates that are required for a webhook. It returns the ca bundle, and it
// stores the server certificate and key locally on the file system.
func GenerateCertificates(ctx context.Context, mgr manager.Manager, certDir, namespace, name, mode, url string) ([]byte, error) {
	var (
		caCert     *secrets.Certificate
		serverCert *secrets.Certificate
		err        error
	)

	// If the namespace is not set then the webhook controller is running locally. We simply generate a new certificate in this case.
	if len(namespace) == 0 {
		caCert, serverCert, err = generateNewCAAndServerCert(mode, namespace, name, url)
		if err != nil {
			return nil, errors.Wrapf(err, "error generating new certificates for webhook server")
		}
		return writeCertificates(certDir, caCert, serverCert)
	}

	// The controller stores the generated webhook certificate in a secret in the cluster. It tries to read it. If it does not exist a
	// new certificate is generated.
	c, err := getClient(mgr)
	if err != nil {
		return nil, err
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, kutil.Key(namespace, certSecretName), secret); err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, errors.Wrapf(err, "error getting cert secret")
		}

		// The secret was not found, let's generate new certificates and store them in the secret afterwards.
		caCert, serverCert, err = generateNewCAAndServerCert(mode, namespace, name, url)
		if err != nil {
			return nil, errors.Wrapf(err, "error generating new certificates for webhook server")
		}

		secret.ObjectMeta = metav1.ObjectMeta{Namespace: namespace, Name: certSecretName}
		secret.Type = corev1.SecretTypeOpaque
		secret.Data = map[string][]byte{
			secrets.DataKeyCertificateCA: caCert.CertificatePEM,
			secrets.DataKeyPrivateKeyCA:  caCert.PrivateKeyPEM,
			secrets.DataKeyCertificate:   serverCert.CertificatePEM,
			secrets.DataKeyPrivateKey:    serverCert.PrivateKeyPEM,
		}
		if err := c.Create(ctx, secret); err != nil {
			return nil, err
		}

		return writeCertificates(certDir, caCert, serverCert)
	}

	// The secret has been found and we are now trying to read the stored certificate inside it.
	caCert, serverCert, err = loadExistingCAAndServerCert(secret.Data)
	if err != nil {
		return nil, errors.Wrapf(err, "error reading data of secret %s/%s", namespace, certSecretName)
	}
	return writeCertificates(certDir, caCert, serverCert)
}

func generateNewCAAndServerCert(mode, namespace, name, url string) (*secrets.Certificate, *secrets.Certificate, error) {
	caConfig := &secrets.CertificateSecretConfig{
		CommonName: "webhook-ca",
		CertType:   secrets.CACert,
	}

	caCert, err := caConfig.GenerateCertificate()
	if err != nil {
		return nil, nil, err
	}

	var (
		dnsNames    []string
		ipAddresses []net.IP
	)

	switch mode {
	case ModeURL:
		if addr := net.ParseIP(url); addr != nil {
			ipAddresses = []net.IP{
				addr,
			}
		} else {
			dnsNames = []string{
				url,
			}
		}

	case ModeService:
		dnsNames = []string{
			fmt.Sprintf("gardener-extension-%s", name),
			fmt.Sprintf("gardener-extension-%s.%s", name, namespace),
			fmt.Sprintf("gardener-extension-%s.%s.svc", name, namespace),
		}
	}

	serverConfig := &secrets.CertificateSecretConfig{
		CommonName:  name,
		DNSNames:    dnsNames,
		IPAddresses: ipAddresses,
		CertType:    secrets.ServerCert,
		SigningCA:   caCert,
	}

	serverCert, err := serverConfig.GenerateCertificate()
	if err != nil {
		return nil, nil, err
	}

	return caCert, serverCert, nil
}

func loadExistingCAAndServerCert(data map[string][]byte) (*secrets.Certificate, *secrets.Certificate, error) {
	secretDataCACert, ok := data[secrets.DataKeyCertificateCA]
	if !ok {
		return nil, nil, fmt.Errorf("secret does not contain %s key", secrets.DataKeyCertificateCA)
	}
	secretDataCAKey, ok := data[secrets.DataKeyPrivateKeyCA]
	if !ok {
		return nil, nil, fmt.Errorf("secret does not contain %s key", secrets.DataKeyPrivateKeyCA)
	}
	caCert, err := secrets.LoadCertificate("", secretDataCAKey, secretDataCACert)
	if err != nil {
		return nil, nil, fmt.Errorf("could not load ca certificate")
	}

	secretDataServerCert, ok := data[secrets.DataKeyCertificate]
	if !ok {
		return nil, nil, fmt.Errorf("secret does not contain %s key", secrets.DataKeyCertificate)
	}
	secretDataServerKey, ok := data[secrets.DataKeyPrivateKey]
	if !ok {
		return nil, nil, fmt.Errorf("secret does not contain %s key", secrets.DataKeyPrivateKey)
	}
	serverCert, err := secrets.LoadCertificate("", secretDataServerKey, secretDataServerCert)
	if err != nil {
		return nil, nil, fmt.Errorf("could not load server certificate")
	}

	return caCert, serverCert, nil
}

func writeCertificates(certDir string, caCert, serverCert *secrets.Certificate) ([]byte, error) {
	var (
		serverKeyPath  = filepath.Join(certDir, secrets.DataKeyPrivateKey)
		serverCertPath = filepath.Join(certDir, secrets.DataKeyCertificate)
	)

	if err := os.MkdirAll(certDir, 0755); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(serverKeyPath, serverCert.PrivateKeyPEM, 0666); err != nil {
		return nil, err
	}
	if err := ioutil.WriteFile(serverCertPath, serverCert.CertificatePEM, 0666); err != nil {
		return nil, err
	}

	return caCert.CertificatePEM, nil
}

func getClient(mgr manager.Manager) (client.Client, error) {
	s := runtime.NewScheme()
	if err := scheme.AddToScheme(s); err != nil {
		return nil, err
	}

	return client.New(mgr.GetConfig(), client.Options{Scheme: s})
}
