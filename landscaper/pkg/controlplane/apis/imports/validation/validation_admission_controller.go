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
	"fmt"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	apisconfig "github.com/gardener/gardener/pkg/admissioncontroller/apis/config"
	admissionconfighelper "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/helper"
	admissionconfigvalidation "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateAdmissionController validates the configuration of the Gardener Admission Controller
func ValidateAdmissionController(config imports.GardenerAdmissionController, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if config.DeploymentConfiguration != nil {
		allErrs = append(allErrs, ValidateCommonDeployment(*config.DeploymentConfiguration, fldPath.Child("deploymentConfiguration"))...)
	}

	if config.ComponentConfiguration != nil {
		allErrs = append(allErrs, ValidateAdmissionControllerComponentConfiguration(*config.ComponentConfiguration, fldPath.Child("componentConfiguration"))...)
	}

	return allErrs
}

// ValidateAdmissionControllerComponentConfiguration validates the component configuration of the Gardener Admission Controller
func ValidateAdmissionControllerComponentConfiguration(config imports.AdmissionControllerComponentConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// providing the CA is mandatory, as it is put in the WebhookConfigurations in the virtual garden cluster to validate the TLS serving certs of the Admission Controller.
	// In addition, the CA needs to be provided if the TLS serving certs are provided because we cannot generate a new
	// CABundle for an existing TLS serving certificate.
	if (config.CA == nil || (config.CA.Crt == nil && config.CA.SecretRef == nil) || len(*config.CA.Crt) == 0) && config.TLS != nil {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("ca").Child("crt"), "It is forbidden to only providing the TLS serving certificates of the Gardener Admission Controller, but not the CA for verification."))
	} else if config.CA != nil && config.CA.Key == nil && config.CA.SecretRef == nil && config.TLS == nil {
		// When providing a custom CA, we need to have the private key in order to generate the TLS serving certs
		// of the Gardener Admission Controller
		allErrs = append(allErrs, field.Invalid(fldPath.Child("ca").Child("key"), "", "When providing a custom CA (public part) and the TLS serving Certificate of the Gardener Admission Controller are not provided, the private key of the CA is required in order to generate the TLS serving certs."))
	}

	if config.CA != nil {
		allErrs = append(allErrs, ValidateCommonCA(*config.CA, fldPath.Child("ca"))...)
	}

	if config.TLS != nil {
		errors := ValidateCommonTLSServer(*config.TLS, fldPath.Child("tls"))

		// only makes sense to further validate the cert against the CA, if the cert is valid in the first place
		if len(errors) == 0 && config.TLS.Crt != nil && config.CA != nil && config.CA.Crt != nil {
			allErrs = append(allErrs, ValidateTLSServingCertificateAgainstCA(*config.TLS.Crt, *config.CA.Crt, fldPath.Child("tls").Child("crt"))...)
		}
		allErrs = append(allErrs, errors...)
	}

	// Convert the admission controller config to an internal version
	if config.Config != nil {
		fldPathComponentConfig := fldPath.Child("config")

		componentConfig, err := admissionconfighelper.ConvertAdmissionControllerConfiguration(config.Config)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPathComponentConfig, config.Config, fmt.Sprintf("could not convert to admission controller configuration: %v", err)))
			return allErrs
		}

		allErrs = append(allErrs, ValidateAdmissionControllerConfiguration(componentConfig, fldPathComponentConfig)...)
	}

	return allErrs
}

// ValidateAdmissionControllerConfiguration validates the Gardener Admission Controller component configuration
func ValidateAdmissionControllerConfiguration(config *apisconfig.AdmissionControllerConfiguration, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.GardenClientConnection.Kubeconfig) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("gardenClientConnection").Child("kubeconfig"), config.GardenClientConnection.Kubeconfig, "The path to the kubeconfig for the Garden cluster in the Gardener Admission Controller must not be set. Instead the provided runtime cluster or virtual garden cluster kubeconfig will be used."))
	}

	if len(config.Server.HTTPS.TLS.ServerCertDir) > 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("server").Child("https").Child("tls").Child("serverCertDir"), config.Server.HTTPS.TLS.ServerCertDir, "The path to the TLS serving certificate of the Gardener Admission Controller must not be set. Instead, directly provide the certificates via the landscaper imports field gardenerAdmissionController.componentConfiguration.tls.certificate and gardenerAdmissionController.componentConfiguration.tls.key."))
	}

	if errorList := admissionconfigvalidation.ValidateAdmissionControllerConfiguration(config); len(errorList) > 0 {
		for _, err := range errorList {
			err.Field = fmt.Sprintf("%s.%s", fldPath.String(), err.Field)
			allErrs = append(allErrs, err)
		}
	}

	return allErrs
}
