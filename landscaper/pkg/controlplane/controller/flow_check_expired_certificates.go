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
	"time"

	landscaperutils "github.com/gardener/gardener/landscaper/common/utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// certificateRotationThresholdPercentage is the threshold marking when x509 certificates should be rotated.
// For example: 0.8 means that a certificate should be rotated after 80% of its lifetime / validity.
const certificateRotationThresholdPercentage = 0.8

// CheckForExpiringCertificates checks the CA & TLS certificates in the import configuration for expiration
// Does not check etcd certificates as the lifecycle of those certificates is not controlled by this component.
// Deletes (and thus regenerates in a later step) dependent TLS certificates of expiring certificates
// Please note, this checks the validity of certificates independent of their origin
//   - detected from an existing installation
//   - supplied by secret reference
//   - supplied by import file
// This means that also secret references are updated with the rotated certificates!
func (o *operation) CheckForExpiringCertificates(ctx context.Context) error {
	// Gardener API Server CA
	if o.imports.GardenerAPIServer.ComponentConfiguration.CA != nil && o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt != nil {
		cert, err := landscaperutils.ParseX509Certificate(*o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt)
		if err != nil {
			return err
		}

		needsRenewal, nextRenewal := landscaperutils.CertificateNeedsRenewal(cert, certificateRotationThresholdPercentage)
		if needsRenewal {
			// regenerate the TLS certificates for the Gardener API Server
			o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = nil
			o.exports.GardenerAPIServerTLSServing.Rotated = true
			o.log.Infof("Gardener API server TLS certificate needs to be regenerated. Reason: Gardener API Server CA certificate will be rotated")

			// regenerate the TLS certificates for the GCM
			if o.imports.GardenerControllerManager.ComponentConfiguration != nil && o.imports.GardenerControllerManager.ComponentConfiguration.TLS != nil && o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt != nil {
				if errors := validation.ValidateTLSServingCertificateAgainstCA(*o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt, *o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt, field.NewPath("")); len(errors) == 0 {
					// the GCM's TLS serving certificates are signed by the Gardener API Server CA - we also need to regenerate it
					o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = nil
					o.exports.GardenerControllerManagerTLSServing.Rotated = true
					o.log.Infof("Gardener Controller Manager TLS certificate needs to be regenerated. Reason: Gardener API Server CA certificate will be rotated")
				}
			}

			o.imports.GardenerAPIServer.ComponentConfiguration.CA.Crt = nil
			o.exports.GardenerAPIServerCA.Rotated = true
			o.log.Infof("Gardener API server CA certificate needs to be regenerated. Reason: %d % of certificate's lifetime exceeded", certificateRotationThresholdPercentage)
		} else {
			o.log.Infof("Next Gardener API Server CA rotation is in %s", nextRenewal.Round(time.Hour).String())
		}
	}

	// Gardener Admission Controller CA
	if o.imports.GardenerAdmissionController.Enabled && (o.imports.GardenerAdmissionController.ComponentConfiguration != nil && o.imports.GardenerAdmissionController.ComponentConfiguration.CA != nil && o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt != nil) {
		cert, err := landscaperutils.ParseX509Certificate(*o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt)
		if err != nil {
			return err
		}

		needsRenewal, nextRenewal := landscaperutils.CertificateNeedsRenewal(cert, certificateRotationThresholdPercentage)
		if needsRenewal {
			o.imports.GardenerAdmissionController.ComponentConfiguration.CA.Crt = nil
			o.exports.GardenerAdmissionControllerCA.Rotated = true
			o.log.Infof("Gardener Admission Controller CA certificate needs to be regenerated. Reason: %d % of certificate's lifetime exceeded", certificateRotationThresholdPercentage)

			// regenerate the TLS certificates for the Gardener Admission Controller
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = nil
			o.exports.GardenerAdmissionControllerTLSServing.Rotated = true
			o.log.Infof("Gardener Admission Controller TLS certificate needs to be regenerated. Reason: Gardener Admission Controller CA certificate will be rotated")
		} else {
			o.log.Infof("Next Admission Controller CA rotation is in %s", nextRenewal.Round(time.Hour).String())
		}
	}

	// check validity of TLS certificates
	if o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt != nil {
		cert, err := landscaperutils.ParseX509Certificate(*o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt)
		if err != nil {
			return err
		}

		needsRenewal, nextRenewal := landscaperutils.CertificateNeedsRenewal(cert, certificateRotationThresholdPercentage)
		if needsRenewal {
			// regenerate the TLS certificates for the Gardener API Server
			o.imports.GardenerAPIServer.ComponentConfiguration.TLS.Crt = nil
			o.exports.GardenerAPIServerTLSServing.Rotated = true
			o.log.Infof("Gardener API Server TLS certificate needs to be regenerated. Reason: %d % of certificate's lifetime exceeded", certificateRotationThresholdPercentage)
		} else {
			o.log.Infof("Next Gardener API Server TLS certificate rotation is in %s", nextRenewal.Round(time.Hour).String())
		}
	}

	if o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt != nil {
		cert, err := landscaperutils.ParseX509Certificate(*o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt)
		if err != nil {
			return err
		}

		needsRenewal, nextRenewal := landscaperutils.CertificateNeedsRenewal(cert, certificateRotationThresholdPercentage)
		if needsRenewal {
			// regenerate the TLS certificates for the Gardener Controller Manager
			o.imports.GardenerControllerManager.ComponentConfiguration.TLS.Crt = nil
			o.exports.GardenerControllerManagerTLSServing.Rotated = true
			o.log.Infof("Gardener Controller Manager TLS certificate needs to be regenerated. Reason: %d % of certificate's lifetime exceeded", certificateRotationThresholdPercentage)
		} else {
			o.log.Infof("Next Gardener Controller Manager TLS certificate rotation is in %s", nextRenewal.Round(time.Hour).String())
		}
	}

	if o.imports.GardenerAdmissionController.Enabled && o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt != nil {
		cert, err := landscaperutils.ParseX509Certificate(*o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt)
		if err != nil {
			return err
		}

		needsRenewal, nextRenewal := landscaperutils.CertificateNeedsRenewal(cert, certificateRotationThresholdPercentage)
		if needsRenewal {
			// regenerate the TLS certificates for the Gardener Admission Controller
			o.imports.GardenerAdmissionController.ComponentConfiguration.TLS.Crt = nil
			o.exports.GardenerAdmissionControllerTLSServing.Rotated = true
			o.log.Infof("Gardener Admission Controller TLS certificate needs to be regenerated. Reason: %d % of certificate's lifetime exceeded", certificateRotationThresholdPercentage)
		} else {
			o.log.Infof("Next Gardener Gardener Admission Controller TLS certificate rotation is in %s", nextRenewal.Round(time.Hour).String())
		}
	}

	return nil
}
