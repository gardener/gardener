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

package controller

import (
	"context"
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	secretsutil "github.com/gardener/gardener/pkg/utils/secrets"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
)

// GenerateAPIServerCertificates generates the certificates for the Gardener API server
func (o *operation) GenerateAPIServerCertificates(_ context.Context) error {
	// Make sure there is a Gardener API Server CA private key to sign the GAPI's TLS serving certificates.
	// Has to be checked after the imports API validation as
	//   - the import config can contain secret references for CA + TLS certificates which first need to be fetched during reconciliation
	//   - the CA + TLS certificates can be synced from an existing installation if not provided
	// This misconfiguration occurs when:
	//   -  the TLS serving certificate is not set (either via secret reference and cannot be found in an existing installation)
	//   -  the Gardener CA certificate is set, but not it's private key. This is either
	//         - a misconfiguration in the secret reference imports.GardenerAPIServer.ComponentConfiguration.CA.SecretRef
	//         - the TLS serving certificate for the Gardener API server has been deleted from an existing Gardener installation (there is no way for this component to re-generate it without having the CA's private key -> the private key is not stored in-cluster -> needs to be provided via the import configuration!)
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key == nil && o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt == nil {
		return fmt.Errorf("attempted to use the CA certificate for the Gardener API Server (either provided by a secret reference or synced from the existing Gardener installation (APIService)) to generate missing TLS serving certificates for the Gardener API Server. However, the private key of the CA is missing. Please provide the private key of the Gardener API Server CA via the import configuration - either directly or via secret reference. If the private key is lost, you can force to rotate all certificates")
	}

	if o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt == nil {
		// safely dereference as the CA Bundle must either be provided or is generated in a previous step
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerAPIServer, *o.imports.GardenerAPIServer.ComponentConfiguration.CA, *o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Validity)
		if err != nil {
			return fmt.Errorf("failed to generate TLS serving cert for the Gardener API server: %v", err)
		}
		o.imports.GardenerAPIServer.ComponentConfiguration.TLS = tlsServingCertificate
	}

	return nil
}

// GenerateControllerManagerCertificates generates the certificates for the Gardener Controller Manager
func (o *operation) GenerateControllerManagerCertificates(_ context.Context) error {
	// we use the Gardener API server CA to generate the TLS serving certs for the GCM.
	// Hence, we need to make we have the CA's private key.
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA != nil && o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key == nil && o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt == nil {
		return fmt.Errorf("attempted to use the CA certificate for the Gardener API Server (either provided by a secret reference or synced from the existing Gardener installation (APIService)) to generate missing TLS serving certificates for the Gardener Controller Manager. However, the private key of the CA is missing. Please provide the private key of the Gardener API Server CA via the import configuration - either directly or via secret reference. If the private key is lost, you can either not supply the CA at all (will be re-generated including its dependent TLS certs), or force to rotate all certificates")
	}

	if o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt == nil {
		// generate using the CA of the Gardener API server
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerControllerManager, *o.imports.GardenerAPIServer.ComponentConfiguration.CA, *o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Validity)
		if err != nil {
			return fmt.Errorf("failed to generate TLS serving cert for the Gardener Controller Manager: %v", err)
		}

		if o.imports.GardenerControllerManager.ComponentConfiguration == nil {
			o.imports.GardenerControllerManager.ComponentConfiguration = &imports.ControllerManagerComponentConfiguration{}
		}

		o.imports.GardenerControllerManager.ComponentConfiguration.TLS = tlsServingCertificate
	}

	return nil
}

// GenerateAdmissionControllerCertificates generates the certificates for the Gardener Admission Controller
func (o *operation) GenerateAdmissionControllerCertificates(_ context.Context) error {
	// we use the Gardener Admission Controller's CA to generate the TLS serving certs for the GCM.
	// Hence, we need to make we have the CA's private key.
	if o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Key == nil && o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt == nil {
		return fmt.Errorf("attempted to use the CA certificate for the Gardener Admission Controller (either provided by a secret reference or synced from the existing Gardener installation) to generate missing TLS serving certificates for the Gardener Admission Controller. However, the private key of the CA is missing. Please provide the private key of the Gardener Admission Controller CA via the import configuration - either directly or via secret reference. If the private key is lost, you can either not supply the CA at all (will be re-generated including its dependent TLS certs), or force to rotate all certificates")
	}

	if o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt == nil {
		// generate using the CA of the Gardener Admission Controller
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerAdmissionController, *o.imports.GardenerAdmissionController.ComponentConfiguration.CA, *o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Validity)
		if err != nil {
			return fmt.Errorf("failed to generate TLS serving cert for the Gardener Admission Controller: %v", err)
		}

		if o.imports.GardenerAdmissionController.ComponentConfiguration == nil {
			o.imports.GardenerAdmissionController.ComponentConfiguration = &imports.AdmissionControllerComponentConfiguration{}
		}

		o.imports.GardenerAdmissionController.ComponentConfiguration.TLS = tlsServingCertificate
	}

	return nil
}

// generateTLSServingCertificate generates a TLS serving certificate public and private key pair
func (o *operation) generateTLSServingCertificate(serviceName string, ca imports.CA, validity metav1.Duration) (*imports.TLSServer, error) {
	caCert, err := secretsutil.LoadCertificate("", []byte(*ca.Key), []byte(*ca.Crt))
	if err != nil {
		return nil, err
	}

	certConfig := &secretsutil.CertificateSecretConfig{
		CertType:   secretsutil.ServerCert,
		SigningCA:  caCert,
		Validity:   &validity.Duration,
		CommonName: serviceName + ".garden.svc.cluster.local",
		DNSNames: []string{
			serviceName,
			serviceName + ".garden",
			serviceName + ".garden.svc",
			serviceName + ".garden.svc.cluster",
			serviceName + ".garden.svc.cluster.local",
		},
	}

	tlsServingCert, err := certConfig.GenerateCertificate()
	if err != nil {
		return nil, err
	}

	return &imports.TLSServer{
		Crt: pointer.String(string(tlsServingCert.CertificatePEM)),
		Key: pointer.String(string(tlsServingCert.PrivateKeyPEM)),
	}, nil
}
