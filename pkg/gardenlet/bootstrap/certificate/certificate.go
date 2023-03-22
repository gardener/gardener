// Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package certificate

import (
	"context"
	"crypto"
	"crypto/x509/pkix"
	"fmt"
	"net"
	"time"

	"github.com/go-logr/logr"
	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	csrutil "k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
	"k8s.io/utils/pointer"

	gardenletbootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
	"github.com/gardener/gardener/pkg/utils/retry"
)

// RequestCertificate will create a certificate signing request for the Gardenlet
// and send it to API server, then it will watch the object's
// status, once approved by the gardener-controller-manager, it will return the kube-controller-manager's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func RequestCertificate(
	ctx context.Context,
	log logr.Logger,
	client kubernetesclientset.Interface,
	certificateSubject *pkix.Name,
	dnsSANs []string,
	ipSANs []net.IP,
	validityDuration *metav1.Duration,
) (
	[]byte,
	[]byte,
	string,
	error,
) {
	if certificateSubject == nil || len(certificateSubject.CommonName) == 0 {
		return nil, nil, "", fmt.Errorf("unable to request certificate. The Common Name (CN) of the of the certificate Subject has to be set")
	}

	privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error generating client certificate private key: %w", err)
	}

	certData, csrName, err := requestCertificate(ctx, log, client, privateKeyData, certificateSubject, dnsSANs, ipSANs, validityDuration)
	if err != nil {
		return nil, nil, "", err
	}
	return certData, privateKeyData, csrName, nil
}

// DigestedName is an alias for gardenletbootstraputil.DigestedName.
// Exposed for testing.
var DigestedName = gardenletbootstraputil.DigestedName

// requestCertificate will create a certificate signing request for the Gardenlet
// and send it to API server, then it will watch the object's
// status, once approved by the gardener-controller-manager, it will return the kube-controller-manager's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func requestCertificate(
	ctx context.Context,
	log logr.Logger,
	client kubernetesclientset.Interface,
	privateKeyData []byte,
	certificateSubject *pkix.Name,
	dnsSANs []string,
	ipSANs []net.IP,
	validityDuration *metav1.Duration,
) (
	certData []byte,
	csrName string,
	err error,
) {
	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, "", fmt.Errorf("invalid private key for certificate request: %w", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate certificate request: %w", err)
	}

	usages := []certificatesv1.KeyUsage{
		certificatesv1.UsageDigitalSignature,
		certificatesv1.UsageKeyEncipherment,
		certificatesv1.UsageClientAuth,
	}

	// The Signer interface contains the Public() method to get the public key.
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, "", fmt.Errorf("private key does not implement crypto.Signer")
	}

	name, err := DigestedName(signer.Public(), certificateSubject, usages)
	if err != nil {
		return nil, "", err
	}

	var requestedDuration *time.Duration
	if validityDuration != nil {
		requestedDuration = pointer.Duration(validityDuration.Duration)
	}

	log = log.WithValues("certificateSigningRequestName", name)
	log.Info("Creating certificate signing request")

	reqName, reqUID, err := csrutil.RequestCertificate(client, csrData, name, certificatesv1.KubeAPIServerClientSignerName, requestedDuration, usages, privateKey)
	if err != nil {
		return nil, "", err
	}

	childCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	log.Info("Waiting for certificate signing request to be approved and contain the client certificate")

	certData, err = waitForCertificate(childCtx, client, reqName, reqUID)
	if err != nil {
		return nil, "", err
	}

	log.Info("Certificate signing request got approved. Retrieved client certificate")

	return certData, reqName, nil
}

// waitForCertificate is heavily inspired from k8s.io/client-go/util/certificate/csr.WaitForCertificate. We don't call
// this function directly because it performs LIST/WATCH requests while waiting for the certificate. However, gardenlet
// is only allowed to GET CSR resources related to its seed.
func waitForCertificate(ctx context.Context, client kubernetesclientset.Interface, reqName string, reqUID types.UID) ([]byte, error) {
	var certData []byte

	if err := retry.Until(ctx, 2*time.Second, func(ctx context.Context) (done bool, err error) {
		if csr, err := client.CertificatesV1().CertificateSigningRequests().Get(ctx, reqName, metav1.GetOptions{}); err == nil {
			if csr.UID != reqUID {
				return retry.SevereError(fmt.Errorf("csr %q changed UIDs", csr.Name))
			}

			approved := false
			for _, c := range csr.Status.Conditions {
				if c.Type == certificatesv1.CertificateDenied {
					return retry.SevereError(fmt.Errorf("certificate signing request is denied, reason: %v, message: %v", c.Reason, c.Message))
				}
				if c.Type == certificatesv1.CertificateFailed {
					return retry.SevereError(fmt.Errorf("certificate signing request failed, reason: %v, message: %v", c.Reason, c.Message))
				}
				if c.Type == certificatesv1.CertificateApproved {
					approved = true
				}
			}

			if !approved {
				return retry.MinorError(fmt.Errorf("certificate signing request %s is not yet approved, waiting", csr.Name))
			}

			if len(csr.Status.Certificate) == 0 {
				return retry.MinorError(fmt.Errorf("certificate signing request %s is approved, waiting to be issued", csr.Name))
			}

			certData = csr.Status.Certificate
			return retry.Ok()
		}

		return retry.SevereError(fmt.Errorf("certificates.k8s.io/v1 requests not succeeded"))
	}); err != nil {
		return nil, err
	}

	return certData, nil
}
