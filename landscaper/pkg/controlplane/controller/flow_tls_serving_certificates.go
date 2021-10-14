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
	"time"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	secretsutil "github.com/gardener/gardener/pkg/utils/secrets"
	"k8s.io/utils/pointer"
)

// GenerateAPIServerCertificates generates the certificates for the Gardener API server
func (o *operation) GenerateAPIServerCertificates(_ context.Context) error {
	if o.imports.GardenerAPIServer.ComponentConfiguration.TLS == nil {
		// safely dereference as the CA Bundle must either be provided or is generated in a previous step
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerAPIServer, *o.imports.GardenerAPIServer.ComponentConfiguration.CA)
		if err != nil {
			return fmt.Errorf("failed to generate TLS serving cert for the Gardener API server: %v", err)
		}
		o.imports.GardenerAPIServer.ComponentConfiguration.TLS = tlsServingCertificate
	}

	return nil
}

// GenerateControllerManagerCertificates generates the certificates for the Gardener Controller Manager
func (o *operation) GenerateControllerManagerCertificates(_ context.Context) error {
	if o.imports.GardenerControllerManager.ComponentConfiguration == nil || o.imports.GardenerControllerManager.ComponentConfiguration.TLS == nil {
		// generate using the CA of the Gardener API server
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerControllerManager, *o.imports.GardenerAPIServer.ComponentConfiguration.CA)
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
	if o.imports.GardenerAdmissionController.ComponentConfiguration == nil || o.imports.GardenerAdmissionController.ComponentConfiguration.TLS == nil {
		// generate using the CA of the Gardener Admission Controller
		tlsServingCertificate, err := o.generateTLSServingCertificate(serviceNameGardenerAdmissionController, *o.imports.GardenerAdmissionController.ComponentConfiguration.CA)
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
func (o *operation) generateTLSServingCertificate(serviceName string, ca imports.CA) (*imports.TLSServer, error) {
	// use the provided or generated private key of the Gardener CA to sign the TLS serving
	// certificate of the Gardener API server
	caCert, err := secretsutil.LoadCertificate("", []byte(*ca.Key), []byte(*ca.Crt))
	if err != nil {
		return nil, err
	}

	date := time.Now().UTC().AddDate(10, 0, 0)
	validity := date.Sub(time.Now().UTC())
	certConfig := &secretsutil.CertificateSecretConfig{
		CertType:   secretsutil.ServerCert,
		SigningCA:  caCert,
		Validity:   &validity,
		CommonName: serviceName + "." + o.namespace + ".svc.cluster.local",
		DNSNames: []string{
			serviceName,
			serviceName + "." + o.namespace,
			serviceName + "." + o.namespace + ".svc",
			serviceName + "." + o.namespace + ".svc.cluster",
			serviceName + "." + o.namespace + ".svc.cluster.local",
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
