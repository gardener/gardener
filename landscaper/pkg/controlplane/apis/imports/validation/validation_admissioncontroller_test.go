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

package validation_test

import (
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ValidateAdmissionController", func() {
	var (
		admissionControllerConfiguration imports.GardenerAdmissionController
		componentConfig                  admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration
		path                             = field.NewPath("admissioncontroler")
		ca                               = GenerateCACertificate("gardener.cloud:system:admissioncontroller")
		caString                         = string(ca.CertificatePEM)
		cert                             = GenerateTLSServingCertificate(&ca)
		certString                       = string(cert.CertificatePEM)
		keyString                        = string(cert.PrivateKeyPEM)
	)

	BeforeEach(func() {
		componentConfig = admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{}
		admissionControllerConfiguration = imports.GardenerAdmissionController{
			DeploymentConfiguration: &imports.CommonDeploymentConfiguration{
				ReplicaCount:       pointer.Int32(1),
				ServiceAccountName: pointer.String("sx"),
				Resources: &corev1.ResourceRequirements{
					Limits: corev1.ResourceList{
						"cpu": resource.MustParse("2"),
					},
					Requests: corev1.ResourceList{
						"memory": resource.MustParse("3Gi"),
					},
				},
				PodLabels:      map[string]string{"foo": "bar"},
				PodAnnotations: map[string]string{"foo": "annotation"},
				VPA:            pointer.Bool(true),
			},
			ComponentConfiguration: &imports.AdmissionControllerComponentConfiguration{
				CABundle: &caString,
				TLS: &imports.TLSServer{
					Crt: certString,
					Key: keyString,
				},
				Configuration: &imports.Configuration{
					ComponentConfiguration: &componentConfig,
				},
			},
		}
	})

	Describe("#Validate Component Configuration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid TLS configuration - CA is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.CABundle = pointer.String("invalid")
			admissionControllerConfiguration.ComponentConfiguration.TLS = nil
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.caBundle"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - TLS serving certificate is not signed by the provided CA", func() {
			someUnknownCA := string(GenerateCACertificate("gardener.cloud:system:unknown").CertificatePEM)
			admissionControllerConfiguration.ComponentConfiguration.CABundle = pointer.String(someUnknownCA)
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("admissioncontroler.componentConfiguration.tls.crt"),
					"Detail": Equal("failed to verify the TLS serving certificate against the given CA bundle: x509: certificate signed by unknown authority"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - cert not specified", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = ""
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - cert is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = ""
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key not specified", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Key = ""
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Key = ""
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid TLS ServerCertPath in the component configuration", func() {
			admissionControllerConfiguration.ComponentConfiguration.ComponentConfiguration = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
				Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
					HTTPS: admissioncontrollerconfigv1alpha1.HTTPSServer{
						TLS: admissioncontrollerconfigv1alpha1.TLSServer{
							ServerCertDir: "/my/path",
						},
					},
				},
			}
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.config.server.https.tls.serverCertDir"),
				})),
			))
		})

		It("should forbid specifiying a Garden kubeconfig in the component configuration", func() {
			admissionControllerConfiguration.ComponentConfiguration.ComponentConfiguration = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
				GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "almost valid kubeconfig",
				},
			}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.componentConfiguration.config.gardenClientConnection.kubeconfig"),
				})),
			))
		})

		// Demonstrate that the ControllerManager component configuration is validated.
		// Otherwise, the component might fail after deployment by the landscaper
		It("should forbid invalid configurations", func() {
			mode := admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode("invalid_mode")
			admissionControllerConfiguration.ComponentConfiguration.ComponentConfiguration = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
				Server: admissioncontrollerconfigv1alpha1.ServerConfiguration{
					ResourceAdmissionConfiguration: &admissioncontrollerconfigv1alpha1.ResourceAdmissionConfiguration{
						OperationMode: &mode,
					},
				},
			}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("admissioncontroler.componentConfiguration.config.server.resourceAdmissionConfiguration.mode"),
				})),
			))
		})
	})

	Context("validate the AdmissionController's deployment configuration", func() {
		It("should validate that the replica count is not negative", func() {
			admissionControllerConfiguration.DeploymentConfiguration.ReplicaCount = pointer.Int32(-1)

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.deploymentConfiguration.replicaCount"),
				})),
			))
		})

		It("should validate that the service account name is valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.deploymentConfiguration.serviceAccountName"),
				})),
			))
		})

		It("should validate that the pod labels are valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.deploymentConfiguration.podLabels"),
				})),
			))
		})

		It("should validate that the podAnnotations are valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroler.deploymentConfiguration.podAnnotations"),
				})),
			))
		})
	})
})
