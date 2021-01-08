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

	"github.com/sirupsen/logrus"
	certificatesv1 "k8s.io/api/certificates/v1"
	kubernetesclientset "k8s.io/client-go/kubernetes"
	certutil "k8s.io/client-go/util/cert"
	"k8s.io/client-go/util/certificate/csr"
	"k8s.io/client-go/util/keyutil"

	bootstraputil "github.com/gardener/gardener/pkg/gardenlet/bootstrap/util"
)

// RequestCertificate will create a certificate signing request for the Gardenlet
// and send it to API server, then it will watch the object's
// status, once approved by the gardener-controller-manager, it will return the kube-controller-manager's issued
// certificate (pem-encoded). If there is any errors, or the watch timeouts, it
// will return an error.
func RequestCertificate(ctx context.Context, logger logrus.FieldLogger, client kubernetesclientset.Interface, certificateSubject *pkix.Name, dnsSANs []string, ipSANs []net.IP) ([]byte, []byte, string, error) {
	if certificateSubject == nil || len(certificateSubject.CommonName) == 0 {
		return nil, nil, "", fmt.Errorf("unable to request certificate. The Common Name (CN) of the of the certificate Subject has to be set")
	}

	privateKeyData, err := keyutil.MakeEllipticPrivateKeyPEM()
	if err != nil {
		return nil, nil, "", fmt.Errorf("error generating client certificate private key: %v", err)
	}

	certData, csrName, err := requestCertificate(ctx, logger, client, privateKeyData, certificateSubject, dnsSANs, ipSANs)
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
func requestCertificate(ctx context.Context, logger logrus.FieldLogger, client kubernetesclientset.Interface, privateKeyData []byte, certificateSubject *pkix.Name, dnsSANs []string, ipSANs []net.IP) (certData []byte, csrName string, err error) {
	privateKey, err := keyutil.ParsePrivateKeyPEM(privateKeyData)
	if err != nil {
		return nil, "", fmt.Errorf("invalid private key for certificate request: %v", err)
	}
	csrData, err := certutil.MakeCSR(privateKey, certificateSubject, dnsSANs, ipSANs)
	if err != nil {
		return nil, "", fmt.Errorf("unable to generate certificate request: %v", err)
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

	name, err := bootstraputil.DigestedName(signer.Public(), certificateSubject, usages)
	if err != nil {
		return nil, "", err
	}

	logger.Info("Creating certificate signing request...")

	// TODO (timebertt): figure out correct signerName for gardenlet CSRs
	// kubernetes.io/legacy-unknown is disallowed when creating new CSR objects in certificates/v1,
	// ref https://github.com/kubernetes/enhancements/tree/master/keps/sig-auth/1513-certificate-signing-request#api-changes-between-v1beta1-and-v1
	reqName, reqUID, err := csr.RequestCertificate(client, csrData, name, certificatesv1.KubeAPIServerClientSignerName, usages, privateKey)
	if err != nil {
		return nil, "", err
	}

	childCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	logger.Infof("Waiting for certificate signing request %q to be approved and contain the client certificate ...", reqName)

	certData, err = csr.WaitForCertificate(childCtx, client, reqName, reqUID)
	if err != nil {
		return nil, "", err
	}

	logger.Infof("Certificate signing request got approved. Retrieved client certificate!")

	return certData, reqName, nil
}
