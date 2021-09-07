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

package validation

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	corevalidation "github.com/gardener/gardener/pkg/apis/core/validation"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1validation "k8s.io/apimachinery/pkg/apis/meta/v1/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/keyutil"
)

// ValidateLandscaperImports validates an imports object.
func ValidateLandscaperImports(imports *imports.Imports) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(imports.RuntimeCluster.Spec.Configuration.RawMessage) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("gardenCluster"), "the runtime cluster kubeconfig has to be provided."))
	}

	if imports.VirtualGarden != nil && imports.VirtualGarden.Enabled && len(imports.VirtualGarden.Kubeconfig.Spec.Configuration.RawMessage) == 0 {
		allErrs = append(allErrs, field.Required(field.NewPath("seedCluster"), "the virtual Garden cluster kubeconfig has to be provided when the virtual Garden setup option is enabled."))
	}

	allErrs = append(allErrs, validateDNS(imports.InternalDomain, field.NewPath("internalDomain"))...)

	for i, dns := range imports.DefaultDomains {
		allErrs = append(allErrs, validateDNS(dns, field.NewPath("defaultDomain").Index(i))...)
	}

	for i, alerting := range imports.Alerting {
		allErrs = append(allErrs, validateAlerting(alerting, field.NewPath("alerting").Index(i))...)
	}

	allErrs = append(allErrs, ValidateAPIServer(imports.GardenerAPIServer, field.NewPath("gardenerApiserver"))...)

	if imports.GardenerControllerManager != nil {
		allErrs = append(allErrs, ValidateControllerManager(*imports.GardenerControllerManager, field.NewPath("gardenerControllerManager"))...)
	}

	if imports.GardenerScheduler != nil {
		allErrs = append(allErrs, ValidateScheduler(*imports.GardenerScheduler, field.NewPath("gardenerScheduler"))...)
	}

	if imports.GardenerAdmissionController != nil {
		allErrs = append(allErrs, ValidateAdmissionController(*imports.GardenerAdmissionController, field.NewPath("gardenerAdmissionController"))...)
	}

	return allErrs
}

func validateDNS(dns imports.DNS, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(dns.Domain) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("domain"), "DNS domain must not be empty."))
	}
	if len(dns.Provider) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("provider"), "DNS provider must not be empty."))
	}
	if len(dns.Credentials) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("credentials"), "DNS credentials must not be empty."))
	}
	return allErrs
}

func validateAlerting(alerting imports.Alerting, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	switch alerting.AuthType {
	case "none":
		if alerting.Url == nil || len(*alerting.Url) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("url"), "The alerting url is required for alerting with basic authentication"))
		}
	case "smtp":
		if alerting.ToEmailAddress == nil || len(*alerting.ToEmailAddress) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("toEmailAddress"), "Using smtp for Alerting requires to set the email address for alerting"))
		}
		if alerting.FromEmailAddress == nil || len(*alerting.FromEmailAddress) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("fromEmailAddress"), "Using smtp for Alerting requires to set the email address for alerting"))
		}
		if alerting.Smarthost == nil || len(*alerting.Smarthost) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("smarthost"), "Using smtp for Alerting requires to set the smarthost"))
		}
		if alerting.AuthUsername == nil || len(*alerting.AuthUsername) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("authUsername"), "Using smtp for Alerting requires to set the username used for authentication"))
		}
		if alerting.AuthIdentity == nil || len(*alerting.AuthIdentity) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("authIdentity"), "Using smtp for Alerting requires to set the identity used for authentication"))
		}
		if alerting.AuthPassword == nil || len(*alerting.AuthPassword) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("authPassword"), "Using smtp for Alerting requires to set the password for authentication"))
		}
	case "basic":
		if alerting.Url == nil || len(*alerting.Url) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("url"), "Using basic authentication for Alerting requires to set the url"))
		}
		if alerting.Username == nil || len(*alerting.Username) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("username"), "Using basic authentication for Alerting requires to set the username"))
		}
		if alerting.Password == nil || len(*alerting.Password) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("password"), "Using basic authentication for Alerting requires to set the password"))
		}
	case "certificate":
		if alerting.Url == nil || len(*alerting.Url) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("url"), "Using certificate authentication for Alerting requires to set the url"))
		}
		if alerting.CaCert == nil || len(*alerting.CaCert) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("caCert"), "Using certificate authentication for Alerting requires to set the CA certificate"))
		}
		if alerting.TlsCert == nil || len(*alerting.TlsCert) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("tlsCert"), "Using certificate authentication for Alerting requires to set the Tls certificate"))
		}
		if alerting.TlsKey == nil || len(*alerting.TlsKey) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("tlsKey"), "Using certificate authentication for Alerting requires to set the Tls key"))
		}
	default:
		allErrs = append(allErrs, field.Invalid(fldPath.Child("AuthType"), alerting.AuthType, "The authentication type for alerting has to be one of [smtp, none, basic, certificate]"))
	}
	return allErrs
}

