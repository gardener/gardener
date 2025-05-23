// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package health

import (
	"fmt"

	certv1alpha1 "github.com/gardener/cert-management/pkg/apis/cert/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

// CheckCertificate checks whether the given certificate object is healthy.
func CheckCertificate(cert *certv1alpha1.Certificate) error {
	for _, condition := range cert.Status.Conditions {
		if condition.Type == certv1alpha1.CertificateConditionReady {
			if err := checkConditionState(condition.Type, string(corev1.ConditionTrue), string(condition.Status), condition.Reason, condition.Message); err != nil {
				return err
			}

			break
		}
	}

	if certState := cert.Status.State; certState != certv1alpha1.StateReady {
		return fmt.Errorf("certificate state is %q (%q expected)", certState, certv1alpha1.StateReady)
	}
	return nil
}

// IsCertificateProgressing returns false if the Certificate's generation matches the observed generation.
func IsCertificateProgressing(cert *certv1alpha1.Certificate) (bool, string) {
	if cert.Status.ObservedGeneration < cert.Generation {
		return true, fmt.Sprintf("observed generation outdated (%d/%d)", cert.Status.ObservedGeneration, cert.Generation)
	}

	return false, "Certificate is fully rolled out"
}

// CheckCertificateIssuer checks whether the given issuer object is healthy.
func CheckCertificateIssuer(issuer *certv1alpha1.Issuer) error {
	if issuerState := issuer.Status.State; issuerState != certv1alpha1.StateReady {
		return fmt.Errorf("issuer state is %q (%q expected)", issuerState, certv1alpha1.StateReady)
	}

	return nil
}

// IsCertificateIssuerProgressing returns false if the Issuer's generation matches the observed generation.
func IsCertificateIssuerProgressing(issuer *certv1alpha1.Issuer) (bool, string) {
	if issuer.Status.ObservedGeneration < issuer.Generation {
		return true, fmt.Sprintf("observed generation outdated (%d/%d)", issuer.Status.ObservedGeneration, issuer.Generation)
	}

	return false, "Issuer is fully rolled out"
}
