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
	controllermanagerconfigv1alpha1 "github.com/gardener/gardener/pkg/controllermanager/apis/config/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ValidateControllerManager", func() {
	var (
		controllerManagerConfiguration imports.GardenerControllerManager
		componentConfig                controllermanagerconfigv1alpha1.ControllerManagerConfiguration
		path                           = field.NewPath("controllermanager")
		cert                           = GenerateTLSServingCertificate(nil)
		certString                     = string(cert.CertificatePEM)
		keyString                      = string(cert.PrivateKeyPEM)
	)

	BeforeEach(func() {
		componentConfig = controllermanagerconfigv1alpha1.ControllerManagerConfiguration{}

		controllerManagerConfiguration = imports.GardenerControllerManager{
			DeploymentConfiguration: &imports.ControllerManagerDeploymentConfiguration{
				CommonDeploymentConfiguration: &imports.CommonDeploymentConfiguration{
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
				AdditionalVolumes:      nil,
				AdditionalVolumeMounts: nil,
				Env:                    nil,
			},
			ComponentConfiguration: &imports.ControllerManagerComponentConfiguration{
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
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid invalid TLS configuration - cert not specified", func() {
			controllerManagerConfiguration.ComponentConfiguration.TLS.Crt = ""
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - cert invalid", func() {
			controllerManagerConfiguration.ComponentConfiguration.TLS.Crt = "invalid"
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.tls.crt"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key not specified", func() {
			controllerManagerConfiguration.ComponentConfiguration.TLS.Key = ""
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid invalid TLS configuration - key invalid", func() {
			controllerManagerConfiguration.ComponentConfiguration.TLS.Key = "invalid"
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.tls.key"),
				})),
			))
		})

		It("should forbid TLS ServerCertPath in the component configuration", func() {
			controllerManagerConfiguration.ComponentConfiguration.ComponentConfiguration = &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
				Server: controllermanagerconfigv1alpha1.ServerConfiguration{
					HTTPS: controllermanagerconfigv1alpha1.HTTPSServer{
						TLS: controllermanagerconfigv1alpha1.TLSServer{
							ServerCertPath: "/my/path",
						},
					},
				},
			}
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.config.server.https.tls.serverCertPath"),
				})),
			))
		})

		It("should forbid TLS ServerKeyPath in the component configuration", func() {
			controllerManagerConfiguration.ComponentConfiguration.ComponentConfiguration = &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
				Server: controllermanagerconfigv1alpha1.ServerConfiguration{
					HTTPS: controllermanagerconfigv1alpha1.HTTPSServer{
						TLS: controllermanagerconfigv1alpha1.TLSServer{
							ServerKeyPath: "/key/path",
						},
					},
				},
			}
			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.config.server.https.tls.serverKeyPath"),
				})),
			))
		})

		It("should forbid specifiying a Garden kubeconfig in the component configuration", func() {
			controllerManagerConfiguration.ComponentConfiguration.ComponentConfiguration = &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
				GardenClientConnection: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
					Kubeconfig: "almost valid kubeconfig",
				},
			}

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.componentConfiguration.config.gardenClientConnection.kubeconfig"),
				})),
			))
		})

		// Demonstrate that the ControllerManager component configuration is validated.
		// Otherwise, the component might fail after deployment by the landscaper
		It("should forbid invalid configurations", func() {
			controllerManagerConfiguration.ComponentConfiguration.ComponentConfiguration = &controllermanagerconfigv1alpha1.ControllerManagerConfiguration{
				LogLevel: "invalid log level",
			}

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("controllermanager.componentConfiguration.config.logLevel"),
				})),
			))
		})
	})

	Context("validate the ControllerManagers's deployment configuration", func() {
		It("should validate that the replica count is not negative", func() {
			controllerManagerConfiguration.DeploymentConfiguration.ReplicaCount = pointer.Int32(-1)

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.deploymentConfiguration.replicaCount"),
				})),
			))
		})

		It("should validate that the service account name is valid", func() {
			controllerManagerConfiguration.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.deploymentConfiguration.serviceAccountName"),
				})),
			))
		})

		It("should validate that the pod labels are valid", func() {
			controllerManagerConfiguration.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.deploymentConfiguration.podLabels"),
				})),
			))
		})

		It("should validate that the podAnnotations are valid", func() {
			controllerManagerConfiguration.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}

			errorList := ValidateControllerManager(controllerManagerConfiguration, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("controllermanager.deploymentConfiguration.podAnnotations"),
				})),
			))
		})
	})
})
