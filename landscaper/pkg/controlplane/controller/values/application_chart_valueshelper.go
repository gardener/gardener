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

package values

import (
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	"github.com/gardener/gardener/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ApplicationChartValuesHelper provides methods computing the values to be used when applying the control plane application chart
type ApplicationChartValuesHelper interface {
	// GetApplicationChartValues computes the values to be used when applying the control plane application chart.
	GetApplicationChartValues() (map[string]interface{}, error)
}

// valuesHelper is a concrete implementation of ApplicationChartValuesHelper
// Contains all values that are needed to render the control plane application chart
type valuesHelper struct {
	// GardenClient targets the garden cluster (either the virtual-garden or the runtime cluster if no-virtual garden is used)
	GardenClient client.Client
	// VirtualGarden defines if the application chart is installed into a virtual Garden cluster
	// this has implications on how the Webhook configurations are set up (cannot use k8s services directly
	// as Configuration and GAC are not deployed in the same cluster)
	VirtualGarden bool // .Values.global.deployment.virtualGarden.enabled
	// VirtualGardenClusterIP is written into the endpoints resource of the "gardener-apiserver" service.
	VirtualGardenClusterIP *string
	// CAPublicKeyGardenerAPIServer is the CA put into the APIService registering the Gardener API groups to validate
	// the TLS serving certificates presented by the Gardener API Server.
	CAPublicCertGardenerAPIServer string // .Values.global.apiserver.caBundle
	// CAPublicCertAdmissionController is the CA put into the MutatingWebhookConfiguration `gardener-admission-controller`
	// to validate the TLS serving certificates presented by the Gardener Admission Controller
	CAPublicCertAdmissionController *string // .Values.global.admission.config.server.https.tls.caBundle
	// InternalDomain configures the internal domain secret
	InternalDomain imports.DNS
	// DefaultDomains configures the internal domain secrets
	DefaultDomains []imports.DNS
	// DiffieHellmannKey configures the secret containing the OpenVPN Diffie-Hellmann-Key
	DiffieHellmannKey string
	// Alerting is needed to set up the Alerting secret
	Alerting []imports.Alerting
	// AdmissionControllerConfig is the configuration of the Gardener Admission Controller
	// Needed for the application chart for the validating webhook configuration registering the webhook `validate-resource-size.gardener.cloud`
	// Uses the resourceAdmissionConfiguration.Limits{apiGroups, apiVersions, resources / not: size, operationMode}
	// Configured to .Values.global.admission.config.server.resourceAdmissionConfiguration.limits
	AdmissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration
}

// NewApplicationChartValuesHelper creates a new ApplicationChartValuesHelper.
func NewApplicationChartValuesHelper(
	gardenClient client.Client,
	virtualGarden bool,
	virtualGardenClusterIP *string,
	caPublicCertGardenerAPIServer string,
	caPublicCertAdmissionController *string,
	internalDomain imports.DNS,
	defaultDomains []imports.DNS,
	diffieHellmannKey string,
	alerting []imports.Alerting,
	admissionControllerConfig *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration,
) ApplicationChartValuesHelper {
	return &valuesHelper{
		GardenClient:                    gardenClient,
		VirtualGarden:                   virtualGarden,
		VirtualGardenClusterIP:          virtualGardenClusterIP,
		CAPublicCertGardenerAPIServer:   caPublicCertGardenerAPIServer,
		CAPublicCertAdmissionController: caPublicCertAdmissionController,
		InternalDomain:                  internalDomain,
		DefaultDomains:                  defaultDomains,
		DiffieHellmannKey:               diffieHellmannKey,
		Alerting:                        alerting,
		AdmissionControllerConfig:       admissionControllerConfig,
	}
}

func (v valuesHelper) GetApplicationChartValues() (map[string]interface{}, error) {
	var (
		values = make(map[string]interface{})
		err    error
	)

	values, err = utils.SetToValuesMap(values, v.VirtualGarden, "deployment", "virtualGarden", "enabled")
	if err != nil {
		return nil, err
	}

	if v.VirtualGardenClusterIP != nil {
		values, err = utils.SetToValuesMap(values, *v.VirtualGardenClusterIP, "deployment", "virtualGarden", "clusterIP")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.CAPublicCertGardenerAPIServer, "apiserver", "caBundle")
	if err != nil {
		return nil, err
	}

	if v.CAPublicCertAdmissionController != nil {
		values, err = utils.SetToValuesMap(values, v.CAPublicCertAdmissionController, "admission", "config", "server", "https", "tls", "caBundle")
		if err != nil {
			return nil, err
		}
	}

	internalDomainValue, err := utils.ToValuesMapWithOptions(v.InternalDomain, utils.Options{
		LowerCaseKeys: true,
	})
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, internalDomainValue, "internalDomain")
	if err != nil {
		return nil, err
	}

	var defaultDomainValues []map[string]interface{}
	for _, domain := range v.DefaultDomains {
		v, err := utils.ToValuesMapWithOptions(domain, utils.Options{
			LowerCaseKeys: true,
		})
		if err != nil {
			return nil, err
		}

		defaultDomainValues = append(defaultDomainValues, v)
	}

	if len(defaultDomainValues) > 0 {
		values, err = utils.SetToValuesMap(values, defaultDomainValues, "defaultDomains")
		if err != nil {
			return nil, err
		}
	}

	values, err = utils.SetToValuesMap(values, v.DiffieHellmannKey, "openVPNDiffieHellmanKey")
	if err != nil {
		return nil, err
	}

	values, err = utils.SetToValuesMap(values, v.DiffieHellmannKey, "apiserver", "caBundle")
	if err != nil {
		return nil, err
	}

	if v.AdmissionControllerConfig != nil && v.AdmissionControllerConfig.Server.ResourceAdmissionConfiguration != nil {
		v, err := utils.ToValuesMap(v.AdmissionControllerConfig.Server.ResourceAdmissionConfiguration)
		if err != nil {
			return nil, err
		}

		values, err = utils.SetToValuesMap(values, v, "admission", "config", "server", "resourceAdmissionConfiguration")
		if err != nil {
			return nil, err
		}
	}

	var alertingValues []map[string]interface{}

	for _, alert := range v.Alerting {
		var alertValues map[string]interface{}

		// custom mapping needed because the keys in the helm chart do not match the keys in the import configuration
		switch alert.AuthType {
		case "none":
			alertValues = map[string]interface{}{
				"url": alert.Url,
			}
		case "smtp":
			alertValues = map[string]interface{}{
				"auth_type":     alert.AuthType,
				"to":            alert.ToEmailAddress,
				"from":          alert.FromEmailAddress,
				"smarthost":     alert.Smarthost,
				"auth_username": alert.AuthUsername,
				"auth_identity": alert.AuthIdentity,
				"auth_password": alert.AuthPassword,
			}
		case "basic":
			alertValues = map[string]interface{}{
				"url":      alert.Url,
				"username": alert.Username,
				"password": alert.Password,
			}
		case "certificate":
			alertValues = map[string]interface{}{
				"url":      alert.Url,
				"ca_crt":   alert.CaCert,
				"tls_cert": alert.TlsCert,
				"tls_key":  alert.TlsKey,
			}
		}
		alertingValues = append(alertingValues, alertValues)
	}

	if len(alertingValues) > 0 {
		values, err = utils.SetToValuesMap(values, alertingValues, "alerting")
		if err != nil {
			return nil, err
		}
	}

	return map[string]interface{}{
		"global": values,
	}, nil
}