// ValidateCommonDeployment validates the deployment configuration
func ValidateCommonDeployment(deployment imports.CommonDeploymentConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if deployment.ReplicaCount != nil {
		allErrs = append(allErrs, apivalidation.ValidateNonnegativeField(int64(*deployment.ReplicaCount), fldPath.Child("replicaCount"))...)
	}
	if deployment.ServiceAccountName != nil {
		for _, msg := range apivalidation.ValidateServiceAccountName(*deployment.ServiceAccountName, false) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("serviceAccountName"), *deployment.ServiceAccountName, msg))
		}
	}

	if deployment.Resources != nil {
		fldPathResources := fldPath.Child("resources")
		for key, requests := range deployment.Resources.Requests {
			allErrs = append(allErrs, corevalidation.ValidateResourceQuantityValue(string(key), requests, fldPathResources.Child("requests"))...)
		}
		for key, limits := range deployment.Resources.Limits {
			allErrs = append(allErrs, corevalidation.ValidateResourceQuantityValue(string(key), limits, fldPathResources.Child("limits"))...)
		}
	}

	allErrs = append(allErrs, metav1validation.ValidateLabels(deployment.PodLabels, fldPath.Child("podLabels"))...)
	allErrs = append(allErrs, apivalidation.ValidateAnnotations(deployment.PodAnnotations, fldPath.Child("podAnnotations"))...)

	return allErrs
}

// ValidateCommonTLSServer validates TLS server configuration
func ValidateCommonTLSServer(config imports.TLSServer, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.Crt) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("crt"), config.Crt, "the TLS certificate must be set"))
	} else {
		allErrs = append(allErrs, ValidateTLSServingCertificate(config.Crt, fldPath.Child("crt"))...)
	}

	if len(config.Key) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("key"), config.Key, "the TLS key must be set"))
	} else {
		allErrs = append(allErrs, ValidatePrivateKey(config.Key, fldPath.Child("key"))...)
	}

	return allErrs
}

// ValidateCABundle validates that the given string contains a valid PEM encoded x509 CA certificate
func ValidateCABundle(bundle string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	block, _ := pem.Decode([]byte(bundle))
	if block == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, "the TLS certificate provided is not a valid PEM encoded X509 certificate"))
		return allErrs
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided cannot be parses as a X509 certificate: %s", err.Error())))
	}

	// Test if parsed key is an RSA Public Key
	var ok bool
	if _, ok = cert.PublicKey.(*rsa.PublicKey); !ok {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided doesn't contain valid RSA Public Key: %s", err.Error())))
	}

	if !cert.IsCA {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, "the TLS certificate provided is not a CA certificate"))
	}

	return allErrs
}

// ValidateTLSServingCertificate validates that the given string contains a valid PEM encoded x509 TLS serving certificate
func ValidateTLSServingCertificate(certificate string, fldPath *field.Path) field.ErrorList {
	return validateCertificate(certificate, x509.ExtKeyUsageServerAuth, fldPath)
}

// ValidateClientCertificate validates that the given string contains a valid PEM encoded x509 TLS client certificate
func ValidateClientCertificate(certificate string, fldPath *field.Path) field.ErrorList {
	return validateCertificate(certificate, x509.ExtKeyUsageClientAuth, fldPath)
}

func validateCertificate(certificate string, keyUsage x509.ExtKeyUsage, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	block, _ := pem.Decode([]byte(certificate))
	if block == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, "the TLS certificate is not PEM encoded"))
		return allErrs
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided cannot be parses as a X509 certificate: %s", err.Error())))
		return allErrs
	}

	// Test if parsed key is an RSA Public Key
	var ok bool
	if _, ok = cert.PublicKey.(*rsa.PublicKey); !ok {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided doesn't contain valid RSA Public Key: %s", err.Error())))
		return allErrs
	}

	found := false
	for _, usage := range cert.ExtKeyUsage {
		if usage == keyUsage {
			found = true
			break
		}
	}

	if !found {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the X509 certificate provided is not valid (missing key usage %q)", keyUsage)))
	}

	return allErrs
}

// ValidatePrivateKey validates that the given string contains a valid PEM encoded x509 TLS private certificate
func ValidatePrivateKey(key string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	_, err := keyutil.ParsePrivateKeyPEM([]byte(key))
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided is not a valid PEM encoded X509 private key: %s", err.Error())))
	}
	return allErrs
}

// ValidateTLSServingCertificateAgainstCA validates the given PEM encoded X509 certificate against the given CA.
func ValidateTLSServingCertificateAgainstCA(cert, ca string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	rootCAs := x509.NewCertPool()

	if ok := rootCAs.AppendCertsFromPEM([]byte(ca)); !ok {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, "internal validation error occurred. Could not add the provided CA to validate against the given X509 certificate"))
		return allErrs
	}

	block, _ := pem.Decode([]byte(cert))
	if block == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, "the TLS certificate is not PEM encoded"))
		return allErrs
	}
	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("the TLS certificate provided cannot be parses as a X509 certificate: %s", err.Error())))
		return allErrs
	}

	opts := x509.VerifyOptions{
		Roots: rootCAs,
		KeyUsages: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
	}

	if _, err := x509Cert.Verify(opts); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, nil, fmt.Sprintf("failed to verify the TLS serving certificate against the given CA bundle: %s", err.Error())))
	}

	return allErrs
}
