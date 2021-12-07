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
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ApplicationChartValuesHelper", func() {
	var (
		expectedValues  func(bool, bool) map[string]interface{}
		getValuesHelper func(bool, bool) ApplicationChartValuesHelper

		clusterIP                 = "10.0.1.1"
		apiServerCACert           = "caCertCM"
		admissionControllerCACert = "admissionCertCM"
		openVPNDiffieHellmanKey   = "openvpn-key"

		internalDomain = imports.DNS{
			Domain:   "abc",
			Provider: "aws",
			Zone:     pointer.String("north"),
		}

		defaultDomains = []imports.DNS{
			{
				Domain:   "abc-default",
				Provider: "aws",
				Zone:     pointer.String("south"),
			},
			{
				Domain:   "abc-default2",
				Provider: "aws2",
				Zone:     pointer.String("south2"),
			},
		}

		alerting = []imports.Alerting{
			{
				AuthType:         "smtp",
				ToEmailAddress:   pointer.String("abc@net.com"),
				FromEmailAddress: pointer.String("xy"),
				Smarthost:        pointer.String("xy"),
				AuthUsername:     pointer.String("xy"),
				AuthIdentity:     pointer.String("xy"),
				AuthPassword:     pointer.String("xy"),
			},
			{
				AuthType: "certificate",
				Url:      pointer.String("url"),
				CaCert:   pointer.String("x"),
				TlsCert:  pointer.String("my cert"),
				TlsKey:   pointer.String("x"),
			},
			{
				AuthType: "basic",
				Url:      pointer.String("url"),
				Username: pointer.String("user"),
				Password: pointer.String("pw"),
			},
			{
				AuthType: "none",
				Url:      pointer.String("url"),
			},
		}

		admissionConfig = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
			Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
				ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
					Limits: []admissioncontrollerconfigv1alpha1.ResourceLimit{
						{
							Size: resource.MustParse("1"),
						},
					},
				},
			},
		}
	)

	BeforeEach(func() {
		getValuesHelper = func(virtualGardenEnabled, admissionControllerEnabled bool) ApplicationChartValuesHelper {
			var (
				gardenerAdmissionControllerCA *string
				admissionCfg                  *admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration
			)
			if admissionControllerEnabled {
				gardenerAdmissionControllerCA = &admissionControllerCACert
				admissionCfg = admissionConfig
			}

			return NewApplicationChartValuesHelper(
				virtualGardenEnabled,
				&clusterIP,
				apiServerCACert,
				gardenerAdmissionControllerCA,
				internalDomain,
				defaultDomains,
				openVPNDiffieHellmanKey,
				alerting,
				admissionCfg,
				pointer.Bool(true),
			)
		}

		expectedValues = func(virtualGardenEnabled bool, admissionControllerEnabled bool) map[string]interface{} {
			result := map[string]interface{}{
				"global": map[string]interface{}{
					"alerting": []map[string]interface{}{
						{
							"auth_type":     "smtp",
							"to":            pointer.String("abc@net.com"),
							"from":          pointer.String("xy"),
							"smarthost":     pointer.String("xy"),
							"auth_username": pointer.String("xy"),
							"auth_identity": pointer.String("xy"),
							"auth_password": pointer.String("xy"),
						},
						{
							"auth_type": "certificate",
							"url":       pointer.String("url"),
							"ca_crt":    pointer.String("x"),
							"tls_cert":  pointer.String("my cert"),
							"tls_key":   pointer.String("x"),
						},
						{
							"auth_type": "basic",
							"url":       pointer.String("url"),
							"username":  pointer.String("user"),
							"password":  pointer.String("pw"),
						},
						{
							"auth_type": "none",
							"url":       pointer.String("url"),
						},
					},
					"deployment": map[string]interface{}{
						"virtualGarden": map[string]interface{}{
							"clusterIP": "10.0.1.1",
						},
					},
					"apiserver": map[string]interface{}{
						"caBundle": "caCertCM",
					},
					"internalDomain": map[string]interface{}{
						"domain":   "abc",
						"provider": "aws",
						"zone":     "north",
					},
					"defaultDomains": []map[string]interface{}{
						{
							"domain":   "abc-default",
							"provider": "aws",
							"zone":     "south",
						},
						{
							"provider": "aws2",
							"zone":     "south2",
							"domain":   "abc-default2",
						},
					},
					"openVPNDiffieHellmanKey": "openvpn-key",
					"admission": map[string]interface{}{
						"seedRestriction": map[string]interface{}{
							"enabled": pointer.Bool(true),
						},
					},
				},
			}

			result, _ = utils.SetToValuesMap(result, virtualGardenEnabled, "global", "deployment", "virtualGarden", "enabled")

			if admissionControllerEnabled {
				result, _ = utils.SetToValuesMap(result, &admissionControllerCACert, "global", "admission", "config", "server", "https", "tls", "caBundle")

				resourceAdmissionConfigurationValues := map[string]interface{}{
					"limits": []interface{}{
						map[string]interface{}{
							"size": "1",
						},
					},
				}

				result, _ = utils.SetToValuesMap(result, resourceAdmissionConfigurationValues, "global", "admission", "config", "server", "resourceAdmissionConfiguration")
			}

			return result
		}
	})

	Describe("#GetApplicationChartValues", func() {
		It("should compute the correct application chart values - virtual garden", func() {
			result, err := getValuesHelper(true, false).GetApplicationChartValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedValues(true, false)))
		})

		It("should compute the correct application chart values - with admission controller CA", func() {
			result, err := getValuesHelper(false, true).GetApplicationChartValues()
			Expect(err).ToNot(HaveOccurred())
			Expect(result).To(Equal(expectedValues(false, true)))
		})
	})

})
