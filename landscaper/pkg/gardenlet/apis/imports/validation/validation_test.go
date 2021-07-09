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
	"encoding/json"

	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	componentbaseconfigv1alpha1 "k8s.io/component-base/config/v1alpha1"
	"k8s.io/utils/pointer"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	"github.com/gardener/gardener/pkg/apis/seedmanagement"
	gardenletconfigv1alpha1 "github.com/gardener/gardener/pkg/gardenlet/apis/config/v1alpha1"

	"github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports"
	. "github.com/gardener/gardener/landscaper/pkg/gardenlet/apis/imports/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("ValidateLandscaperImports", func() {
	var (
		landscaperGardenletImport *imports.Imports
		gardenletConfiguration    gardenletconfigv1alpha1.GardenletConfiguration
	)

	BeforeEach(func() {
		ingressDomain := "super.domain"
		gardenletConfiguration = gardenletconfigv1alpha1.GardenletConfiguration{
			SeedConfig: &gardenletconfigv1alpha1.SeedConfig{
				SeedTemplate: gardencorev1beta1.SeedTemplate{
					ObjectMeta: metav1.ObjectMeta{
						Name: "my-seed",
					},
					Spec: gardencorev1beta1.SeedSpec{
						Provider: gardencorev1beta1.SeedProvider{
							Type:   "a",
							Region: "north-west",
						},
						DNS: gardencorev1beta1.SeedDNS{IngressDomain: &ingressDomain},
						Networks: gardencorev1beta1.SeedNetworks{
							Pods:     "100.96.0.0/32",
							Services: "100.40.0.0/32",
						},
					},
				},
			},
			Server: &gardenletconfigv1alpha1.ServerConfiguration{
				HTTPS: gardenletconfigv1alpha1.HTTPSServer{
					Server: gardenletconfigv1alpha1.Server{
						BindAddress: "0.0.0.0",
						Port:        2720,
					},
				},
			},
		}

		landscaperGardenletImport = &imports.Imports{
			SeedCluster: landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{
				Configuration: landscaperv1alpha1.AnyJSON{RawMessage: []byte("dummy")},
			}},
			GardenCluster: landscaperv1alpha1.Target{Spec: landscaperv1alpha1.TargetSpec{
				Configuration: landscaperv1alpha1.AnyJSON{RawMessage: []byte("dummy")},
			}},
			ComponentConfiguration:  &gardenletConfiguration,
			DeploymentConfiguration: &seedmanagement.GardenletDeployment{},
		}

	})

	Describe("#ValidateLandscaperImports", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateLandscaperImports(landscaperGardenletImport)
			Expect(errorList).To(BeEmpty())
		})

		It("should validate the runtime cluster is set", func() {
			landscaperGardenletImport.SeedCluster = landscaperv1alpha1.Target{}
			errorList := ValidateLandscaperImports(landscaperGardenletImport)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("seedCluster"),
				})),
			))
		})

		It("should validate the garden cluster is set", func() {
			landscaperGardenletImport.GardenCluster = landscaperv1alpha1.Target{}
			errorList := ValidateLandscaperImports(landscaperGardenletImport)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("gardenCluster"),
				})),
			))
		})

		Context("validate Gardenlet configuration", func() {
			It("should validate that the kubeconfig in the GardenClientConnection is not set", func() {
				gardenletConfiguration.GardenClientConnection = &gardenletconfigv1alpha1.GardenClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						Kubeconfig: "path",
					},
				}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("componentConfiguration.gardenClientConnection.kubeconfig"),
					})),
				))
			})

			It("should validate that the kubeconfig in the SeedClientConnection is not set", func() {
				gardenletConfiguration.SeedClientConnection = &gardenletconfigv1alpha1.SeedClientConnection{
					ClientConnectionConfiguration: componentbaseconfigv1alpha1.ClientConnectionConfiguration{
						Kubeconfig: "path",
					},
				}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("componentConfiguration.seedClientConnection.kubeconfig"),
					})),
				))
			})

			It("should make sure that the Gardenlet configuration is validated by the Gardenlet validation", func() {
				// only pick one required Gardenlet component configuration to show that the configuration is indeed verified
				// neither seedSelector nor seedConfig are provided. One of them has to be set to
				// pass the validation of the GardenletConfiguration
				gardenletConfiguration.SeedConfig = nil
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(HaveLen(1))
			})
		})
		Context("validate the gardenlet deployment configuration", func() {
			It("should validate that the image is not set", func() {
				landscaperGardenletImport.DeploymentConfiguration.Image = &seedmanagement.Image{}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("deploymentConfiguration.image"),
					})),
				))
			})

			It("should validate that the replica count is not negative", func() {
				landscaperGardenletImport.DeploymentConfiguration.ReplicaCount = pointer.Int32(-1)
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("deploymentConfiguration.replicaCount"),
					})),
				))
			})

			It("should validate that the RevisionHistoryLimit is not negative", func() {
				landscaperGardenletImport.DeploymentConfiguration.RevisionHistoryLimit = pointer.Int32(-1)
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("deploymentConfiguration.revisionHistoryLimit"),
					})),
				))
			})

			It("should validate that the service account name is valid", func() {
				landscaperGardenletImport.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("deploymentConfiguration.serviceAccountName"),
					})),
				))
			})

			It("should validate that the pod labels are valid", func() {
				landscaperGardenletImport.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("deploymentConfiguration.podLabels"),
					})),
				))
			})

			It("should validate that the podAnnotations are valid", func() {
				landscaperGardenletImport.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("deploymentConfiguration.podAnnotations"),
					})),
				))
			})
		})

		Context("validate the backup configuration", func() {
			It("do not require landscaperImports.SeedBackupCredentials to be set if Seed is configured with Backup", func() {
				gardenletConfiguration.SeedConfig.Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider: "a",
					SecretRef: corev1.SecretReference{
						Name:      "a",
						Namespace: "b",
					},
				}

				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(BeEmpty())
			})

			It("require field componentConfiguration.SeedConfig.Spec.Backup in imports to be set when field seedBackup is configured", func() {
				landscaperGardenletImport.SeedBackupCredentials = &json.RawMessage{}

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("componentConfiguration.seedConfig.spec.backup"),
					})),
				))
			})

			It("Seed Backup provider missing", func() {
				gardenletConfiguration.SeedConfig.Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider: "",
					SecretRef: corev1.SecretReference{
						Name: "a",
					},
				}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				landscaperGardenletImport.SeedBackupCredentials = &json.RawMessage{}

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("componentConfiguration.seedConfig.spec.backup.provider"),
					})),
				))
			})

			It("Seed Backup credentials missing", func() {
				gardenletConfiguration.SeedConfig.Spec.Backup = &gardencorev1beta1.SeedBackup{
					Provider:  "x",
					SecretRef: corev1.SecretReference{},
				}
				landscaperGardenletImport.ComponentConfiguration = &gardenletConfiguration

				landscaperGardenletImport.SeedBackupCredentials = &json.RawMessage{}

				errorList := ValidateLandscaperImports(landscaperGardenletImport)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("componentConfiguration.seedConfig.spec.backup.secretRef.Name"),
					})),
				))
			})
		})

	})
})
