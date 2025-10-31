// SPDX-FileCopyrightText: SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package gardener

import (
	"crypto/x509"
	"fmt"
	"reflect"
	"strings"

	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
)

var csrRequiredKeyUsages = []certificatesv1.KeyUsage{
	certificatesv1.UsageKeyEncipherment,
	certificatesv1.UsageDigitalSignature,
	certificatesv1.UsageClientAuth,
}

// IsSeedClientCert returns true when the given CSR and usages match the requirements for a client certificate for a
// seed. If false is returned, a reason will be returned explaining which requirement was not met.
func IsSeedClientCert(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage) (bool, string) {
	requiredOrganizations := []string{v1beta1constants.SeedsGroup}

	if !reflect.DeepEqual(requiredOrganizations, x509cr.Subject.Organization) {
		return false, fmt.Sprintf("subject's organization is not set to %v", requiredOrganizations)
	}

	if !strings.HasPrefix(x509cr.Subject.CommonName, v1beta1constants.SeedUserNamePrefix) {
		return false, fmt.Sprintf("subject's common name does not start with %q", v1beta1constants.SeedUserNamePrefix)
	}

	return areCSRRequirementsMet(x509cr, usages)
}

// IsGardenadmClientCert returns true when the given CSR and usages match the requirements for a client
// certificate for a self-hosted shoot with the `gardenadm connect` prefix. If false is returned, a reason will be
// returned explaining which requirement was not met.
func IsGardenadmClientCert(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage) (bool, string) {
	return isSelfHostedShootClientCert(x509cr, usages, v1beta1constants.GardenadmUserNamePrefix)
}

// IsShootClientCert returns true when the given CSR and usages match the requirements for a client certificate for an
// self-hosted shoot with the gardenlet prefix. If false is returned, a reason will be returned explaining which
// requirement was not met.
func IsShootClientCert(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage) (bool, string) {
	return isSelfHostedShootClientCert(x509cr, usages, v1beta1constants.ShootUserNamePrefix)
}

func isSelfHostedShootClientCert(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage, prefix string) (bool, string) {
	requiredOrganizations := []string{v1beta1constants.ShootsGroup}

	if !reflect.DeepEqual(requiredOrganizations, x509cr.Subject.Organization) {
		return false, fmt.Sprintf("subject's organization is not set to %v", requiredOrganizations)
	}

	if !strings.HasPrefix(x509cr.Subject.CommonName, prefix) {
		return false, fmt.Sprintf("subject's common name does not start with %q", prefix)
	}

	if parts := strings.Split(strings.TrimPrefix(x509cr.Subject.CommonName, prefix), ":"); len(parts) != 2 ||
		parts[0] == "" || parts[1] == "" {
		return false, fmt.Sprintf("subject's common name must follow this structure: %q", prefix+"<namespace>:<name>")
	}

	return areCSRRequirementsMet(x509cr, usages)
}

func areCSRRequirementsMet(x509cr *x509.CertificateRequest, usages []certificatesv1.KeyUsage) (bool, string) {
	if (len(x509cr.DNSNames) > 0) || (len(x509cr.EmailAddresses) > 0) || (len(x509cr.IPAddresses) > 0) {
		return false, "DNSNames, EmailAddresses and IPAddresses fields must be empty"
	}

	if !sets.New(usages...).Equal(sets.New(csrRequiredKeyUsages...)) {
		return false, fmt.Sprintf("key usages are not set to %v", csrRequiredKeyUsages)
	}

	return true, ""
}
