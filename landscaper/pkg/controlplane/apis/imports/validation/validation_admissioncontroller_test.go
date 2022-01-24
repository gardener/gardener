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
	testutils "github.com/gardener/gardener/landscaper/common/test-utils"
	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	admissioncontrollerconfigv1alpha1 "github.com/gardener/gardener/pkg/admissioncontroller/apis/config/v1alpha1"
	. "github.com/onsi/ginkgo/v2"
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
		path                             = field.NewPath("admissioncontroller")
		ca                               = testutils.GenerateCACertificate("gardener.cloud:system:admissioncontroller")
		caCrt                            = string(ca.CertificatePEM)
		caKey                            = string(ca.PrivateKeyPEM)
		cert                             = testutils.GenerateTLSServingCertificate(&ca)
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
				CA: &imports.CA{
					Crt: &caCrt,
					Key: &caKey,
				},
				TLS: &imports.TLSServer{
					Crt: &certString,
					Key: &keyString,
				},
				Config: &componentConfig,
			},
		}
	})

	Describe("#Validate Component Configuration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(BeEmpty())
		})

		Context("CA", func() {
			It("CA public key must be provided in order to validate the TLS serving cert of the Gardener Admission Controller server", func() {
				admissionControllerConfiguration.ComponentConfiguration.CA.Crt = nil
				errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("admissioncontroller.componentConfiguration.ca.crt"),
						"Detail": Equal("It is forbidden to only provide the TLS serving certificates of the Gardener Admission Controller, but not the CA for verification."),
					})),
				))
			})

			It("CA private key must be provided to generate TLS serving certs", func() {
				admissionControllerConfiguration.ComponentConfiguration.CA.Key = nil
				admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = nil
				errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("admissioncontroller.componentConfiguration.ca.key"),
						"Detail": Equal("When providing a custom CA (public part) and the TLS serving Certificate of the Gardener Admission Controller are not provided, the private key of the CA is required in order to generate the TLS serving certs."),
					})),
				))
			})

			It("Should forbid providing both an CA secret reference as well as the values", func() {
				admissionControllerConfiguration.ComponentConfiguration.CA.SecretRef = &corev1.SecretReference{}
				errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("admissioncontroller.componentConfiguration.ca.secretRef"),
						"Detail": Equal("cannot both set the secret reference and the CA certificate values"),
					})),
				))
			})
		})

		It("should forbid providing both the TLS certificate values and the secret reference", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.SecretRef = &corev1.SecretReference{}
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("admissioncontroller.componentConfiguration.tls.secretRef"),
					"Detail": ContainSubstring("cannot both set the secret reference and the TLS certificate values"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - CA is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.CA.Crt = pointer.String("invalid")
			admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = nil
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.ca.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - TLS serving certificate is not signed by the provided CA", func() {
			someUnknownCA := string(testutils.GenerateCACertificate("gardener.cloud:system:unknown").CertificatePEM)
			admissionControllerConfiguration.ComponentConfiguration.CA.Crt = &someUnknownCA
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("admissioncontroller.componentConfiguration.tls.crt"),
					"Detail": Equal("failed to verify the TLS serving certificate against the given CA bundle: x509: certificate signed by unknown authority"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - cert not specified", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = pointer.String("")
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - cert is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Crt = pointer.String("")
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key not specified", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Key = pointer.String("")
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key is invalid", func() {
			admissionControllerConfiguration.ComponentConfiguration.TLS.Key = pointer.String("")
			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid TLS ServerCertPath in the component configuration", func() {
			admissionControllerConfiguration.ComponentConfiguration.Config = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
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
					"Field": Equal("admissioncontroller.componentConfiguration.config.server.https.tls.serverCertDir"),
				})),
			))
		})

		It("should forbid specifiying a Garden kubeconfig in the component configuration", func() {
			admissionControllerConfiguration.ComponentConfiguration.Config = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
				GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "almost valid kubeconfig",
				},
			}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.componentConfiguration.config.gardenClientConnection.kubeconfig"),
				})),
			))
		})

		// Demonstrate that the ControllerManager component configuration is validated.
		// Otherwise, the component might fail after deployment by the landscaper
		It("should forbid invalid configurations", func() {
			mode := admissioncontrollerconfigv1alpha1.ResourceAdmissionWebhookMode("invalid_mode")
			admissionControllerConfiguration.ComponentConfiguration.Config = &admissioncontrollerconfigv1alpha1.AdmissionControllerConfiguration{
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
					"Field": Equal("admissioncontroller.componentConfiguration.config.server.resourceAdmissionConfiguration.mode"),
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
					"Field": Equal("admissioncontroller.deploymentConfiguration.replicaCount"),
				})),
			))
		})

		It("should validate that the service account name is valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.deploymentConfiguration.serviceAccountName"),
				})),
			))
		})

		It("should validate that the pod labels are valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.deploymentConfiguration.podLabels"),
				})),
			))
		})

		It("should validate that the podAnnotations are valid", func() {
			admissionControllerConfiguration.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}

			errorList := ValidateAdmissionController(admissionControllerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("admissioncontroller.deploymentConfiguration.podAnnotations"),
				})),
			))
		})
	})
})
