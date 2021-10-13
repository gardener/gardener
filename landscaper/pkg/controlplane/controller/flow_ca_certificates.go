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
	"github.com/sirupsen/logrus"
	"k8s.io/utils/pointer"
)

// GenerateCACertificates fills missing CA certificates in the import configuration.
// Fetches existing CA certificates from the virtual-garden cluster or generates new CA certificates.
func (o *operation) GenerateCACertificates(ctx context.Context) error {
	// Gardener API Server CA
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA == nil {
		publicKeyBytes, privateKeyBytes, err := generateCACertificate(o.log, commonNameGardenerCA, 5)
		if err != nil {
			return err
		}

		o.imports.GardenerAPIServer.ComponentConfiguration.CA = &imports.CA{
			Crt: pointer.String(string(publicKeyBytes)),
		}

		// private key of the Gardener CA is only required to sign & generate new TLS serving certificates for the Gardener API server.
		// If those are already provided, we do not need the private CA key.
		if privateKeyBytes != nil {
			o.imports.GardenerAPIServer.ComponentConfiguration.CA.Key = pointer.String(string(privateKeyBytes))
		}
	}

	// Gardener Admission Controller CA
	if o.imports.GardenerAdmissionController.Enabled &&
		(o.imports.GardenerAdmissionController.ComponentConfiguration == nil || o.imports.GardenerAdmissionController.ComponentConfiguration.CA == nil) {
		publicKeyBytes, privateKeyBytes, err := o.generateAdmissionControllerCA()
		if err != nil {
			return err
		}

		if o.imports.GardenerAdmissionController.ComponentConfiguration == nil {
			o.imports.GardenerAdmissionController.ComponentConfiguration = &imports.AdmissionControllerComponentConfiguration{}
		}

		o.imports.GardenerAdmissionController.ComponentConfiguration.CA = &imports.CA{
			Crt: pointer.String(string(publicKeyBytes)),
			Key: pointer.String(string(privateKeyBytes)),
		}
	}

	return nil
}

// generateAdmissionControllerCA generates a new CA for the Gardener Admission Controller
// returns the public & private key of the CA or an error
func (o *operation) generateAdmissionControllerCA() ([]byte, []byte, error) {
	// NOTE: we can only safely generate a new CA Bundle when there is no already existing Gardener Admission Controller deployed in the runtime cluster.
	// API validation makes sure the TLS serving cert and key of the Gardener Admission Controller are provided together with the corresponding CABundle.
	// This is important, as we cannot generate a new CA Bundle for existing TLS serving certs (validating the serving certs with the new CA bundle in the Webhook configuration would fail).
	// Hence, the following assumes the AdmissionController's TLS serving certs do not exist yet.
	// We can safely generate a new CA bundle for the Gardener Admission Controller.

	// Additionally, we do not try to obtain the Gardener Admission Controller's CA bundle from an existing webhook
	// configuration in the virtual garden cluster.
	// Therefore, the edge case that the Gardener Admission Controller is deleted from the runtime cluster,
	// but its webhook configurations including its CA are still deployed in the virtual garden, is not handled.
	return generateCACertificate(o.log, commonNameGardenerAdmissionController, 5)
}

// generateCACertificate generates a CA certificate and returns the PEM-encode certificate and private key
func generateCACertificate(log logrus.FieldLogger, commonName string, yearsValidity int) ([]byte, []byte, error) {
	var caCertificate *secretsutil.Certificate
	date := time.Now().UTC().AddDate(yearsValidity, 0, 0)
	validity := date.Sub(time.Now().UTC())
	apiServerCABundle := secretsutil.CertificateSecretConfig{
		Name:       commonName,
		CertType:   secretsutil.CACert,
		SigningCA:  caCertificate,
		CommonName: commonName,
		Validity:   &validity,
	}
	certificate, err := apiServerCABundle.GenerateCertificate()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA certificate for CN %q: %v", commonName, err)
	}
	log.Infof("Successfully generated a new CA certificate for CN %q", commonName)
	return certificate.CertificatePEM, certificate.PrivateKeyPEM, nil
}
