// Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"

	"github.com/sirupsen/logrus"
	certificatesv1beta1 "k8s.io/api/certificates/v1beta1"
	certificatesv1beta1client "k8s.io/client-go/kubernetes/typed/certificates/v1beta1"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"
)

// RequestCertificate will create a certificate signing request for the Gardenlet
// and send it to API server, then it will watch the object's
// status, once approved by the gardener-controller-manager, it will return the kube-controller-manager's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func RequestCertificate(ctx context.Context, logger logrus.FieldLogger, certClient certificatesv1beta1client.CertificatesV1beta1Interface, certificateSubject *pkix.Name, dnsSANs []string, ipSANs []net.IP) ([]byte, []byte, string, error) {
	if certificateSubject == nil || len(certificateSubject.CommonName) == 0 {
		return nil, nil, "", fmt.Errorf("unable to request certificate. The Common Name (CN) of the of the certificate Subject has to be set")
	}

	privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error generating client certificate private key: %v", err)
	}

	certData, csrName, err := requestCertificate(ctx, logger, certClient.CertificateSigningRequests(), privateKeyData, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return nil, nil, "", err
	}
	return certData, privateKeyData, csrName, nil
}

// requestCertificate will create a certificate signing request for the Gardenlet
// and send it to API server, then it will watch the object's
// status, once approved by the gardener-controller-manager, it will return the kube-controller-manager's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func requestCertificate(ctx context.Context, logger logrus.FieldLogger, certificateClient certificatesv1beta1client.CertificateSigningRequestInterface, privateKeyData []byte, certificateSubject *pkix.Name, dnsSANs []string, ipSANs []net.IP) (certData []byte, csrName string, err error) {
	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, "", fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate certificate request: %v", err)
	}

	usages := []certificatesv1beta1.KeyUsage{
		certificatesv1beta1.UsageDigitalSignature,
		certificatesv1beta1.UsageKeyEncipherment,
		certificatesv1beta1.UsageClientAuth,
	}

	// The Signer interface contains the Public() method to get the public key.
	signer, ok := privateKey.(crypto.Signer)
	if !ok {
		return nil, "", fmt.Errorf("private key does not implement crypto.Signer")
	}

	name, err := bootstraputil.DigestedName(signer.Public(), certificateSubject, usages)
	if err != nil {
		return nil, "", err
	}

	logger.Info("Creating certificate signing request...")

	// LegacyUnkownSignerName based on https://github.com/kubernetes/kubernetes/blob/db4ca87d9d872b3c31df58860bac996c70df6b5b/pkg/apis/certificates/v1beta1/defaults.go#L52.
	req, err := csr.RequestCertificate(certificateClient, csrData, name, certificatesv1beta1.LegacyUnknownSignerName, usages, privateKey)
	if err != nil {
		return nil, "", err
	}

	childCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	logger.Infof("Waiting for certificate signing request %q to be approved and contain the client certificate ...", req.Name)

	certData, err = csr.WaitForCertificate(childCtx, certificateClient, req)
	if err != nil {
		return nil, "", err
	}

	logger.Infof("Certificate signing request got approved. Retrieved client certificate!")

	return certData, req.Name, nil
}
