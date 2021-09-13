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
	"time"

	"github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports"
	. "github.com/gardener/gardener/landscaper/pkg/controlplane/apis/imports/validation"
	hvpav1alpha1 "github.com/gardener/hvpa-controller/api/v1alpha1"
	landscaperv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apiserverv1 "k8s.io/apiserver/pkg/apis/apiserver/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	apiserverconfigv1 "k8s.io/apiserver/pkg/apis/config/v1"
	"k8s.io/utils/pointer"
)

var _ = Describe("ValidateAPIServer", func() {
	var (
		gardenerAPIServer imports.GardenerAPIServer
		path              = field.NewPath("apiserver")

		caAPIServerTLS                = GenerateCACertificate("gardener.cloud:system:apiserver")
		caAPIServerString             = string(caAPIServerTLS.CertificatePEM)
		apiServerTLSServingCert       = GenerateTLSServingCertificate(&caAPIServerTLS)
		apiServerTLSServingCertString = string(apiServerTLSServingCert.CertificatePEM)
		apiServerTLSServingKeyString  = string(apiServerTLSServingCert.PrivateKeyPEM)

		caEtcdTLS      = GenerateCACertificate("gardener.cloud:system:etcd-virtual")
		caEtcdString   = string(caEtcdTLS.CertificatePEM)
		etcdClientCert = GenerateClientCertificate(&caEtcdTLS)
		etcdCertString = string(etcdClientCert.CertificatePEM)
		etcdKeyString  = string(etcdClientCert.PrivateKeyPEM)
	)

	BeforeEach(func() {
		gardenerAPIServer = imports.GardenerAPIServer{
			DeploymentConfiguration: &imports.APIServerDeploymentConfiguration{
				CommonDeploymentConfiguration: imports.CommonDeploymentConfiguration{
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
				LivenessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/healthz",
							Scheme: corev1.URISchemeHTTPS,
							Port:   intstr.FromInt(10259),
						},
					},
					SuccessThreshold:    1,
					FailureThreshold:    2,
					InitialDelaySeconds: 15,
					PeriodSeconds:       10,
					TimeoutSeconds:      15,
				},
				ReadinessProbe: &corev1.Probe{
					Handler: corev1.Handler{
						HTTPGet: &corev1.HTTPGetAction{
							Path:   "/readyz",
							Scheme: corev1.URISchemeHTTPS,
							Port:   intstr.FromInt(10260),
						},
					},
					SuccessThreshold:    1,
					FailureThreshold:    2,
					InitialDelaySeconds: 15,
					PeriodSeconds:       10,
					TimeoutSeconds:      15,
				},
				MinReadySeconds: pointer.Int32(10),
				Hvpa: &imports.HVPAConfiguration{
					Enabled:               nil,
					MaintenanceTimeWindow: nil,
					HVPAConfigurationHPA:  nil,
					HVPAConfigurationVPA:  nil,
				},
			},
			ComponentConfiguration: imports.APIServerComponentConfiguration{
				ClusterIdentity: pointer.String("identity"),
				Encryption:      nil,
				Etcd: imports.APIServerEtcdConfiguration{
					Url:        "etcd-virtual-garden:2237",
					CABundle:   &caEtcdString,
					ClientCert: &etcdCertString,
					ClientKey:  &etcdKeyString,
				},
				CABundle: &caAPIServerString,
				TLS: &imports.TLSServer{
					Crt: apiServerTLSServingCertString,
					Key: apiServerTLSServingKeyString,
				},
				FeatureGates:                 nil,
				Admission:                    nil,
				GoAwayChance:                 nil,
				Http2MaxStreamsPerConnection: nil,
				ShutdownDelayDuration:        nil,
				Requests:                     nil,
				WatchCacheSize:               nil,
				Audit:                        nil,
			},
		}
	})

	Describe("#Validate Component Configuration", func() {
		It("should allow valid configurations", func() {
			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(BeEmpty())
		})

		Context("#ValidateAPIServerETCDConfiguration", func() {
			It("should forbid invalid TLS configuration - etcd url is not set", func() {
				gardenerAPIServer.ComponentConfiguration.Etcd.Url = ""
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("apiserver.componentConfiguration.etcd.url"),
					})),
				))
			})

			It("should forbid invalid TLS configuration - etcd CA is invalid", func() {
				gardenerAPIServer.ComponentConfiguration.Etcd.CABundle = pointer.String("invalid")
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("apiserver.componentConfiguration.etcd.caBundle"),
					})),
				))
			})

			It("should forbid invalid TLS configuration - etcd client certificate is invalid", func() {
				gardenerAPIServer.ComponentConfiguration.Etcd.ClientCert = pointer.String("invalid")
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("apiserver.componentConfiguration.etcd.clientCert"),
					})),
				))
			})

			It("should forbid invalid TLS configuration - etcd client certificate is invalid", func() {
				gardenerAPIServer.ComponentConfiguration.Etcd.ClientKey = pointer.String("invalid")
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("apiserver.componentConfiguration.etcd.clientKey"),
					})),
				))
			})
		})

		Context("TLS Serving", func() {
			It("should forbid invalid TLS configuration - TLS serving certificate is not signed by the provided CA", func() {
				someUnknownCA := string(GenerateCACertificate("gardener.cloud:system:unknown").CertificatePEM)
				gardenerAPIServer.ComponentConfiguration.CABundle = pointer.String(someUnknownCA)
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.tls.crt"),
						"Detail": Equal("failed to verify the TLS serving certificate against the given CA bundle: x509: certificate signed by unknown authority"),
					})),
				))
			})

			It("should forbid invalid TLS configuration - TLS serving certificate is invalid", func() {
				gardenerAPIServer.ComponentConfiguration.TLS.Crt = "invalid"
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.tls.crt"),
						"Detail": Equal("the TLS certificate is not PEM encoded"),
					})),
				))
			})

			It("should forbid invalid TLS configuration - TLS key is invalid", func() {
				gardenerAPIServer.ComponentConfiguration.TLS.Key = "invalid"
				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.tls.key"),
						"Detail": ContainSubstring("the TLS certificate provided is not a valid PEM encoded X509 private key"),
					})),
				))
			})
		})

		Context("#ValidateAPIServerEncryptionConfiguration", func() {
			It("should forbid an invalid encryption configuration", func() {
				gardenerAPIServer.ComponentConfiguration.Encryption = &apiserverconfigv1.EncryptionConfiguration{
					Resources: []apiserverconfigv1.ResourceConfiguration{
						{
							Resources: []string{},
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.encryption"),
						"Detail": ContainSubstring("failed to validate API server encryption config"),
					})),
				))
			})
		})

		Context("#ValidateAPIServerAdmission", func() {
			It("Admission plugin name must be set", func() {
				gardenerAPIServer.ComponentConfiguration.Admission = &imports.APIServerAdmissionConfiguration{
					Plugins: []apiserverv1.AdmissionPluginConfiguration{
						{
							Name: "",
							Configuration: &runtime.Unknown{
								Raw: []byte("whats that"),
							},
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.admission.plugins[0].name"),
						"Detail": ContainSubstring("Admission plugin name must be set"),
					})),
				))
			})

			It("Admission plugin path must be set", func() {
				gardenerAPIServer.ComponentConfiguration.Admission = &imports.APIServerAdmissionConfiguration{
					Plugins: []apiserverv1.AdmissionPluginConfiguration{
						{
							Name: "name",
							Path: "my path",
							Configuration: &runtime.Unknown{
								Raw: []byte("whats that"),
							},
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.admission.plugins[0].path"),
						"Detail": ContainSubstring("Admission plugin path must not be set"),
					})),
				))
			})

			It("Admission plugin path must be set", func() {
				gardenerAPIServer.ComponentConfiguration.Admission = &imports.APIServerAdmissionConfiguration{
					Plugins: []apiserverv1.AdmissionPluginConfiguration{
						{
							Name: "name",
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.admission.plugins[0].configuration"),
						"Detail": ContainSubstring("Admission plugin configuration must be set"),
					})),
				))
			})
		})

		It("The goAwayChance can be in the interval [0, 0.02]", func() {
			gardenerAPIServer.ComponentConfiguration.GoAwayChance = pointer.Float32(-1)

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.componentConfiguration.goAwayChance"),
					"Detail": ContainSubstring("The goAwayChance can be in the interval [0, 0.02]"),
				})),
			))
		})

		It("The goAwayChance can be in the interval [0, 0.02]", func() {
			gardenerAPIServer.ComponentConfiguration.GoAwayChance = pointer.Float32(0.03)

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.componentConfiguration.goAwayChance"),
					"Detail": ContainSubstring("The goAwayChance can be in the interval [0, 0.02]"),
				})),
			))
		})

		It("The Http2MaxStreamsPerConnection cannot be negative", func() {
			gardenerAPIServer.ComponentConfiguration.Http2MaxStreamsPerConnection = pointer.Int32(-1)

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.componentConfiguration.http2MaxStreamsPerConnection"),
					"Detail": ContainSubstring("The Http2MaxStreamsPerConnection cannot be negative"),
				})),
			))
		})

		It("The shutdownDelayDuration cannot be negative", func() {
			gardenerAPIServer.ComponentConfiguration.ShutdownDelayDuration = &metav1.Duration{
				Duration: -1 * time.Second,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.componentConfiguration.shutdownDelayDuration"),
					"Detail": ContainSubstring("must be non-negative"),
				})),
			))
		})

		Context("#ValidateAPIServerRequests", func() {
			It("The MaxNonMutatingInflight field cannot be negative", func() {
				var i int = -1
				gardenerAPIServer.ComponentConfiguration.Requests = &imports.APIServerRequests{
					MaxNonMutatingInflight: &i,
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.requests.maxNonMutatingInflight"),
						"Detail": ContainSubstring("The MaxNonMutatingInflight field cannot be negative"),
					})),
				))
			})

			It("The MaxMutatingInflight field cannot be negative", func() {
				var i int = -1
				gardenerAPIServer.ComponentConfiguration.Requests = &imports.APIServerRequests{
					MaxMutatingInflight: &i,
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.requests.maxMutatingInflight"),
						"Detail": ContainSubstring("The MaxMutatingInflight field cannot be negative"),
					})),
				))
			})

			It("The MinTimeout field must be a positive duration", func() {
				gardenerAPIServer.ComponentConfiguration.Requests = &imports.APIServerRequests{
					MinTimeout: &metav1.Duration{
						Duration: -1 * time.Second,
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.requests.minTimeout"),
						"Detail": ContainSubstring("must be non-negative"),
					})),
				))
			})

			It("The timeout field must be a positive duration", func() {
				gardenerAPIServer.ComponentConfiguration.Requests = &imports.APIServerRequests{
					Timeout: &metav1.Duration{
						Duration: -1 * time.Second,
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.requests.timeout"),
						"Detail": ContainSubstring("must be non-negative"),
					})),
				))
			})
		})

		Context("ValidateAPIServerWatchCache", func() {
			It("The default watch cache size cannot be negative", func() {
				gardenerAPIServer.ComponentConfiguration.WatchCacheSize = &imports.APIServerWatchCacheConfiguration{
					DefaultSize: pointer.Int32(-1),
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.watchCacheSize.defaultSize"),
						"Detail": ContainSubstring("The default watch cache size cannot be negative"),
					})),
				))
			})

			It("The default watch cache size cannot be negative", func() {
				gardenerAPIServer.ComponentConfiguration.WatchCacheSize = &imports.APIServerWatchCacheConfiguration{
					Resources: []imports.WatchCacheSizeResource{
						{
							ApiGroup: "",
							Resource: "xy",
							Size:     0,
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.watchCacheSize.resources[0].apiGroup"),
						"Detail": ContainSubstring("The API Group of the watch cache resource cannot be empty"),
					})),
				))
			})

			It("The name of the watch cache resource cannot be empty", func() {
				gardenerAPIServer.ComponentConfiguration.WatchCacheSize = &imports.APIServerWatchCacheConfiguration{
					Resources: []imports.WatchCacheSizeResource{
						{
							ApiGroup: "xy",
							Resource: "",
							Size:     0,
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.watchCacheSize.resources[0].resource"),
						"Detail": ContainSubstring("The name of the watch cache resource cannot be empty"),
					})),
				))
			})

			It("The size of the watch cache resource cannot be negative", func() {
				gardenerAPIServer.ComponentConfiguration.WatchCacheSize = &imports.APIServerWatchCacheConfiguration{
					Resources: []imports.WatchCacheSizeResource{
						{
							ApiGroup: "xy",
							Resource: "xy",
							Size:     -1,
						},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.watchCacheSize.resources[0].size"),
						"Detail": ContainSubstring("The size of the watch cache resource cannot be negative"),
					})),
				))
			})
		})

		Context("#ValidateAPIServerAuditConfiguration", func() {
			It("The dynamic auditing feature gate has to be set", func() {
				gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
					DynamicConfiguration: pointer.Bool(true),
				}

				gardenerAPIServer.ComponentConfiguration.FeatureGates = append(gardenerAPIServer.ComponentConfiguration.FeatureGates, "DynamicAuditing=false")

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.audit.dynamicConfiguration"),
						"Detail": ContainSubstring("DynamicConfiguration requires the feature gate 'DynamicAuditing=true' to be set"),
					})),
				))
			})

			It("The validate the dynamic auditing confguration successfully", func() {
				gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
					DynamicConfiguration: pointer.Bool(true),
				}

				gardenerAPIServer.ComponentConfiguration.FeatureGates = append(gardenerAPIServer.ComponentConfiguration.FeatureGates, "DynamicAuditing=true")

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(BeEmpty())
			})

			It("Should validate the provided audit policy", func() {
				gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
					Policy: &auditv1.Policy{
						OmitStages: []auditv1.Stage{"no stage at all"},
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.componentConfiguration.audit.policy"),
						"Detail": ContainSubstring("Audit policy is invalid - the following validation errors occured"),
					})),
				))
			})

			Context("Log backend", func() {
				It("Should validate the log configuration - format is invalid", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode: pointer.String("invalid mode"),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.mode"),
							"Detail": ContainSubstring("The mode strategy for sending audit events must be one of [batch,blocking,blocking-strict]"),
						})),
					))
				})

				It("Should validate the log configuration - batch - buffer  size", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode:            pointer.String("batch"),
								BatchBufferSize: pointer.Int32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.batchBufferSize"),
							"Detail": ContainSubstring("The BatchBufferSize must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - BatchMaxSize", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode:         pointer.String("batch"),
								BatchMaxSize: pointer.Int32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.batchMaxSize"),
							"Detail": ContainSubstring("The BatchMaxSize must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - batchMaxWait", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode: pointer.String("batch"),
								BatchMaxWait: &metav1.Duration{
									Duration: -1 * time.Second,
								},
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.batchMaxWait"),
							"Detail": ContainSubstring("must be non-negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - batchThrottleBurst", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode:               pointer.String("batch"),
								BatchThrottleBurst: pointer.Int32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.batchThrottleBurst"),
							"Detail": ContainSubstring("The BatchThrottleBurst must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - batchThrottleQPS", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode:             pointer.String("batch"),
								BatchThrottleQPS: pointer.Float32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.batchThrottleQPS"),
							"Detail": ContainSubstring("The BatchThrottleQPS must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - truncateMaxBatchSize", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Mode:                 pointer.String("batch"),
								TruncateMaxBatchSize: pointer.Int32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.truncateMaxBatchSize"),
							"Detail": ContainSubstring("The TruncateMaxBatchSize must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - truncateMaxEventSize", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								TruncateMaxEventSize: pointer.Int32(-1),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.truncateMaxEventSize"),
							"Detail": ContainSubstring("The TruncateMaxEventSize must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - version", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								Version: pointer.String(""),
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.version"),
							"Detail": ContainSubstring("The version name of the API group and version used for serializing audit events must not be empty"),
						})),
					))
				})

				It("Should validate the log configuration - batch - format", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							Format: pointer.String("invalid-format"),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.format"),
							"Detail": ContainSubstring("The log format of the Audit log must be [legacy,json]"),
						})),
					))
				})

				It("Should validate the log configuration - batch - maxAge", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							MaxAge: pointer.Int32(-1),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.maxAge"),
							"Detail": ContainSubstring("The maximum age configured for Audit logs must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - MaxBackup", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							MaxBackup: pointer.Int32(-1),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.maxBackup"),
							"Detail": ContainSubstring("The maximum number of old audit log files to retain must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - MaxSize", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							MaxSize: pointer.Int32(-1),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.maxSize"),
							"Detail": ContainSubstring("The maximum size of audit log files must not be negative"),
						})),
					))
				})

				It("Should validate the log configuration - batch - MaxSize", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							Path: pointer.String(""),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.log.path"),
							"Detail": ContainSubstring("The filepath to store the audit log file must not be empty"),
						})),
					))
				})

				It("Should validate the log configuration successfully", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Log: &imports.APIServerAuditLogBackend{
							APIServerAuditCommonBackendConfiguration: imports.APIServerAuditCommonBackendConfiguration{
								BatchBufferSize: pointer.Int32(1),
								BatchMaxSize:    pointer.Int32(1),
								BatchMaxWait: &metav1.Duration{
									Duration: 1 * time.Second,
								},
								BatchThrottleBurst:   pointer.Int32(1),
								BatchThrottleEnable:  pointer.Bool(true),
								BatchThrottleQPS:     pointer.Float32(3.0),
								Mode:                 pointer.String("batch"),
								TruncateEnabled:      pointer.Bool(true),
								TruncateMaxBatchSize: pointer.Int32(1),
								TruncateMaxEventSize: pointer.Int32(1),
								Version:              pointer.String("some valid  version"),
							},
							Format:    pointer.String("json"),
							MaxAge:    pointer.Int32(1),
							MaxBackup: pointer.Int32(1),
							MaxSize:   pointer.Int32(1),
							Path:      pointer.String("path"),
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(BeEmpty())
				})
			})

			Context("Webhook backend", func() {
				It("Should validate the webhook configuration successfully", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Webhook: &imports.APIServerAuditWebhookBackend{
							Kubeconfig: landscaperv1alpha1.Target{
								Spec: landscaperv1alpha1.TargetSpec{
									Configuration: landscaperv1alpha1.AnyJSON{
										RawMessage: []byte("awesome kubeconfig"),
									},
								},
							},
							InitialBackoff: &metav1.Duration{
								Duration: 1 * time.Second,
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(BeEmpty())
				})

				It("Should validate the webhook configuration - invalid kubeconfig", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Webhook: &imports.APIServerAuditWebhookBackend{
							Kubeconfig: landscaperv1alpha1.Target{},
							InitialBackoff: &metav1.Duration{
								Duration: 1 * time.Second,
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.webhook.kubeconfig"),
							"Detail": ContainSubstring("The kubeconfig for the external audit log backend must be set"),
						})),
					))
				})

				It("Should validate the webhook configuration - invalid kubeconfig", func() {
					gardenerAPIServer.ComponentConfiguration.Audit = &imports.APIServerAuditConfiguration{
						Webhook: &imports.APIServerAuditWebhookBackend{
							Kubeconfig: landscaperv1alpha1.Target{
								Spec: landscaperv1alpha1.TargetSpec{
									Configuration: landscaperv1alpha1.AnyJSON{
										RawMessage: []byte("awesome kubeconfig"),
									},
								},
							},
							InitialBackoff: &metav1.Duration{
								Duration: -1 * time.Second,
							},
						},
					}

					errorList := ValidateAPIServer(gardenerAPIServer, path)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("apiserver.componentConfiguration.audit.webhook.initialBackoff"),
							"Detail": ContainSubstring("must be non-negative"),
						})),
					))
				})
			})
		})
	})

	Describe("#ValidateAPIServerDeploymentConfiguration", func() {
		It("should validate that the replica count is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReplicaCount = pointer.Int32(-1)

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("apiserver.deploymentConfiguration.replicaCount"),
				})),
			))
		})

		It("should validate that the service account name is valid", func() {
			gardenerAPIServer.DeploymentConfiguration.ServiceAccountName = pointer.String("x121ä232..")

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("apiserver.deploymentConfiguration.serviceAccountName"),
				})),
			))
		})

		It("should validate that the pod labels are valid", func() {
			gardenerAPIServer.DeploymentConfiguration.PodLabels = map[string]string{"foo!": "bar"}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("apiserver.deploymentConfiguration.podLabels"),
				})),
			))
		})

		It("should validate that the podAnnotations are valid", func() {
			gardenerAPIServer.DeploymentConfiguration.PodAnnotations = map[string]string{"bar@": "baz"}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("apiserver.deploymentConfiguration.podAnnotations"),
				})),
			))
		})

		It("should validate that the liveness probe - initialDelaySeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.LivenessProbe = &corev1.Probe{
				InitialDelaySeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.livenessProbe.initialDelaySeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the liveness probe - periodSeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.LivenessProbe = &corev1.Probe{
				PeriodSeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.livenessProbe.periodSeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the liveness probe - successThreshold is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.LivenessProbe = &corev1.Probe{
				SuccessThreshold: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.livenessProbe.successThreshold"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the liveness probe - failureThreshold is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.LivenessProbe = &corev1.Probe{
				FailureThreshold: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.livenessProbe.failureThreshold"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the liveness probe - timeoutSeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.LivenessProbe = &corev1.Probe{
				TimeoutSeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.livenessProbe.timeoutSeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the readiness probe - initialDelaySeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReadinessProbe = &corev1.Probe{
				InitialDelaySeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.readinessProbe.initialDelaySeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the readiness probe - periodSeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReadinessProbe = &corev1.Probe{
				PeriodSeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.readinessProbe.periodSeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the liveness probe - successThreshold is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReadinessProbe = &corev1.Probe{
				SuccessThreshold: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.readinessProbe.successThreshold"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the readiness probe - failureThreshold is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReadinessProbe = &corev1.Probe{
				FailureThreshold: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.readinessProbe.failureThreshold"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the readiness probe - timeoutSeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.ReadinessProbe = &corev1.Probe{
				TimeoutSeconds: -1,
			}

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.readinessProbe.timeoutSeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		It("should validate that the MinReadySeconds is not negative", func() {
			gardenerAPIServer.DeploymentConfiguration.MinReadySeconds = pointer.Int32(-1)

			errorList := ValidateAPIServer(gardenerAPIServer, path)
			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("apiserver.deploymentConfiguration.minReadySeconds"),
					"Detail": ContainSubstring("value must not be negative"),
				})),
			))
		})

		Context("#ValidateHVPA", func() {
			It("should successfully validate HVPA", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: "230000+0000",
						End:   "000000+0000",
					},
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						MinReplicas:                    pointer.Int32(1),
						MaxReplicas:                    pointer.Int32(2),
						TargetAverageUtilizationCpu:    pointer.Int32(50),
						TargetAverageUtilizationMemory: pointer.Int32(30),
					},
					HVPAConfigurationVPA: &imports.HVPAConfigurationVPA{
						ScaleUpMode:   pointer.String("Auto"),
						ScaleDownMode: pointer.String("Auto"),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(BeEmpty())
			})

			It("invalid maintenance begin", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: "0",
						End:   "000000+0000",
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.maintenanceTimeWindow.begin"),
						"Detail": ContainSubstring("could not parse the value into the maintenanceTime format"),
					})),
				))
			})

			It("invalid maintenance end", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					MaintenanceTimeWindow: &hvpav1alpha1.MaintenanceTimeWindow{
						Begin: "230000+0000",
						End:   "xy",
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.maintenanceTimeWindow.end"),
						"Detail": ContainSubstring("could not parse the value into the maintenanceTime format"),
					})),
				))
			})

			It("HVPAConfigurationVPA - invalid ScaleUpMode", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),

					HVPAConfigurationVPA: &imports.HVPAConfigurationVPA{
						ScaleUpMode:   pointer.String("xa"),
						ScaleDownMode: pointer.String("Auto"),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationVPA.scaleUpMode"),
						"Detail": ContainSubstring("valid scale up modes are [Auto,Off,MaintenanceWindow]"),
					})),
				))
			})

			It("HVPAConfigurationVPA - invalid ScaleDownMode", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),

					HVPAConfigurationVPA: &imports.HVPAConfigurationVPA{
						ScaleUpMode:   pointer.String("Auto"),
						ScaleDownMode: pointer.String("xa"),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationVPA.scaleDownMode"),
						"Detail": ContainSubstring("valid scale down modes are [Auto,Off,MaintenanceWindow]"),
					})),
				))
			})

			It("HVPAConfigurationHPA - min replicas cannot be negative", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						MinReplicas:                    pointer.Int32(-1),
						MaxReplicas:                    pointer.Int32(2),
						TargetAverageUtilizationCpu:    pointer.Int32(50),
						TargetAverageUtilizationMemory: pointer.Int32(30),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.minReplicas"),
						"Detail": ContainSubstring("value cannot be negative"),
					})),
				))
			})

			It("HVPAConfigurationHPA - max replicas cannot be negative", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						MaxReplicas:                    pointer.Int32(-1),
						TargetAverageUtilizationCpu:    pointer.Int32(50),
						TargetAverageUtilizationMemory: pointer.Int32(30),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.maxReplicas"),
						"Detail": ContainSubstring("value cannot be negative"),
					})),
				))
			})

			It("HVPAConfigurationHPA - targetAverageUtilizationCpu must be in range [0, 100]", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						TargetAverageUtilizationCpu: pointer.Int32(-1),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.targetAverageUtilizationCpu"),
						"Detail": ContainSubstring("value is invalid"),
					})),
				))

				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						TargetAverageUtilizationCpu: pointer.Int32(101),
					},
				}

				errorList = ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.targetAverageUtilizationCpu"),
						"Detail": ContainSubstring("value is invalid"),
					})),
				))
			})

			It("HVPAConfigurationHPA - targetAverageUtilizationCpu must be in range [0, 100]", func() {
				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						TargetAverageUtilizationMemory: pointer.Int32(-1),
					},
				}

				errorList := ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.targetAverageUtilizationMemory"),
						"Detail": ContainSubstring("value is invalid"),
					})),
				))

				gardenerAPIServer.DeploymentConfiguration.Hvpa = &imports.HVPAConfiguration{
					Enabled: pointer.Bool(true),
					HVPAConfigurationHPA: &imports.HVPAConfigurationHPA{
						TargetAverageUtilizationMemory: pointer.Int32(101),
					},
				}

				errorList = ValidateAPIServer(gardenerAPIServer, path)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("apiserver.deploymentConfiguration.hvpa.hvpaConfigurationHPA.targetAverageUtilizationMemory"),
						"Detail": ContainSubstring("value is invalid"),
					})),
				))
			})
		})
	})
})
