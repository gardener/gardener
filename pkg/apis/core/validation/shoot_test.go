// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation_test

import (
	"fmt"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/ptr"

	"github.com/gardener/gardener/pkg/apis/core"
	v1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	"github.com/gardener/gardener/pkg/features"
	"github.com/gardener/gardener/pkg/utils/test"
	. "github.com/gardener/gardener/pkg/utils/test/matchers"
)

var _ = Describe("Shoot Validation Tests", func() {
	BeforeEach(func() {
		DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.CredentialsRotationWithoutWorkersRollout, true))
	})

	Describe("#ValidateShoot, #ValidateShootUpdate", func() {
		var (
			shoot *core.Shoot

			domain          = "my-cluster.example.com"
			dnsProviderType = "some-provider"
			secretName      = "some-secret"
			purpose         = core.ShootPurposeEvaluation
			addon           = core.Addon{
				Enabled: true,
			}

			maxSurge         = intstr.FromInt32(1)
			maxUnavailable   = intstr.FromInt32(0)
			systemComponents = &core.WorkerSystemComponents{
				Allow: true,
			}
			worker = core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				Minimum:          1,
				Maximum:          1,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
				UpdateStrategy:   ptr.To(core.AutoRollingUpdate),
			}
			invalidWorker = core.Worker{
				Name: "",
				Machine: core.Machine{
					Type:         "",
					Architecture: ptr.To("amd64"),
				},
				Minimum:          -1,
				Maximum:          -2,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
			invalidWorkerName = core.Worker{
				Name: "not_compliant",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				Minimum:          1,
				Maximum:          1,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
			invalidWorkerTooLongName = core.Worker{
				Name: "worker-name-is-too-long",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				Minimum:          1,
				Maximum:          1,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
			workerAutoScalingMinZero = core.Worker{
				Name: "cpu-worker",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				Minimum:          0,
				Maximum:          2,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
			workerAutoScalingMinMaxZero = core.Worker{
				Name: "cpu-worker",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				Minimum:          0,
				Maximum:          0,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
		)

		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: core.ShootSpec{
					Addons: &core.Addons{
						KubernetesDashboard: &core.KubernetesDashboard{
							Addon: addon,
						},
						NginxIngress: &core.NginxIngress{
							Addon: addon,
						},
					},
					CloudProfileName:  ptr.To("aws-profile"),
					Region:            "eu-west-1",
					SecretBindingName: ptr.To("my-secret"),
					Purpose:           &purpose,
					DNS: &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    &dnsProviderType,
								Primary: ptr.To(true),
							},
						},
						Domain: &domain,
					},
					Kubernetes: core.Kubernetes{
						Version: "1.30.3",
						ETCD: &core.ETCD{
							Main: &core.ETCDConfig{
								Autoscaling: &core.ControlPlaneAutoscaling{
									MinAllowed: map[corev1.ResourceName]resource.Quantity{
										"cpu": resource.MustParse("2"),
									},
								},
							},
							Events: &core.ETCDConfig{
								Autoscaling: &core.ControlPlaneAutoscaling{
									MinAllowed: map[corev1.ResourceName]resource.Quantity{
										"cpu": resource.MustParse("1"),
									},
								},
							},
						},
						KubeAPIServer: &core.KubeAPIServerConfig{
							OIDCConfig: &core.OIDCConfig{
								CABundle:       ptr.To("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       ptr.To("client-id"),
								GroupsClaim:    ptr.To("groups-claim"),
								GroupsPrefix:   ptr.To("groups-prefix"),
								IssuerURL:      ptr.To("https://some-endpoint.com"),
								UsernameClaim:  ptr.To("user-claim"),
								UsernamePrefix: ptr.To("user-prefix"),
								RequiredClaims: map[string]string{"foo": "bar"},
							},
							AdmissionPlugins: []core.AdmissionPlugin{
								{
									Name: "PodNodeSelector",
									Config: &runtime.RawExtension{
										Raw: []byte(`podNodeSelectorPluginConfig:
  clusterDefaultNodeSelector: <node-selectors-labels>
  namespace1: <node-selectors-labels>
	namespace2: <node-selectors-labels>`),
									},
								},
							},
							AuditConfig: &core.AuditConfig{
								AuditPolicy: &core.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: "audit-policy-config",
									},
								},
							},
							Autoscaling: &core.ControlPlaneAutoscaling{
								MinAllowed: map[corev1.ResourceName]resource.Quantity{
									"cpu":    resource.MustParse("20m"),
									"memory": resource.MustParse("200M"),
								},
							},
						},
						KubeControllerManager: &core.KubeControllerManagerConfig{
							NodeCIDRMaskSize: ptr.To[int32](22),
							HorizontalPodAutoscalerConfig: &core.HorizontalPodAutoscalerConfig{
								SyncPeriod: &metav1.Duration{Duration: 30 * time.Second},
								Tolerance:  ptr.To(0.1),
							},
						},
					},
					Networking: &core.Networking{
						Type: ptr.To("some-network-plugin"),
					},
					Provider: core.Provider{
						Type:    "aws",
						Workers: []core.Worker{worker},
					},
					Maintenance: &core.Maintenance{
						AutoUpdate: &core.MaintenanceAutoUpdate{
							KubernetesVersion: true,
						},
						TimeWindow: &core.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Monitoring: &core.Monitoring{
						Alerting: &core.Alerting{},
					},
					Tolerations: []core.Toleration{
						{Key: "foo"},
					},
				},
			}
		})

		DescribeTable("Shoot metadata",
			func(objectMeta metav1.ObjectMeta, matcher gomegatypes.GomegaMatcher) {
				shoot.ObjectMeta = objectMeta

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(matcher)
			},

			Entry("should forbid Shoot with empty metadata",
				metav1.ObjectMeta{},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("metadata.namespace"),
					})),
				),
			),
			Entry("should forbid Shoot with empty name",
				metav1.ObjectMeta{Name: "", Namespace: "my-namespace"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Shoot with '.' in the name (not a DNS-1123 label compliant name)",
				metav1.ObjectMeta{Name: "shoot.test", Namespace: "my-namespace"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Shoot with '_' in the name (not a DNS-1123 subdomain)",
				metav1.ObjectMeta{Name: "shoot_test", Namespace: "my-namespace"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
			Entry("should forbid Shoot with name containing two consecutive hyphens",
				metav1.ObjectMeta{Name: "sho--oot", Namespace: "my-namespace"},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("metadata.name"),
				}))),
			),
		)

		It("should forbid empty Shoot resources", func() {
			shoot := &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       core.ShootSpec{},
			}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("metadata.namespace"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.version"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloudProfile.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.region"),
				})),
			))
		})

		Context("#ValidateShootHAControlPlaneUpdate", func() {
			It("should pass as Shoot ControlPlane Spec with HA set to zone has not changed", func() {
				shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
				newShoot := prepareShootForUpdate(shoot)
				errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should pass as non-HA Shoot ControlPlane Spec has not changed", func() {
				newShoot := prepareShootForUpdate(shoot)
				errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should allow upgrading from non-HA to HA Shoot ControlPlane.HighAvailability Spec", func() {
				shoot.Spec.ControlPlane = &core.ControlPlane{}
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
				errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
				Expect(errorList).To(BeEmpty())
			})

			Context("shoot is scheduled", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = ptr.To("someSeed")
				})

				It("should forbid to change the Shoot ControlPlane spec", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"BadValue": Equal(core.FailureToleranceTypeNode),
							"Field":    Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
						})),
					))
				})

				It("should forbid to unset of Shoot ControlPlane", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = nil

					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
						})),
					))
				})
			})

			Context("shoot is not scheduled", func() {
				It("should allow to change the Shoot ControlPlane spec", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}

					Expect(ValidateShootHAConfigUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should allow to unset of Shoot ControlPlane", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = nil

					Expect(ValidateShootHAConfigUpdate(newShoot, shoot)).To(BeEmpty())
				})
			})

			Context("shoot is hibernated", func() {
				It("should not allow upgrading from non-HA to HA when Spec.Hibernation.Enabled is set to `true`", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeZone}}}
					newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}
					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
							"Detail": Equal("Shoot is currently hibernated and cannot be scaled up to HA. Please make sure your cluster has woken up before scaling it up to HA"),
						})),
					))
				})

				It("should not allow upgrading from non-HA to HA when Status.IsHibernation is set to `true`", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}
					newShoot.Status.IsHibernated = true
					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
							"Detail": Equal("Shoot is currently hibernated and cannot be scaled up to HA. Please make sure your cluster has woken up before scaling it up to HA"),
						})),
					))
				})

				It("should not allow upgrading from non-HA to HA when Spec.Hibernation.Enabled is set to `false` and Status.IsHibernation is set to `true`", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}
					newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
					newShoot.Status.IsHibernated = true
					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
							"Detail": Equal("Shoot is currently hibernated and cannot be scaled up to HA. Please make sure your cluster has woken up before scaling it up to HA"),
						})),
					))
				})

				It("should allow upgrading from non-HA to HA when Spec.Hibernation.Enabled is set to `false` and Status.IsHibernation is set to `false`", func() {
					shoot.Spec.ControlPlane = &core.ControlPlane{}
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: core.FailureToleranceTypeNode}}}
					newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
					newShoot.Status.IsHibernated = false
					errorList := ValidateShootHAConfigUpdate(newShoot, shoot)
					Expect(errorList).To(BeEmpty())
				})
			})
		})

		Context("#ValidateShootHAConfig", func() {
			It("should forbid to set unsupported failure tolerance type", func() {
				shoot.Spec.ControlPlane = &core.ControlPlane{}
				unsupportedFailureTolerance := core.FailureToleranceType("not-supported-value")
				shoot.Spec.ControlPlane = &core.ControlPlane{HighAvailability: &core.HighAvailability{FailureTolerance: core.FailureTolerance{Type: unsupportedFailureTolerance}}}
				errorList := ValidateShootHAConfig(shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("spec.controlPlane.highAvailability.failureTolerance.type"),
					})),
				))
			})
		})

		Context("#ValidateForceDeletion", func() {
			It("should not allow setting the force-deletion annotation if the Shoot does not have a deletionTimestamp", func() {
				newShoot := prepareShootForUpdate(shoot)

				metav1.SetMetaDataAnnotation(&newShoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "1")

				Expect(ValidateForceDeletion(newShoot, shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[confirmation.gardener.cloud/force-deletion]"),
						"Detail": Equal("force-deletion annotation cannot be set when Shoot deletionTimestamp is nil"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[confirmation.gardener.cloud/force-deletion]"),
						"Detail": Equal("force-deletion annotation cannot be set when Shoot status does not contain one of these error codes: [ERR_CLEANUP_CLUSTER_RESOURCES ERR_CONFIGURATION_PROBLEM ERR_INFRA_DEPENDENCIES ERR_INFRA_UNAUTHENTICATED ERR_INFRA_UNAUTHORIZED]"),
					})),
				))
			})

			It("should not allow setting the force-deletion annotation if the Shoot status does not have an error code", func() {
				newShoot := prepareShootForUpdate(shoot)

				metav1.SetMetaDataAnnotation(&newShoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "T")
				newShoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				Expect(ValidateForceDeletion(newShoot, shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[confirmation.gardener.cloud/force-deletion]"),
						"Detail": Equal("force-deletion annotation cannot be set when Shoot status does not contain one of these error codes: [ERR_CLEANUP_CLUSTER_RESOURCES ERR_CONFIGURATION_PROBLEM ERR_INFRA_DEPENDENCIES ERR_INFRA_UNAUTHENTICATED ERR_INFRA_UNAUTHORIZED]"),
					})),
				))
			})

			It("should not allow setting the force-deletion annotation if the Shoot status does not have a required error code", func() {
				newShoot := prepareShootForUpdate(shoot)

				metav1.SetMetaDataAnnotation(&newShoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "T")
				newShoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				newShoot.Status = core.ShootStatus{
					LastErrors: []core.LastError{
						{
							Codes: []core.ErrorCode{core.ErrorProblematicWebhook},
						},
					},
				}

				Expect(ValidateForceDeletion(newShoot, shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[confirmation.gardener.cloud/force-deletion]"),
						"Detail": Equal("force-deletion annotation cannot be set when Shoot status does not contain one of these error codes: [ERR_CLEANUP_CLUSTER_RESOURCES ERR_CONFIGURATION_PROBLEM ERR_INFRA_DEPENDENCIES ERR_INFRA_UNAUTHENTICATED ERR_INFRA_UNAUTHORIZED]"),
					})),
				))
			})

			It("should not do anything if the both new and old Shoot have the annotation", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
				shoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				newShoot := shoot.DeepCopy()

				Expect(ValidateForceDeletion(newShoot, shoot)).To(BeEmpty())
			})

			It("should forbid to remove the annotation once set", func() {
				newShoot := shoot.DeepCopy()

				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
				newShoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}

				err := ValidateForceDeletion(newShoot, shoot)
				Expect(err).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("metadata.annotations[confirmation.gardener.cloud/force-deletion]"),
					"Detail": Equal("force-deletion annotation cannot be removed once set"),
				}))))
			})

			It("should allow setting the force-deletion annotation if the Shoot has a deletionTimestamp and the status has a required ErrorCode", func() {
				newShoot := shoot.DeepCopy()
				metav1.SetMetaDataAnnotation(&newShoot.ObjectMeta, v1beta1constants.AnnotationConfirmationForceDeletion, "true")
				newShoot.DeletionTimestamp = &metav1.Time{Time: time.Now()}
				newShoot.Status = core.ShootStatus{
					LastErrors: []core.LastError{
						{
							Codes: []core.ErrorCode{core.ErrorConfigurationProblem},
						},
					},
				}

				Expect(ValidateForceDeletion(newShoot, shoot)).To(BeEmpty())
			})
		})

		Context("exposure class", func() {
			It("should pass as exposure class is not changed", func() {
				shoot.Spec.ExposureClassName = ptr.To("exposure-class-1")
				newShoot := prepareShootForUpdate(shoot)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid to change the exposure class", func() {
				shoot.Spec.ExposureClassName = ptr.To("exposure-class-1")
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.ExposureClassName = ptr.To("exposure-class-2")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.exposureClassName"),
					})),
				))
			})
		})

		DescribeTable("purpose validation",
			func(purpose core.ShootPurpose, namespace string, matcher gomegatypes.GomegaMatcher) {
				shootCopy := shoot.DeepCopy()
				shootCopy.Namespace = namespace
				shootCopy.Spec.Purpose = &purpose
				shootCopy.Spec.Addons = nil
				errorList := ValidateShoot(shootCopy)
				Expect(errorList).To(matcher)
			},

			Entry("evaluation purpose", core.ShootPurposeEvaluation, "dev", BeEmpty()),
			Entry("testing purpose", core.ShootPurposeTesting, "dev", BeEmpty()),
			Entry("development purpose", core.ShootPurposeDevelopment, "dev", BeEmpty()),
			Entry("production purpose", core.ShootPurposeProduction, "dev", BeEmpty()),
			Entry("infrastructure purpose in garden namespace", core.ShootPurposeInfrastructure, "garden", BeEmpty()),
			Entry("infrastructure purpose in other namespace", core.ShootPurposeInfrastructure, "dev", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.purpose"),
			})))),
			Entry("unknown purpose", core.ShootPurpose("foo"), "dev", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.purpose"),
			})))),
		)

		Context("Addons validation", func() {
			DescribeTable("addons validation",
				func(purpose core.ShootPurpose, allowed bool) {
					shootCopy := shoot.DeepCopy()
					shootCopy.Spec.Purpose = &purpose

					errorList := ValidateShoot(shootCopy)

					if allowed {
						Expect(errorList).To(BeEmpty())
					} else {
						Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("spec.addons"),
						}))))
					}
				},
				Entry("should allow addons on evaluation shoots", core.ShootPurposeEvaluation, true),
				Entry("should forbid addons on testing shoots", core.ShootPurposeTesting, false),
				Entry("should forbid addons on development shoots", core.ShootPurposeDevelopment, false),
				Entry("should forbid addons on production shoots", core.ShootPurposeProduction, false),
			)

			It("should forbid addon configuration if the shoot is workerless", func() {
				shoot.Spec.Provider.Workers = []core.Worker{}
				shoot.Spec.Addons = &core.Addons{}
				shoot.Spec.Kubernetes.KubeControllerManager = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.addons"),
					"Detail": ContainSubstring("addons cannot be enabled for Workerless Shoot clusters"),
				}))))
			})

			It("should forbid unsupported addon configuration", func() {
				shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = ptr.To("does-not-exist")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.addons.kubernetesDashboard.authenticationMode"),
				}))))
			})

			It("should allow external traffic policies 'Cluster' for nginx-ingress", func() {
				v := corev1.ServiceExternalTrafficPolicyCluster
				shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should allow external traffic policies 'Local' for nginx-ingress", func() {
				v := corev1.ServiceExternalTrafficPolicyLocal
				shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid unsupported external traffic policies for nginx-ingress", func() {
				v := corev1.ServiceExternalTrafficPolicy("something-else")
				shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.addons.nginxIngress.externalTrafficPolicy"),
				}))))
			})
		})

		It("should forbid unsupported specification (provider independent)", func() {
			shoot.Spec.CloudProfileName = nil
			shoot.Spec.Region = ""
			shoot.Spec.SecretBindingName = ptr.To("")
			shoot.Spec.SeedName = ptr.To("")
			shoot.Spec.SeedSelector = &core.SeedSelector{
				LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
			}
			shoot.Spec.Provider.Type = ""

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloudProfile.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretBindingName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedSelector.matchLabels"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
			))
		})

		Context("SecretBindingName/CredentialsBinding validation", func() {
			It("should forbid adding secretBindingName in case of workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.SecretBindingName = ptr.To("foo")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.secretBindingName"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))
			})

			It("should forbid adding credentialsBindingName in case of workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.CredentialsBindingName = ptr.To("foo")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.credentialsBindingName"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))
			})

			It("should allow nil secretBindingName in case of workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Addons = nil
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil
				shoot.Spec.Networking = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow nil credentialsBindingName in case of workerless shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Addons = nil
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.CredentialsBindingName = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil
				shoot.Spec.Networking = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid setting both secretBindingName and credentialsBindingName", func() {
				shoot.Spec.SecretBindingName = ptr.To("foo")
				shoot.Spec.CredentialsBindingName = ptr.To("foo")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.secretBindingName"),
					"Detail": Equal("is incompatible with credentialsBindingName"),
				}))))
			})

			It("should forbid not setting at least one of secretBindingName or credentialsBindingName", func() {
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.CredentialsBindingName = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.secretBindingName"),
					"Detail": Equal("must be set when credentialsBindingName is not"),
				}))))
			})

			It("should forbid updating secretBindingName when not migrating to credentialsBindingName", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Spec.SecretBindingName = ptr.To("another-reference")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.secretBindingName"),
					})),
				))
			})

			It("should allow updating credentialsBindingName", func() {
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.CredentialsBindingName = ptr.To("foo")
				newShoot := prepareShootForUpdate(shoot)
				shoot.Spec.CredentialsBindingName = ptr.To("another-reference")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow switching from secretBindingName to credentialsBindingName", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SecretBindingName = nil
				newShoot.Spec.CredentialsBindingName = ptr.To("another-reference")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should not allow switching from credentialsBindingName to secretBindingName", func() {
				shoot.Spec.SecretBindingName = nil
				shoot.Spec.CredentialsBindingName = ptr.To("another-reference")
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SecretBindingName = ptr.To("foo")
				newShoot.Spec.CredentialsBindingName = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.secretBindingName"),
						"Detail": Equal("field is immutable"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.credentialsBindingName"),
						"Detail": Equal("the field cannot be unset"),
					})),
				))
			})
		})

		It("should forbid adding invalid/duplicate emails", func() {
			shoot.Spec.Monitoring.Alerting.EmailReceivers = []string{
				"z",
				"foo@bar.baz",
				"foo@bar.baz",
			}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.monitoring.alerting.emailReceivers[0]"),
					"Detail": Equal("must provide a valid email"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.monitoring.alerting.emailReceivers[2]"),
				})),
			))
		})

		It("should forbid invalid tolerations", func() {
			shoot.Spec.Tolerations = []core.Toleration{
				{},
				{Key: "foo"},
				{Key: "foo"},
				{Key: "bar", Value: ptr.To("baz")},
				{Key: "bar", Value: ptr.To("baz")},
			}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.tolerations[0].key"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations[2]"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.tolerations[4]"),
				})),
			))
		})

		It("should forbid updating some cloud keys", func() {
			newShoot := prepareShootForUpdate(shoot)
			shoot.Spec.CloudProfileName = ptr.To("another-profile")
			shoot.Spec.Region = "another-region"
			// shoot.Spec.SecretBindingName = ptr.To("another-reference")
			// shoot.Spec.CredentialsBindingName = ptr.To("another-reference")
			shoot.Spec.Provider.Type = "another-provider"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.type"),
				})),
			))
		})

		Context("seed selector", func() {
			seedSelector := &core.SeedSelector{LabelSelector: metav1.LabelSelector{MatchLabels: map[string]string{"foo": "bar"}}}

			When("seed name is not set", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = nil
				})

				It("should allow setting the seed selector", func() {
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.SeedSelector = seedSelector

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should allow changing the seed selector", func() {
					shoot.Spec.SeedSelector = seedSelector
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.SeedSelector.MatchLabels["foo"] = "baz"

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should allow removing the seed selector", func() {
					shoot.Spec.SeedSelector = seedSelector
					newShoot := prepareShootForUpdate(shoot)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})
			})

			When("seed name is set", func() {
				BeforeEach(func() {
					shoot.Spec.SeedName = ptr.To("some-seed")
				})

				It("should not allow setting the seed selector", func() {
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.SeedSelector = seedSelector

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.seedSelector"),
						"Detail": Equal("cannot set seed selector when .spec.seedName is set"),
					}))))
				})

				It("should not allow changing the seed selector", func() {
					shoot.Spec.SeedSelector = seedSelector
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.SeedSelector.MatchLabels["foo"] = "baz"

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.seedSelector"),
						"Detail": Equal("field is immutable"),
					}))))
				})

				It("should allow removing the seed selector", func() {
					shoot.Spec.SeedSelector = seedSelector
					newShoot := prepareShootForUpdate(shoot)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})
			})
		})

		Context("Extensions validation", func() {
			It("should forbid passing an extension w/o type information", func() {
				extension := core.Extension{}
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.extensions[0].type"),
					}))))
			})

			It("should allow passing an extension w/ type information", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid passing an extension of same type more than once", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension, extension)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.extensions[1].type"),
					}))))
			})

			It("should allow passing more than one extension of different type", func() {
				extension := core.Extension{
					Type: "arbitrary",
				}
				shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension, extension)
				shoot.Spec.Extensions[1].Type = "arbitrary-2"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Resources validation", func() {
			It("should forbid resources w/o names or w/ invalid references", func() {
				ref := core.NamedResourceReference{}
				shoot.Spec.Resources = append(shoot.Spec.Resources, ref)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.kind"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.name"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.resources[0].resourceRef.apiVersion"),
					})),
				))
			})

			It("should forbid resources of kind other than Secret/ConfigMap", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "ServiceAccount",
						Name:       "test-sa",
						APIVersion: "v1",
					},
				}
				shoot.Spec.Resources = append(shoot.Spec.Resources, ref)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeNotSupported),
						"Field":    Equal("spec.resources[0].resourceRef.kind"),
						"BadValue": Equal("ServiceAccount"),
					})),
				))
			})

			It("should forbid resources with non-unique names", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret",
						APIVersion: "v1",
					},
				}
				shoot.Spec.Resources = append(shoot.Spec.Resources, ref, ref)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.resources[1].name"),
					})),
				))
			})

			It("should allow resources w/ names and valid references", func() {
				ref := core.NamedResourceReference{
					Name: "test",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "Secret",
						Name:       "test-secret",
						APIVersion: "v1",
					},
				}

				ref2 := core.NamedResourceReference{
					Name: "test-cm",
					ResourceRef: autoscalingv1.CrossVersionObjectReference{
						Kind:       "ConfigMap",
						Name:       "test-cm",
						APIVersion: "v1",
					},
				}

				shoot.Spec.Resources = append(shoot.Spec.Resources, ref, ref2)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})
		})

		It("should allow updating the seed if it has not been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.SeedName = ptr.To("another-seed")
			shoot.Spec.SeedName = nil

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(BeEmpty())
		})

		Context("Provider validation", func() {
			BeforeEach(func() {
				provider := core.Provider{
					Type:    "foo",
					Workers: []core.Worker{worker},
				}

				shoot.Spec.Provider = provider
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Provider.Workers = []core.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("spec.provider.workers[1].name"),
				}))))
			})

			It("should forbid invalid worker configuration", func() {
				w := invalidWorker.DeepCopy()
				shoot.Spec.Provider.Workers = []core.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].name"),
						"Detail": ContainSubstring("a lowercase RFC 1123 label must consist of lower case alphanumeric characters or"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.provider.workers[0].machine.type"),
						"Detail": ContainSubstring("must specify a machine type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].minimum"),
						"Detail": ContainSubstring("minimum value must not be negative"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].maximum"),
						"Detail": ContainSubstring("maximum value must not be negative"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.provider.workers[0].maximum"),
						"Detail": ContainSubstring("maximum value must not be less than minimum value"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.provider.workers[0].maximum"),
						"Detail": ContainSubstring("maximum node count should be greater than or equal to the number of zones specified for this pool"),
					})),
				))
			})

			It("should allow workers min = 0 if max > 0", func() {
				shoot.Spec.Provider.Workers = []core.Worker{workerAutoScalingMinZero, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Provider.Workers = []core.Worker{worker, workerAutoScalingMinMaxZero}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow workers having min=0", func() {
				shoot.Spec.Provider.Workers[0].Minimum = 0
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Provider.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal("spec.provider.workers[0].name"),
				}))))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Provider.Workers = []core.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.workers[0].name"),
				}))))
			})

			It("should not allow adding a worker pool to a workerless shoot", func() {
				shoot.Spec.Provider.Workers = []core.Worker{}
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers"),
					"Detail": ContainSubstring("cannot switch from a workerless Shoot to a Shoot with workers"),
				}))))
			})

			It("should not allow switching from a Shoot with workers to a workerless Shoot", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers = []core.Worker{}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers"),
					"Detail": ContainSubstring("cannot switch from a Shoot with workers to a workerless Shoot"),
				}))))
			})

			It("should allow adding a worker pool", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow removing a worker pool", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow swapping worker pools", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

				newShoot.Spec.Provider.Workers = []core.Worker{newShoot.Spec.Provider.Workers[1], newShoot.Spec.Provider.Workers[0]}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should prevent setting InfrastructureConfig for workerless Shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Addons = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil

				shoot.Spec.Provider.InfrastructureConfig = &runtime.RawExtension{
					Raw: []byte("foo"),
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.infrastructureConfig"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))
			})

			It("should prevent setting ControlPlaneConfig for workerless Shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Addons = nil
				shoot.Spec.Kubernetes.KubeControllerManager = nil

				shoot.Spec.Provider.ControlPlaneConfig = &runtime.RawExtension{
					Raw: []byte("foo"),
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.controlPlaneConfig"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))
			})

			Describe("WorkersSettings validation", func() {
				It("should not allow setting it for workerless Shoots", func() {
					shoot.Spec.Provider.Workers = []core.Worker{}
					shoot.Spec.Provider.WorkersSettings = &core.WorkersSettings{}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.provider.workersSettings"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))))
				})

				It("should be able to set it for Shoots with worker", func() {
					shoot.Spec.Provider.WorkersSettings = &core.WorkersSettings{}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(BeEmpty())
				})

				It("should prevent setting 'ControlPlane' field for worker pool", func() {
					shoot.Spec.Provider.Workers[0].ControlPlane = &core.WorkerControlPlane{}

					Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.provider.workers[0].controlPlane"),
					}))))
				})
			})

			Describe("ClusterAutoscaler options validation", func() {
				var (
					negativeDuration = metav1.Duration{Duration: -time.Second}
					positiveDuration = metav1.Duration{Duration: time.Second}
				)

				DescribeTable("cluster autoscaler values",
					func(caOptions core.ClusterAutoscalerOptions, matcher gomegatypes.GomegaMatcher) {
						Expect(ValidateClusterAutoscalerOptions(&caOptions, nil)).To(matcher)
					},
					Entry("valid with empty options", core.ClusterAutoscalerOptions{}, BeEmpty()),
					Entry("valid with nil options", nil, BeEmpty()),
					Entry("valid with all options", core.ClusterAutoscalerOptions{
						ScaleDownUtilizationThreshold:    ptr.To(0.5),
						ScaleDownGpuUtilizationThreshold: ptr.To(0.5),
						ScaleDownUnneededTime:            ptr.To(positiveDuration),
						ScaleDownUnreadyTime:             ptr.To(positiveDuration),
						MaxNodeProvisionTime:             ptr.To(positiveDuration),
					}, BeEmpty()),
					Entry("valid with ScaleDownUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownUtilizationThreshold: ptr.To(0.5),
					}, BeEmpty()),
					Entry("invalid negative ScaleDownUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownUtilizationThreshold: ptr.To(-0.5),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
					Entry("invalid > 1 ScaleDownUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownUtilizationThreshold: ptr.To(1.5),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
					Entry("valid with ScaleDownGpuUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownGpuUtilizationThreshold: ptr.To(0.5),
					}, BeEmpty()),
					Entry("invalid negative ScaleDownGpuUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownGpuUtilizationThreshold: ptr.To(-0.5),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownGpuUtilizationThreshold"), -0.5, "can not be negative"))),
					Entry("invalid > 1 ScaleDownGpuUtilizationThreshold", core.ClusterAutoscalerOptions{
						ScaleDownGpuUtilizationThreshold: ptr.To(1.5),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownGpuUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
					Entry("valid with ScaleDownUnneededTime", core.ClusterAutoscalerOptions{
						ScaleDownUnneededTime: ptr.To(metav1.Duration{Duration: time.Minute}),
					}, BeEmpty()),
					Entry("invalid negative ScaleDownUnneededTime", core.ClusterAutoscalerOptions{
						ScaleDownUnneededTime: ptr.To(negativeDuration),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownUnneededTime"), negativeDuration, "can not be negative"))),
					Entry("valid with ScaleDownUnreadyTime", core.ClusterAutoscalerOptions{
						ScaleDownUnreadyTime: ptr.To(metav1.Duration{Duration: time.Minute}),
					}, BeEmpty()),
					Entry("invalid negative ScaleDownUnreadyTime", core.ClusterAutoscalerOptions{
						ScaleDownUnreadyTime: ptr.To(negativeDuration),
					}, ConsistOf(field.Invalid(field.NewPath("scaleDownUnreadyTime"), negativeDuration, "can not be negative"))),
					Entry("valid with MaxNodeProvisionTime", core.ClusterAutoscalerOptions{
						MaxNodeProvisionTime: ptr.To(metav1.Duration{Duration: time.Minute}),
					}, BeEmpty()),
					Entry("invalid negative MaxNodeProvisionTime", core.ClusterAutoscalerOptions{
						MaxNodeProvisionTime: ptr.To(negativeDuration),
					}, ConsistOf(field.Invalid(field.NewPath("maxNodeProvisionTime"), negativeDuration, "can not be negative"))),
				)
			})
		})

		Context("DNS section", func() {
			It("should forbid specifying a provider without a domain", func() {
				shoot.Spec.DNS.Domain = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should allow specifying the 'unmanaged' provider without a domain", func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:    ptr.To(core.DNSUnmanaged),
						Primary: ptr.To(true),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid specifying invalid domain", func() {
				shoot.Spec.DNS.Providers = nil
				shoot.Spec.DNS.Domain = ptr.To("foo/bar.baz")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid specifying a secret name when provider equals 'unmanaged'", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       ptr.To(core.DNSUnmanaged),
						SecretName: ptr.To(""),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.providers[0].secretName"),
				}))))
			})

			It("should require a provider if a secret name is given", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						SecretName: ptr.To(""),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.dns.providers[0].type"),
				}))))
			})

			It("should allow assigning the dns domain (dns nil)", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.DNS = nil
				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.DNS = &core.DNS{
					Domain: ptr.To("some-domain.com"),
				}

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow assigning the dns domain (dns non-nil)", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.DNS = &core.DNS{}
				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.DNS.Domain = ptr.To("some-domain.com")

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid removing the dns section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns"),
				}))))
			})

			It("should forbid updating the dns domain", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Domain = ptr.To("another-domain.com")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid updating the dns providers", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.DNS.Providers[0].Type = ptr.To("some-dns-provider")

				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.SeedName = ptr.To("seed")
				newShoot.Spec.DNS.Providers = nil

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid to unset the primary DNS provider type", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SeedName = ptr.To("seed")
				newShoot.Spec.DNS.Providers[0].Type = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid to remove the primary DNS provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SeedName = ptr.To("seed")
				newShoot.Spec.DNS.Providers[0].Primary = ptr.To(false)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid adding another primary provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SeedName = ptr.To("seed")
				newShoot.Spec.DNS.Providers = append(newShoot.Spec.DNS.Providers, core.DNSProvider{
					Primary: ptr.To(true),
				})

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers[1].primary"),
				}))))
			})

			It("should forbid having a provider with invalid secretName", func() {
				invalidSecretName := "foo/bar"

				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						SecretName: &secretName,
						Type:       &dnsProviderType,
					},
					{
						SecretName: &invalidSecretName,
						Type:       &dnsProviderType,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.providers[1]"),
				}))))
			})

			It("should forbid having the same provider multiple times", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						SecretName: &secretName,
						Type:       &dnsProviderType,
					},
					{
						SecretName: &secretName,
						Type:       &dnsProviderType,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.providers[1]"),
				}))))
			})

			It("should allow updating the dns secret name", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Providers[0].SecretName = ptr.To("my-dns-secret")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid having more than one primary provider", func() {
				shoot.Spec.DNS.Providers = append(shoot.Spec.DNS.Providers, core.DNSProvider{
					Primary: ptr.To(true),
				})

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers[1].primary"),
				}))))
			})
		})

		Context("ETCD validation", func() {
			Context("Autoscaling validation", func() {
				It("should succeed defining minAllowed values", func() {
					shoot.Spec.Kubernetes.ETCD = &core.ETCD{
						Main: &core.ETCDConfig{
							Autoscaling: &core.ControlPlaneAutoscaling{
								MinAllowed: corev1.ResourceList{
									"memory": resource.MustParse("300M"),
								},
							},
						},
						Events: &core.ETCDConfig{
							Autoscaling: &core.ControlPlaneAutoscaling{
								MinAllowed: corev1.ResourceList{
									"memory": resource.MustParse("60M"),
								},
							},
						},
					}

					Expect(ValidateShoot(shoot)).To(BeEmpty())
				})

				It("should not allow minAllowed values below minimum", func() {
					shoot.Spec.Kubernetes.ETCD = &core.ETCD{
						Main: &core.ETCDConfig{
							Autoscaling: &core.ControlPlaneAutoscaling{
								MinAllowed: corev1.ResourceList{
									"memory": resource.MustParse("299M"),
								},
							},
						},
						Events: &core.ETCDConfig{
							Autoscaling: &core.ControlPlaneAutoscaling{
								MinAllowed: corev1.ResourceList{
									"memory": resource.MustParse("59M"),
								},
							},
						},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.kubernetes.etcd.main.autoscaling.minAllowed.memory"),
							"BadValue": Equal(resource.MustParse("299M")),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":     Equal(field.ErrorTypeInvalid),
							"Field":    Equal("spec.kubernetes.etcd.events.autoscaling.minAllowed.memory"),
							"BadValue": Equal(resource.MustParse("59M")),
						})),
					))
				})
			})
		})

		Context("KubeAPIServer validation", func() {
			Context("OIDC validation", func() {
				It("should forbid setting OIDC configuration from kubernetes version 1.32", func() {
					shoot.Spec.Kubernetes.Version = "1.32"

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig"),
					}))))
				})

				It("should forbid unsupported OIDC configuration", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.CABundle = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsClaim = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsPrefix = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernameClaim = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernamePrefix = ptr.To("")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.SigningAlgs = []string{"foo"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.caBundle"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsClaim"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsPrefix"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("spec.kubernetes.kubeAPIServer.oidcConfig.signingAlgs[0]"),
						"Detail": Equal("supported values: \"ES256\", \"ES384\", \"ES512\", \"PS256\", \"PS384\", \"PS512\", \"RS256\", \"RS384\", \"RS512\", \"none\""),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernameClaim"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernamePrefix"),
					}))))
				})

				DescribeTable("should forbid issuerURL to be empty string or nil, if clientID exists ", func(errorListSize int, issuerURL *string) {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("someClientID")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = issuerURL

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(errorListSize))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
					}))
				},
					Entry("should add error if clientID is set but issuerURL is nil", 1, nil),
					Entry("should add error if clientID is set but issuerURL is empty string", 2, ptr.To("")),
				)

				It("should not fail if both clientID and issuerURL are set", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = ptr.To("https://issuer.com")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = ptr.To("someClientID")

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(BeEmpty())
				})

				DescribeTable("should forbid clientID to be empty string or nil, if issuerURL exists ", func(errorListSize int, clientID *string) {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = ptr.To("https://issuer.com")
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = clientID

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(errorListSize))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
					}))
				},
					Entry("should add error if issuerURL is set but clientID is nil", 1, nil),
					Entry("should add error if issuerURL is set but clientID is empty string ", 2, ptr.To("")),
				)

				It("should forbid setting clientAuthentication from kubernetes version 1.31", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientAuthentication = &core.OpenIDConnectClientAuthentication{}
					shoot.Spec.Kubernetes.Version = "1.31"

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientAuthentication"),
					}))
				})
			})

			Context("AdmissionPlugins validation", func() {
				It("should allow not specifying admission plugins", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = nil

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(BeEmpty())
				})

				It("should forbid specifying admission plugins without a name", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
						{
							Name: "",
						},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(HaveLen(1))
					Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.kubernetes.kubeAPIServer.admissionPlugins[0].name"),
					}))
				})

				It("should forbid specifying the SecurityContextDeny admission plugin", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
						{
							Name: "SecurityContextDeny",
						},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.kubeAPIServer.admissionPlugins[0].name"),
					}))))
				})

				It("should forbid disabling the required plugins", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []core.AdmissionPlugin{
						{
							Name:     "MutatingAdmissionWebhook",
							Disabled: ptr.To(true),
						},
						{
							Name:     "NamespaceLifecycle",
							Disabled: ptr.To(false),
						},
						{
							Name:     "NodeRestriction",
							Disabled: ptr.To(true),
						},
					}
					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeAPIServer.admissionPlugins[0]"),
						"Detail": Equal(fmt.Sprintf("admission plugin %q cannot be disabled", shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins[0].Name)),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeAPIServer.admissionPlugins[2]"),
						"Detail": Equal(fmt.Sprintf("admission plugin %q cannot be disabled", shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins[2].Name)),
					}))))
				})
			})

			Context("EncryptionConfig validation", func() {
				BeforeEach(func() {
					shoot.Spec.Kubernetes.Version = "1.28"
				})

				It("should allow specifying valid resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"configmaps", "nonexistingresource", "postgres.fancyoperator.io"},
					}

					Expect(ValidateShoot(shoot)).To(BeEmpty())
				})

				It("should deny specifying duplicated resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"configmaps", "configmaps", "services", "services."},
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[1]"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[3]"),
						})),
					))
				})

				It("should deny specifying duplicated resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"services.", "services."},
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeDuplicate),
							"Field": Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[1]"),
						})),
					))
				})

				It("should deny specifying wildcard resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"*.apps", "*.*"},
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[0]"),
							"Detail": Equal("wildcards are not supported"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[1]"),
							"Detail": Equal("wildcards are not supported"),
						})),
					))
				})

				It("should deny specifying 'secrets' resource in resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"configmaps", "secrets"},
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[1]"),
							"Detail": Equal("\"secrets\" are always encrypted"),
						})),
					))
				})

				It("should deny specifying 'secrets.' resource in resources", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"configmaps", "secrets."},
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources[1]"),
							"Detail": Equal("\"secrets.\" are always encrypted"),
						})),
					))
				})

				It("should deny changing items when resources in the spec and status are not equal", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: []string{"configmaps", "deployments.apps"},
					}

					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"configmaps", "new.fancyresource.io"}

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources"),
							"Detail": Equal("resources cannot be changed because a previous encryption configuration change is currently being rolled out"),
						})),
					))
				})

				It("should deny changing items when shoot is in hibernation", func() {
					resources := []string{"configmaps", "deployments.apps"}
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: resources,
					}
					shoot.Status.EncryptedResources = resources
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"configmaps", "new.fancyresource.io"}

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources"),
							"Detail": Equal("resources cannot be changed when shoot is in hibernation"),
						})),
					))
				})

				It("should allow changing items when shoot is waking up", func() {
					resources := []string{"configmaps", "deployments.apps"}
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: resources,
					}
					shoot.Status.EncryptedResources = resources
					shoot.Status.IsHibernated = true

					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
					newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"configmaps", "new.fancyresource.io"}

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should deny changing items during ETCD Encryption Key rotation", func() {
					resources := []string{"configmaps", "deployments.apps"}
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: resources,
					}
					shoot.Status.EncryptedResources = resources

					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"configmaps", "new.fancyresource.io"}
					newShoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					}

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("spec.kubernetes.kubeAPIServer.encryptionConfig.resources"),
							"Detail": Equal("resources cannot be changed when .status.credentials.rotation.etcdEncryptionKey.phase is not \"Completed\""),
						})),
					))
				})

				It("should allow changing items if ETCD Encryption Key rotation is in phase Completed or was never rotated", func() {
					resources := []string{"configmaps", "deployments.apps"}
					shoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig = &core.EncryptionConfig{
						Resources: resources,
					}
					shoot.Status.EncryptedResources = resources

					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Kubernetes.KubeAPIServer.EncryptionConfig.Resources = []string{"deployments.apps", "newresource.fancyresource.io"}
					newShoot.Status.Credentials = nil

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())

					newShoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					}

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})
			})

			Context("WatchCacheSizes validation", func() {
				var negativeSize int32 = -1

				DescribeTable("watch cache size validation",
					func(sizes *core.WatchCacheSizes, matcher gomegatypes.GomegaMatcher) {
						Expect(ValidateWatchCacheSizes(sizes, nil)).To(matcher)
					},

					Entry("valid (unset)", nil, BeEmpty()),
					Entry("valid (fields unset)", &core.WatchCacheSizes{}, BeEmpty()),
					Entry("valid (default=0)", &core.WatchCacheSizes{
						Default: ptr.To[int32](0),
					}, BeEmpty()),
					Entry("valid (default>0)", &core.WatchCacheSizes{
						Default: ptr.To[int32](42),
					}, BeEmpty()),
					Entry("invalid (default<0)", &core.WatchCacheSizes{
						Default: ptr.To(negativeSize),
					}, ConsistOf(
						field.Invalid(field.NewPath("default"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
					)),

					// APIGroup unset (core group)
					Entry("valid (core/secrets=0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							Resource:  "secrets",
							CacheSize: 0,
						}},
					}, BeEmpty()),
					Entry("valid (core/secrets=>0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							Resource:  "secrets",
							CacheSize: 42,
						}},
					}, BeEmpty()),
					Entry("invalid (core/secrets=<0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							Resource:  "secrets",
							CacheSize: negativeSize,
						}},
					}, ConsistOf(
						field.Invalid(field.NewPath("resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
					)),
					Entry("invalid (core/resource empty)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							Resource:  "",
							CacheSize: 42,
						}},
					}, ConsistOf(
						field.Required(field.NewPath("resources[0].resource"), "must not be empty"),
					)),

					// APIGroup set
					Entry("valid (apps/deployments=0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							APIGroup:  ptr.To("apps"),
							Resource:  "deployments",
							CacheSize: 0,
						}},
					}, BeEmpty()),
					Entry("valid (apps/deployments=>0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							APIGroup:  ptr.To("apps"),
							Resource:  "deployments",
							CacheSize: 42,
						}},
					}, BeEmpty()),
					Entry("invalid (apps/deployments=<0)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							APIGroup:  ptr.To("apps"),
							Resource:  "deployments",
							CacheSize: negativeSize,
						}},
					}, ConsistOf(
						field.Invalid(field.NewPath("resources[0].size"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
					)),
					Entry("invalid (apps/resource empty)", &core.WatchCacheSizes{
						Resources: []core.ResourceWatchCacheSize{{
							Resource:  "",
							CacheSize: 42,
						}},
					}, ConsistOf(
						field.Required(field.NewPath("resources[0].resource"), "must not be empty"),
					)),
				)
			})

			Context("APIServerLogging validation", func() {
				var negativeSize int32 = -1

				DescribeTable("APIServerLogging validation",
					func(loggingConfig *core.APIServerLogging, matcher gomegatypes.GomegaMatcher) {
						Expect(ValidateAPIServerLogging(loggingConfig, nil)).To(matcher)
					},

					Entry("valid (unset)", nil, BeEmpty()),
					Entry("valid (fields unset)", &core.APIServerLogging{}, BeEmpty()),
					Entry("valid (verbosity=0)", &core.APIServerLogging{
						Verbosity: ptr.To[int32](0),
					}, BeEmpty()),
					Entry("valid (httpAccessVerbosity=0)", &core.APIServerLogging{
						HTTPAccessVerbosity: ptr.To[int32](0),
					}, BeEmpty()),
					Entry("valid (verbosity>0)", &core.APIServerLogging{
						Verbosity: ptr.To[int32](3),
					}, BeEmpty()),
					Entry("valid (httpAccessVerbosity>0)", &core.APIServerLogging{
						HTTPAccessVerbosity: ptr.To[int32](3),
					}, BeEmpty()),
					Entry("invalid (verbosity<0)", &core.APIServerLogging{
						Verbosity: ptr.To(negativeSize),
					}, ConsistOf(
						field.Invalid(field.NewPath("verbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
					)),
					Entry("invalid (httpAccessVerbosity<0)", &core.APIServerLogging{
						HTTPAccessVerbosity: ptr.To(negativeSize),
					}, ConsistOf(
						field.Invalid(field.NewPath("httpAccessVerbosity"), int64(negativeSize), apivalidation.IsNegativeErrorMsg),
					)),
				)
			})

			Context("Requests validation", func() {
				It("should not allow too high values for max inflight requests fields", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.Requests = &core.APIServerRequests{
						MaxNonMutatingInflight: ptr.To[int32](123123123),
						MaxMutatingInflight:    ptr.To[int32](412412412),
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.requests.maxNonMutatingInflight"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.requests.maxMutatingInflight"),
					}))))
				})

				It("should not allow negative values for max inflight requests fields", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.Requests = &core.APIServerRequests{
						MaxNonMutatingInflight: ptr.To(int32(-1)),
						MaxMutatingInflight:    ptr.To(int32(-1)),
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.requests.maxNonMutatingInflight"),
					})), PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.requests.maxMutatingInflight"),
					}))))
				})
			})

			Context("ServiceAccountConfig validation", func() {
				It("should not allow to specify a negative max token duration", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = &core.ServiceAccountConfig{
						MaxTokenExpiration: &metav1.Duration{Duration: -1},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration"),
					}))))
				})

				It("should forbid too low values for the max token duration", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = &core.ServiceAccountConfig{
						MaxTokenExpiration: &metav1.Duration{Duration: time.Hour},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration"),
					}))))
				})

				It("should forbid too high values for the max token duration", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = &core.ServiceAccountConfig{
						MaxTokenExpiration: &metav1.Duration{Duration: 3000 * time.Hour},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.kubernetes.kubeAPIServer.serviceAccountConfig.maxTokenExpiration"),
					}))))
				})

				It("should not allow to specify duplicates in accepted issuers", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = &core.ServiceAccountConfig{
						AcceptedIssuers: []string{
							"foo",
							"foo",
						},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeDuplicate),
						"Field": Equal("spec.kubernetes.kubeAPIServer.serviceAccountConfig.acceptedIssuers[1]"),
					}))))
				})

				It("should not allow to duplicate the issuer in accepted issuers", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.ServiceAccountConfig = &core.ServiceAccountConfig{
						Issuer:          ptr.To("foo"),
						AcceptedIssuers: []string{"foo"},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.kubeAPIServer.serviceAccountConfig.acceptedIssuers[0]"),
						"Detail": ContainSubstring("acceptedIssuers cannot contains the issuer field value: foo"),
					}))))
				})
			})

			Context("Autoscaling validation", func() {
				It("should succeed defining minAllowed values", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.Autoscaling = &core.ControlPlaneAutoscaling{
						MinAllowed: corev1.ResourceList{
							"cpu":    resource.MustParse("20m"),
							"memory": resource.MustParse("200M"),
						},
					}

					Expect(ValidateShoot(shoot)).To(BeEmpty())
				})

				It("should not allow minAllowed values below minimum", func() {
					shoot.Spec.Kubernetes.KubeAPIServer.Autoscaling = &core.ControlPlaneAutoscaling{
						MinAllowed: corev1.ResourceList{
							"cpu": resource.MustParse("19m"),
						},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("spec.kubernetes.kubeAPIServer.autoscaling.minAllowed.cpu"),
						"BadValue": Equal(resource.MustParse("19m")),
					}))))
				})
			})

			It("should not allow to specify a negative event ttl duration", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.EventTTL = &metav1.Duration{Duration: -1}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.eventTTL"),
				}))))
			})

			It("should not allow to specify an event ttl duration longer than 7d", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.EventTTL = &metav1.Duration{Duration: time.Hour * 24 * 8}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.eventTTL"),
				}))))
			})

			It("should not allow to specify a negative defaultNotReadyTolerationSeconds", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds = ptr.To(int64(-1))

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.defaultNotReadyTolerationSeconds"),
				}))))
			})

			It("should allow to specify a valid defaultNotReadyTolerationSeconds", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.DefaultNotReadyTolerationSeconds = ptr.To[int64](120)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should not allow to specify a negative defaultUnreachableTolerationSeconds", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds = ptr.To(int64(-1))

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.defaultUnreachableTolerationSeconds"),
				}))))
			})

			It("should allow to specify a valid defaultUnreachableTolerationSeconds", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.DefaultUnreachableTolerationSeconds = ptr.To[int64](120)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("kubernetes.enableStaticTokenKubeconfig field validation", func() {
			It("should deny creating shoots with this field set to true", func() {
				shoot.Spec.Kubernetes.EnableStaticTokenKubeconfig = ptr.To(true)

				errorList := ValidateShoot(shoot)
				Expect(errorList).Should(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":     Equal(field.ErrorTypeInvalid),
					"Field":    Equal("spec.kubernetes.enableStaticTokenKubeconfig"),
					"BadValue": Equal(true),
					"Detail":   ContainSubstring("setting this field to true is not supported"),
				}))))
			})
		})

		Context("KubeControllerManager validation", func() {
			Context("for workerless shoots", func() {
				BeforeEach(func() {
					shoot.Spec.Provider.Workers = []core.Worker{}
				})

				It("should prevent setting nodeCIDRMaskSize", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](23)

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))))
				})

				It("should prevent setting horizontalPodAutoscaler", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig = &core.HorizontalPodAutoscalerConfig{
						CPUInitializationPeriod: &metav1.Duration{Duration: 5 * time.Minute},
					}

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))))
				})

				It("should prevent setting podEvictionTimeout", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout = &metav1.Duration{Duration: 5 * time.Minute}

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.podEvictionTimeout"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))))
				})

				It("should prevent setting nodeMonitorGracePeriod", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: 5 * time.Minute}

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeMonitorGracePeriod"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))))
				})
			})

			It("should forbid unsupported HPA configuration", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = &metav1.Duration{Duration: -1 * time.Second}
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = &metav1.Duration{Duration: -1 * time.Second}
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = &metav1.Duration{Duration: -1 * time.Second}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleStabilization"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.initialReadinessDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.cpuInitializationPeriod"),
				}))))
			})

			It("should succeed when using valid configuration parameters", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = &metav1.Duration{Duration: 5 * time.Minute}
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = &metav1.Duration{Duration: 30 * time.Second}
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = &metav1.Duration{Duration: 5 * time.Minute}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail updating immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](22)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})

			It("should succeed not changing immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](24)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			Describe("nodeCIDRMaskSize validation", func() {
				It("should fail when nodeCIDRMaskSize is out of upper boundary", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](32)

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
						"Detail": ContainSubstring("nodeCIDRMaskSize must be between 16 and 28"),
					}))))
				})

				It("should fail when nodeCIDRMaskSize is out of lower boundary", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](0)

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
						"Detail": ContainSubstring("nodeCIDRMaskSize must be between 16 and 28"),
					}))))
				})

				It("should succeed when nodeCIDRMaskSize is within boundaries", func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To[int32](22)

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(BeEmpty())
				})

				Context("cross validation with maxPods", func() {
					var (
						defaultNodeCIDRMaskSize  int32
						tooLargeNodeCIDRMaskSize int32
					)

					BeforeEach(func() {
						shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{MaxPods: ptr.To[int32](110)}

						firstWorker := shoot.Spec.Provider.Workers[0].DeepCopy()
						firstWorker.Kubernetes = &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								MaxPods: ptr.To[int32](110),
							},
						}

						secondWorker := firstWorker.DeepCopy()
						secondWorker.Name += "2"
						secondWorker.Kubernetes.Kubelet.MaxPods = ptr.To[int32](220)
						shoot.Spec.Provider.Workers = []core.Worker{*firstWorker, *secondWorker}
					})

					Context("IPv4", func() {
						BeforeEach(func() {
							// /24 CIDR can host 254 pod IPs (prefix is small enough for the largest maxPods setting)
							defaultNodeCIDRMaskSize = 24
							// /25 CIDR can host 126 pod IPs (prefix is too large for the largest maxPods setting)
							tooLargeNodeCIDRMaskSize = 25
							shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To(defaultNodeCIDRMaskSize)
						})

						It("should allow the default maxPods and nodeCIDRMaskSize", func() {
							Expect(ValidateShoot(shoot)).To(BeEmpty())
						})

						It("should deny too large nodeCIDRMaskSize", func() {
							shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To(tooLargeNodeCIDRMaskSize)

							Expect(ValidateShoot(shoot)).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring("only supports 126 IP addresses"),
							}))
						})
					})

					Context("IPv6", func() {
						BeforeEach(func() {
							shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}

							// /64 CIDR can host a lot of pod IPs (prefix is small enough for the largest maxPods setting)
							defaultNodeCIDRMaskSize = 64
							// /121 CIDR can host 126 pod IPs (prefix is too large for the largest maxPods setting)
							tooLargeNodeCIDRMaskSize = 121
							shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To(defaultNodeCIDRMaskSize)
						})

						It("should allow the default maxPods and nodeCIDRMaskSize", func() {
							Expect(ValidateShoot(shoot)).To(BeEmpty())
						})

						It("should deny too large nodeCIDRMaskSize", func() {
							shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = ptr.To(tooLargeNodeCIDRMaskSize)

							Expect(ValidateShoot(shoot)).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring("only supports 126 IP addresses"),
							}))
						})
					})
				})
			})

			It("should prevent setting a negative pod eviction timeout", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout = &metav1.Duration{Duration: -1}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.podEvictionTimeout"),
				}))))
			})

			It("should prevent setting the pod eviction timeout to 0", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.PodEvictionTimeout = &metav1.Duration{}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.podEvictionTimeout"),
				}))))
			})

			It("should prevent setting a negative node monitor grace period", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{Duration: -1}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeMonitorGracePeriod"),
				}))))
			})

			It("should prevent setting the node monitor grace period to 0", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeMonitorGracePeriod = &metav1.Duration{}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeMonitorGracePeriod"),
				}))))
			})
		})

		Context("KubeScheduler validation", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeScheduler = &core.KubeSchedulerConfig{}
			})

			It("should prevent setting kubescheduler config for workerless shoots", func() {
				profile := core.SchedulingProfileBinPacking
				shoot.Spec.Provider.Workers = []core.Worker{}
				shoot.Spec.Kubernetes.KubeScheduler = &core.KubeSchedulerConfig{
					Profile: &profile,
				}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeScheduler"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))

			})

			It("should succeed when using valid scheduling profile", func() {
				profile := core.SchedulingProfileBalanced
				shoot.Spec.Kubernetes.KubeScheduler.Profile = &profile

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should succeed when using nil scheduling profile", func() {
				shoot.Spec.Kubernetes.KubeScheduler.Profile = nil

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail when using unknown scheduling profile", func() {
				profile := core.SchedulingProfile("foo")
				shoot.Spec.Kubernetes.KubeScheduler.Profile = &profile

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.kubernetes.kubeScheduler.profile"),
				}))))
			})
		})

		Context("KubeProxy validation", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeProxy = &core.KubeProxyConfig{}
			})

			It("should prevent setting kubeproxy config for workerless shoots", func() {
				shoot.Spec.Provider.Workers = []core.Worker{}
				shoot.Spec.Kubernetes.KubeProxy = &core.KubeProxyConfig{
					KubernetesConfig: core.KubernetesConfig{
						FeatureGates: nil,
					},
				}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeProxy"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				}))))
			})

			It("should succeed when using IPTables mode", func() {
				mode := core.ProxyModeIPTables
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should succeed when using IPVS mode", func() {
				mode := core.ProxyModeIPVS
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should fail when using nil proxy mode", func() {
				shoot.Spec.Kubernetes.KubeProxy.Mode = nil
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.kubeProxy.mode"),
				}))))
			})

			It("should fail when using unknown proxy mode", func() {
				m := core.ProxyMode("fooMode")
				shoot.Spec.Kubernetes.KubeProxy.Mode = &m
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.kubernetes.kubeProxy.mode"),
				}))))
			})

			It("should be successful when proxy mode is changed", func() {
				mode := core.ProxyMode("IPVS")
				kubernetesConfig := core.KubernetesConfig{}
				config := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &mode,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				shoot.Spec.Kubernetes.Version = "1.28.1"
				oldMode := core.ProxyMode("IPTables")
				oldConfig := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &oldMode,
				}
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, metav1.ObjectMeta{}, field.NewPath("spec"))
				Expect(errorList).To(BeEmpty())
			})

			It("should not fail when kube-proxy is switched off", func() {
				kubernetesConfig := core.KubernetesConfig{}
				disabled := false
				config := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Enabled:          &disabled,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				enabled := true
				oldConfig := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Enabled:          &enabled,
				}
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, metav1.ObjectMeta{}, field.NewPath("spec"))

				Expect(errorList).To(BeEmpty())
			})
		})

		var (
			negativeDuration                    = metav1.Duration{Duration: -time.Second}
			negativeInteger               int32 = -100
			positiveInteger               int32 = 100
			expanderLeastWaste                  = core.ClusterAutoscalerExpanderLeastWaste
			expanderMostPods                    = core.ClusterAutoscalerExpanderMostPods
			expanderPriority                    = core.ClusterAutoscalerExpanderPriority
			expanderRandom                      = core.ClusterAutoscalerExpanderRandom
			expanderPriorityAndLeastWaste       = core.ClusterAutoscalerExpanderPriority + "," + core.ClusterAutoscalerExpanderLeastWaste
			invalidExpander                     = core.ClusterAutoscalerExpanderPriority + ", test-expander"
			invalidMultipleExpanderString       = core.ClusterAutoscalerExpanderPriority + ", " + core.ClusterAutoscalerExpanderLeastWaste
			taintsUnique                        = []string{"taint-1", "taint-2"}
			taintsDuplicate                     = []string{"taint-1", "taint-1"}
			taintsInvalid                       = []string{"taint 1", "taint-1"}
			version_1_28                        = "1.28.4"
			version_1_30                        = "1.30.1"
			version_1_32                        = "1.32.0"
		)

		Context("ClusterAutoscaler validation", func() {
			DescribeTable("cluster autoscaler values",
				func(clusterAutoscaler core.ClusterAutoscaler, version string, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateClusterAutoscaler(clusterAutoscaler, version, nil)).To(matcher)
				},
				Entry("valid", core.ClusterAutoscaler{}, version_1_28, BeEmpty()),
				Entry("valid", core.ClusterAutoscaler{}, version_1_30, BeEmpty()),
				Entry("valid with threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: ptr.To(0.5),
				}, version_1_28, BeEmpty()),
				Entry("invalid negative threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: ptr.To(-0.5),
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
				Entry("invalid > 1 threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: ptr.To(1.5),
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
				Entry("valid with maxNodeProvisionTime", core.ClusterAutoscaler{
					MaxNodeProvisionTime: &metav1.Duration{Duration: time.Minute},
				}, version_1_28, BeEmpty()),
				Entry("invalid with negative maxNodeProvisionTime", core.ClusterAutoscaler{
					MaxNodeProvisionTime: &negativeDuration,
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("maxNodeProvisionTime"), negativeDuration, "can not be negative"))),
				Entry("valid with maxGracefulTerminationSeconds", core.ClusterAutoscaler{
					MaxGracefulTerminationSeconds: &positiveInteger,
				}, version_1_28, BeEmpty()),
				Entry("invalid with negative maxGracefulTerminationSeconds", core.ClusterAutoscaler{
					MaxGracefulTerminationSeconds: &negativeInteger,
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("maxGracefulTerminationSeconds"), negativeInteger, "can not be negative"))),
				Entry("valid with expander least waste", core.ClusterAutoscaler{
					Expander: &expanderLeastWaste,
				}, version_1_28, BeEmpty()),
				Entry("valid with expander most pods", core.ClusterAutoscaler{
					Expander: &expanderMostPods,
				}, version_1_28, BeEmpty()),
				Entry("valid with expander priority", core.ClusterAutoscaler{
					Expander: &expanderPriority,
				}, version_1_28, BeEmpty()),
				Entry("valid with expander random", core.ClusterAutoscaler{
					Expander: &expanderRandom,
				}, version_1_28, BeEmpty()),
				Entry("invalid with startup taint on K8S v1.28", core.ClusterAutoscaler{
					StartupTaints: taintsUnique,
				}, version_1_28, ConsistOf(field.Forbidden(field.NewPath("startupTaints.StartupTaints"), "not supported in Kubernetes version 1.28.4"))),
				Entry("valid with startup taint", core.ClusterAutoscaler{
					StartupTaints: taintsUnique,
				}, version_1_30, BeEmpty()),
				Entry("duplicate startup taint", core.ClusterAutoscaler{
					StartupTaints: taintsDuplicate,
				}, version_1_30, ConsistOf(field.Duplicate(field.NewPath("startupTaints").Index(1), taintsDuplicate[1]))),
				Entry("invalid with startup taint",
					core.ClusterAutoscaler{
						StartupTaints: taintsInvalid,
					}, version_1_30,
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("startupTaints[0]"),
					}))),
				),
				Entry("invalid with startup taint on K8S v1.28", core.ClusterAutoscaler{
					StatusTaints: taintsUnique,
				}, version_1_28, ConsistOf(field.Forbidden(field.NewPath("statusTaints.StatusTaints"), "not supported in Kubernetes version 1.28.4"))),
				Entry("valid with status taint", core.ClusterAutoscaler{
					StatusTaints: taintsUnique,
				}, version_1_30, BeEmpty()),
				Entry("duplicate status taint", core.ClusterAutoscaler{
					StatusTaints: taintsDuplicate,
				}, version_1_30, ConsistOf(field.Duplicate(field.NewPath("statusTaints").Index(1), taintsDuplicate[1]))),
				Entry("invalid with status taint",
					core.ClusterAutoscaler{
						StatusTaints: taintsInvalid,
					}, version_1_30,
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("statusTaints[0]"),
					}))),
				),
				Entry("invalid with ignore taint on K8S v1.32", core.ClusterAutoscaler{
					IgnoreTaints: taintsUnique,
				}, version_1_32, ConsistOf(field.Forbidden(field.NewPath("ignoreTaints.IgnoreTaints"), "not supported in Kubernetes version 1.32.0"))),
				Entry("valid with ignore taint", core.ClusterAutoscaler{
					IgnoreTaints: taintsUnique,
				}, version_1_28, BeEmpty()),
				Entry("duplicate ignore taint", core.ClusterAutoscaler{
					IgnoreTaints: taintsDuplicate,
				}, version_1_28, ConsistOf(field.Duplicate(field.NewPath("ignoreTaints").Index(1), taintsDuplicate[1]))),
				Entry("invalid with ignore taint",
					core.ClusterAutoscaler{
						IgnoreTaints: taintsInvalid,
					}, version_1_28,
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("ignoreTaints[0]"),
					}))),
				),
				Entry("valid with expander priority and least-waste", core.ClusterAutoscaler{Expander: &expanderPriorityAndLeastWaste}, version_1_28, BeEmpty()),
				Entry("invalid expander in multiple expander string",
					core.ClusterAutoscaler{
						Expander: &invalidExpander,
					}, version_1_28,
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("expander"),
					}))),
				),
				Entry("incorrect multiple expander string",
					core.ClusterAutoscaler{
						Expander: &invalidMultipleExpanderString,
					}, version_1_28,
					ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("expander"),
					}))),
				),
				Entry("valid with newPodScaleUpDelay", core.ClusterAutoscaler{
					NewPodScaleUpDelay: &metav1.Duration{Duration: time.Minute},
				}, version_1_28, BeEmpty()),
				Entry("invalid with negative newPodScaleUpDelay", core.ClusterAutoscaler{
					NewPodScaleUpDelay: &negativeDuration,
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("newPodScaleUpDelay"), negativeDuration, "can not be negative"))),
				Entry("valid with maxEmptyBulkDelete", core.ClusterAutoscaler{
					MaxEmptyBulkDelete: &positiveInteger,
				}, version_1_28, BeEmpty()),
				Entry("invalid with negative maxEmptyBulkDelete", core.ClusterAutoscaler{
					MaxEmptyBulkDelete: &negativeInteger,
				}, version_1_28, ConsistOf(field.Invalid(field.NewPath("maxEmptyBulkDelete"), negativeInteger, "can not be negative"))),
			)

			Describe("taint validation", func() {
				var (
					clusterAutoscaler core.ClusterAutoscaler
					version           = "1.28.4"
					fldPath           *field.Path
				)

				It("should allow empty ignore taints list", func() {
					errList := ValidateClusterAutoscaler(clusterAutoscaler, version, fldPath)

					Expect(errList).To(BeEmpty())
				})

				It("should allow valid ignore taints list", func() {
					clusterAutoscaler.IgnoreTaints = []string{
						"allowed-1",
						"allowed-2",
					}

					errList := ValidateClusterAutoscaler(clusterAutoscaler, version, fldPath)

					Expect(errList).To(BeEmpty())
				})

				It("should deny reserved taint keys", func() {
					clusterAutoscaler.IgnoreTaints = []string{
						"node.gardener.cloud/critical-components-not-ready",
						"allowed-1",
					}

					errList := ValidateClusterAutoscaler(clusterAutoscaler, version, fldPath)

					Expect(errList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":   Equal(field.ErrorTypeForbidden),
							"Field":  Equal("ignoreTaints[0]"),
							"Detail": Equal("taint key is reserved by gardener"),
						})),
					))
				})
			})
		})

		Context("VerticalPodAutoscaler validation", func() {
			var (
				percentileLessThanZero   = -1.0
				percentileGreaterThanOne = 3.14
			)

			DescribeTable("verticalPod autoscaler values",
				func(verticalPodAutoscaler core.VerticalPodAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateVerticalPodAutoscaler(verticalPodAutoscaler, nil)).To(matcher)
				},
				Entry("valid", core.VerticalPodAutoscaler{}, BeEmpty()),
				Entry("invalid negative durations", core.VerticalPodAutoscaler{
					EvictAfterOOMThreshold:                   &negativeDuration,
					UpdaterInterval:                          &negativeDuration,
					RecommenderInterval:                      &negativeDuration,
					TargetCPUPercentile:                      &percentileLessThanZero,
					RecommendationLowerBoundCPUPercentile:    &percentileLessThanZero,
					RecommendationUpperBoundCPUPercentile:    &percentileGreaterThanOne,
					TargetMemoryPercentile:                   &percentileGreaterThanOne,
					RecommendationLowerBoundMemoryPercentile: &percentileLessThanZero,
					RecommendationUpperBoundMemoryPercentile: &percentileGreaterThanOne,
					CPUHistogramDecayHalfLife:                &negativeDuration,
					MemoryHistogramDecayHalfLife:             &negativeDuration,
					MemoryAggregationInterval:                &negativeDuration,
					MemoryAggregationIntervalCount:           ptr.To[int64](-1),
				}, ConsistOf(
					field.Invalid(field.NewPath("evictAfterOOMThreshold"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("updaterInterval"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("recommenderInterval"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("targetCPUPercentile"), percentileLessThanZero, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("recommendationLowerBoundCPUPercentile"), percentileLessThanZero, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("recommendationUpperBoundCPUPercentile"), percentileGreaterThanOne, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("targetMemoryPercentile"), percentileGreaterThanOne, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("recommendationLowerBoundMemoryPercentile"), percentileLessThanZero, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("recommendationUpperBoundMemoryPercentile"), percentileGreaterThanOne, "percentile value must be in the range [0, 1]"),
					field.Invalid(field.NewPath("cpuHistogramDecayHalfLife"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("memoryHistogramDecayHalfLife"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("memoryAggregationInterval"), negativeDuration.Duration.String(), "must be non-negative"),
					field.Invalid(field.NewPath("memoryAggregationIntervalCount"), int64(-1), "must be greater than or equal to 0"),
				)),
			)
		})

		Context("AuditConfig validation", func() {
			It("should forbid empty name", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig.AuditPolicy.ConfigMapRef.Name = ""
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.kubeAPIServer.auditConfig.auditPolicy.configMapRef.name"),
				}))))
			})

			It("should allow nil AuditConfig", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AuditConfig = nil
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})
		})

		Context("Authentication validation", func() {
			It("should forbid for version < v1.30", func() {
				shoot.Spec.Kubernetes.Version = "v1.29.0"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = nil
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &core.StructuredAuthentication{
					ConfigMapName: "foo",
				}
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthentication"),
					"Detail": Equal("is available for Kubernetes versions >= v1.30"),
				}))))
			})

			It("should forbid empty name", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = nil
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &core.StructuredAuthentication{}
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthentication.configMapName"),
					"Detail": Equal("must provide a name"),
				}))))
			})

			It("should forbid setting structured authentication when feature gate is disabled", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = nil
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &core.StructuredAuthentication{
					ConfigMapName: "foo",
				}
				shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = map[string]bool{
					"StructuredAuthenticationConfiguration": false,
				}
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthentication"),
					"Detail": Equal("requires feature gate StructuredAuthenticationConfiguration to be enabled"),
				}))))
			})

			It("should forbid setting both oidcConfig and structured authentication", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &core.StructuredAuthentication{
					ConfigMapName: "foo",
				}
				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.oidcConfig"),
					"Detail": Equal("is incompatible with structuredAuthentication"),
				}))))
			})

			It("should allow when config is valid", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig = nil
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthentication = &core.StructuredAuthentication{
					ConfigMapName: "foo",
				}

				Expect(ValidateShoot(shoot)).To(BeEmpty())
			})
		})

		Context("Authorization validation", func() {
			It("should forbid for version < v1.30", func() {
				shoot.Spec.Kubernetes.Version = "v1.29.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &core.StructuredAuthorization{
					ConfigMapName: "foo",
					Kubeconfigs:   []core.AuthorizerKubeconfigReference{{AuthorizerName: "foo", SecretName: "bar"}},
				}

				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthorization"),
					"Detail": Equal("is available for Kubernetes versions >= v1.30"),
				}))))
			})

			It("should forbid empty name", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &core.StructuredAuthorization{}

				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthorization.configMapName"),
					"Detail": Equal("must provide a name"),
				}))))
			})

			It("should forbid empty list of kubeconfig references", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &core.StructuredAuthorization{
					ConfigMapName: "foo",
				}

				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthorization.kubeconfigs"),
					"Detail": Equal("must provide kubeconfig secret references if an authorization config is configured"),
				}))))
			})

			It("should forbid setting structured authorization when feature gate is disabled", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &core.StructuredAuthorization{
					ConfigMapName: "foo",
					Kubeconfigs:   []core.AuthorizerKubeconfigReference{{}},
				}
				shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = map[string]bool{
					"StructuredAuthorizationConfiguration": false,
				}

				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.kubeAPIServer.structuredAuthorization"),
					"Detail": Equal("requires feature gate StructuredAuthorizationConfiguration to be enabled"),
				}))))
			})

			It("should allow when config is valid", func() {
				shoot.Spec.Kubernetes.Version = "v1.30.0"
				shoot.Spec.Kubernetes.KubeAPIServer.StructuredAuthorization = &core.StructuredAuthorization{
					ConfigMapName: "foo",
					Kubeconfigs: []core.AuthorizerKubeconfigReference{{
						AuthorizerName: "some-authz",
						SecretName:     "some-secret",
					}},
				}

				Expect(ValidateShoot(shoot)).To(BeEmpty())
			})
		})

		Context("FeatureGates validation", func() {
			It("should forbid invalid feature gates", func() {
				featureGates := map[string]bool{
					"AnyVolumeDataSource":  true,
					"GracefulNodeShutdown": true,
					"Foo":                  true,
				}
				shoot.Spec.Kubernetes.Version = "1.31.1"
				shoot.Spec.Kubernetes.KubeAPIServer.FeatureGates = featureGates
				shoot.Spec.Kubernetes.KubeControllerManager.FeatureGates = featureGates
				shoot.Spec.Kubernetes.KubeScheduler = &core.KubeSchedulerConfig{
					KubernetesConfig: core.KubernetesConfig{
						FeatureGates: featureGates,
					},
				}
				proxyMode := core.ProxyModeIPTables
				shoot.Spec.Kubernetes.KubeProxy = &core.KubeProxyConfig{
					KubernetesConfig: core.KubernetesConfig{
						FeatureGates: featureGates,
					},
					Mode: &proxyMode,
				}
				shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{
					KubernetesConfig: core.KubernetesConfig{
						FeatureGates: featureGates,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeAPIServer.featureGates.Foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeControllerManager.featureGates.Foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeScheduler.featureGates.Foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeProxy.featureGates.Foo"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubelet.featureGates.Foo"),
					})),
				))
			})
		})

		Context("Kubernetes Version", func() {
			It("should require a kubernetes version", func() {
				shoot.Spec.Kubernetes.Version = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.version"),
				}))
			})

			It("should forbid removing the kubernetes version", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = ""

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElements(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.kubernetes.version"),
						"Detail": Equal("cannot validate kubernetes version upgrade because it is unset"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
						"Detail": Equal("cannot validate kubernetes version upgrade because it is unset"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("spec.kubernetes.version"),
						"Detail": Equal("kubernetes version must not be empty"),
					})),
				))
			})

			It("should forbid kubernetes version downgrades", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.7.2"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.version"),
						"Detail": Equal("kubernetes version downgrade is not supported"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
						"Detail": Equal("kubernetes version downgrade is not supported"),
					})),
				))
			})

			It("should forbid kubernetes version upgrades skipping a minor version", func() {
				shoot.Spec.Kubernetes.Version = "1.25.2"
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.27.1"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.kubernetes.version"),
						"Detail": Equal("kubernetes version upgrade cannot skip a minor version"),
					})),
				))
			})
		})

		Context("worker pool kubernetes version", func() {
			It("should forbid worker pool kubernetes version higher than control plane", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.31.1")}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
					"Detail": Equal("worker group kubernetes version must not be higher than control plane version"),
				}))))
			})

			It("should allow to set worker pool kubernetes version equal to control plane version", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: &shoot.Spec.Kubernetes.Version}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("should allow to set worker pool kubernetes version lower one minor than control plane version", func() {
				shoot.Spec.Kubernetes.Version = "1.26.2"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.26.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.27.2"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("should allow to set worker pool kubernetes version lower two minor than control plane version", func() {
				shoot.Spec.Kubernetes.Version = "1.25.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.24.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.26.0"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("forbid to set worker pool kubernetes version lower three minor than control plane version for k8s version < 1.28", func() {
				shoot.Spec.Kubernetes.Version = "1.26.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.24.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.27.0"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
					"Detail": Equal("worker group kubernetes version must be at most two minor versions behind control plane version"),
				}))))
			})

			It("allow to set worker pool kubernetes version lower three minor than control plane version for k8s version >= 1.28", func() {
				shoot.Spec.Kubernetes.Version = "1.27.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.25.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.28.0"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("forbid to set worker pool kubernetes version lower four minor than control plane version for k8s version >= 1.28", func() {
				shoot.Spec.Kubernetes.Version = "1.27.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.24.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.Version = "1.28.0"

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
					"Detail": Equal("worker group kubernetes version must be at most three minor versions behind control plane version"),
				}))))
			})

			It("should allow to skip minor versions during worker pool kubernetes version upgrade", func() {
				shoot.Spec.Kubernetes.Version = "1.28.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.25.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.27.0")}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("should prevent skipping minor versions during Kubernetes version upgrades in worker pools when the update strategy is set to AutoInPlaceUpdate or ManualInPlaceUpdate", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))

				shoot.Spec.Kubernetes.Version = "1.28.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.25.2")}
				shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.27.0")}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0].kubernetes.version"),
					"Detail": Equal("kubernetes version upgrade cannot skip a minor version"),
				}))))
			})

			It("should allow to set worker pool kubernetes version to nil with one minor difference", func() {
				shoot.Spec.Kubernetes.Version = "1.25.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.24.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: nil}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})

			It("should allow to set worker pool kubernetes version to nil with more than one minor difference", func() {
				shoot.Spec.Kubernetes.Version = "1.28.0"
				shoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: ptr.To("1.25.2")}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Provider.Workers[0].Kubernetes = &core.WorkerKubernetes{Version: nil}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})
		})

		Context("networking section", func() {
			Context("Workerless Shoots", func() {
				It("should forbid setting networking.type, networking.providerConfig, networking.pods, networking.nodes", func() {
					shoot.Spec.Provider.Workers = nil
					shoot.Spec.SecretBindingName = nil
					shoot.Spec.Addons = nil
					shoot.Spec.Kubernetes.KubeControllerManager = nil
					shoot.Spec.Networking = &core.Networking{
						Type: ptr.To("some-type"),
						ProviderConfig: &runtime.RawExtension{
							Raw: []byte("foo"),
						},
						Pods:       ptr.To("0.0.0.0/0"),
						Nodes:      ptr.To("0.0.0.0/0"),
						Services:   ptr.To("0.0.0.0/0"),
						IPFamilies: []core.IPFamily{core.IPFamilyIPv4},
					}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.networking.type"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.networking.providerConfig"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.networking.pods"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
					}))
				})
			})

			It("should forbid empty Network configuration if shoot is having workers", func() {
				shoot.Spec.Networking = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("spec.networking"),
					"Detail": ContainSubstring("networking should not be nil for a Shoot with workers"),
				}))))
			})

			It("should forbid not specifying a networking type", func() {
				shoot.Spec.Networking.Type = ptr.To("")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.type"),
				}))))
			})

			It("should forbid changing the networking type", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Networking.Type = ptr.To("some-other-type")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networking.type"),
				}))))
			})

			It("should allow increasing the networking nodes range", func() {
				shoot.Spec.Networking.Nodes = ptr.To("10.181.0.0/18")
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Networking.Nodes = ptr.To("10.181.0.0/16")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid specifying unsupported IP family", func() {
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{"IPv5"}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.networking.ipFamilies[0]"),
				}))))
			})

			Context("IPv4", func() {
				It("should allow valid networking configuration", func() {
					shoot.Spec.Networking.Nodes = ptr.To("10.250.0.0/16")
					shoot.Spec.Networking.Services = ptr.To("100.64.0.0/13")
					shoot.Spec.Networking.Pods = ptr.To("100.96.0.0/11")

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid network CIDRs", func() {
					invalidCIDR := "invalid-cidr"

					shoot.Spec.Networking.Nodes = &invalidCIDR
					shoot.Spec.Networking.Services = &invalidCIDR
					shoot.Spec.Networking.Pods = &invalidCIDR

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}))
				})

				It("should forbid IPv6 CIDRs with IPv4 IP family", func() {
					shoot.Spec.Networking.Pods = ptr.To("2001:db8:1::/48")
					shoot.Spec.Networking.Nodes = ptr.To("2001:db8:2::/48")
					shoot.Spec.Networking.Services = ptr.To("2001:db8:3::/48")

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": ContainSubstring("must be a valid IPv4 address"),
					}))
				})

				It("should forbid non canonical CIDRs", func() {
					shoot.Spec.Networking.Nodes = ptr.To("10.250.0.3/16")
					shoot.Spec.Networking.Services = ptr.To("100.64.0.5/13")
					shoot.Spec.Networking.Pods = ptr.To("100.96.0.4/11")

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": Equal("must be valid canonical CIDR"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": Equal("must be valid canonical CIDR"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": Equal("must be valid canonical CIDR"),
					}))
				})
			})

			Context("IPv6", func() {
				BeforeEach(func() {
					shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6}
				})

				It("should allow valid networking configuration", func() {
					shoot.Spec.Networking.Pods = ptr.To("2001:db8:1::/48")
					shoot.Spec.Networking.Nodes = ptr.To("2001:db8:2::/48")
					shoot.Spec.Networking.Services = ptr.To("2001:db8:3::/48")

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(BeEmpty())
				})

				It("should forbid invalid network CIDRs", func() {
					invalidCIDR := "invalid-cidr"

					shoot.Spec.Networking.Nodes = &invalidCIDR
					shoot.Spec.Networking.Services = &invalidCIDR
					shoot.Spec.Networking.Pods = &invalidCIDR

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": ContainSubstring("invalid CIDR address"),
					}))
				})

				It("should forbid IPv4 CIDRs with IPv6 IP family", func() {
					shoot.Spec.Networking.Nodes = ptr.To("10.250.0.0/16")
					shoot.Spec.Networking.Services = ptr.To("100.64.0.0/13")
					shoot.Spec.Networking.Pods = ptr.To("100.96.0.0/11")

					errorList := ValidateShoot(shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": ContainSubstring("must be a valid IPv6 address"),
					}))
				})

				It("should forbid non canonical CIDRs", func() {
					shoot.Spec.Networking.Pods = ptr.To("2001:db8:1::1/48")
					shoot.Spec.Networking.Nodes = ptr.To("2001:db8:2::2/48")
					shoot.Spec.Networking.Services = ptr.To("2001:db8:3::3/48")

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.nodes"),
						"Detail": Equal("must be valid canonical CIDR"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.pods"),
						"Detail": Equal("must be valid canonical CIDR"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.networking.services"),
						"Detail": Equal("must be valid canonical CIDR"),
					}))
				})
			})

			It("should allow updating ipfamilies", func() {
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv4, core.IPFamilyIPv6}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("dual-stack", func() {
			BeforeEach(func() {
				shoot.Spec.Networking.IPFamilies = []core.IPFamily{core.IPFamilyIPv6, core.IPFamilyIPv4}
			})

			It("should allow IPv4 networking configuration", func() {
				shoot.Spec.Networking.Nodes = ptr.To("10.250.0.0/16")
				shoot.Spec.Networking.Services = ptr.To("100.64.0.0/13")
				shoot.Spec.Networking.Pods = ptr.To("100.96.0.0/11")
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("maintenance section", func() {
			It("should forbid invalid formats for the time window begin and end values", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "invalidformat"
				shoot.Spec.Maintenance.TimeWindow.End = "invalidformat"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.maintenance.timeWindow.begin/end"),
				}))))
			})

			It("should forbid time windows greater than 6 hours", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "145000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "215000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should forbid time windows smaller than 30 minutes", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "225000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "231000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should allow time windows which overlap over two days", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "230000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "010000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should not allow setting machineImageVersion for autoUpdate if it's a workerless Shoot", func() {
				shoot.Spec.Provider.Workers = nil
				shoot.Spec.Maintenance.AutoUpdate.MachineImageVersion = ptr.To(true)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ContainElements(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.maintenance.autoUpdate.machineImageVersion"),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot cluster"),
				}))))
			})
		})

		It("should forbid updating the spec for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.DeletionTimestamp = &deletionTimestamp
			newShoot.DeletionTimestamp = &deletionTimestamp
			newShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeForbidden),
				"Field":  Equal("spec"),
				"Detail": Equal("Maintenance.AutoUpdate.KubernetesVersion: false != true"),
			}))))
		})

		It("should allow updating the metadata for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.DeletionTimestamp = &deletionTimestamp
			newShoot.DeletionTimestamp = &deletionTimestamp
			newShoot.Labels = map[string]string{
				"new-key": "new-value",
			}

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(BeEmpty())
		})

		Describe("#ValidateSystemComponents", func() {
			DescribeTable("validate system components",
				func(systemComponents *core.SystemComponents, workerlessShoot bool, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateSystemComponents(systemComponents, nil, workerlessShoot)).To(matcher)
				},
				Entry("no system components", nil, false, BeEmpty()),
				Entry("no system components Workerless Shoot", nil, false, BeEmpty()),
				Entry("system components Workerless Shoot", &core.SystemComponents{}, true, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Detail": ContainSubstring("this field should not be set for workerless Shoot clusters"),
				})))),
				Entry("empty system components", &core.SystemComponents{}, false, BeEmpty()),
				Entry("empty core dns", &core.SystemComponents{CoreDNS: &core.CoreDNS{}}, false, BeEmpty()),
				Entry("horizontal core dns autoscaler", &core.SystemComponents{CoreDNS: &core.CoreDNS{Autoscaling: &core.CoreDNSAutoscaling{Mode: core.CoreDNSAutoscalingModeHorizontal}}}, false, BeEmpty()),
				Entry("cluster proportional core dns autoscaler", &core.SystemComponents{CoreDNS: &core.CoreDNS{Autoscaling: &core.CoreDNSAutoscaling{Mode: core.CoreDNSAutoscalingModeHorizontal}}}, false, BeEmpty()),
				Entry("incorrect core dns autoscaler", &core.SystemComponents{CoreDNS: &core.CoreDNS{Autoscaling: &core.CoreDNSAutoscaling{Mode: "dummy"}}}, false, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeNotSupported),
				})))),
			)
		})

		Describe("#ValidateCoreDNSRewritingCommonSuffixes", func() {
			DescribeTable("validate core dns rewriting common suffixes",
				func(commonSuffixes []string, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateCoreDNSRewritingCommonSuffixes(commonSuffixes, nil)).To(matcher)
				},
				Entry("should allow no common suffixes", nil, BeEmpty()),
				Entry("should allow empty common suffixes", []string{}, BeEmpty()),
				Entry("should allow normal common suffixes", []string{"gardener.cloud", "github.com", ".example.com"}, BeEmpty()),
				Entry("should not allow too few dots", []string{"foo", "foo.bar"}, ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"BadValue": Equal("foo"),
						"Detail":   ContainSubstring("must contain at least one non-leading dot"),
					})),
				)),
				Entry("should not allow duplicate entries", []string{"foo.bar.", ".foo.bar."}, ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeDuplicate),
						"BadValue": Equal("foo.bar."),
					})),
				)),
			)
		})

		Context("operation validation", func() {
			It("should do nothing if the operation annotation is not set", func() {
				Expect(ValidateShoot(shoot)).To(BeEmpty())
			})

			Context("CredentialsRotationWithoutWorkersRollout feature gate", func() {
				table := func(allow bool) {
					DescribeTable("validate specifying some operation annotations",
						func(key, value string) {
							matcher := ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
								"Type":   Equal(field.ErrorTypeForbidden),
								"Field":  Equal(fmt.Sprintf("metadata.annotations[%s]", key)),
								"Detail": ContainSubstring(fmt.Sprintf("the %s operation can only be used when the CredentialsRotationWithoutWorkersRollout feature gate is enabled", value)),
							})))

							metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, key, value)
							if !allow {
								Expect(ValidateShoot(shoot)).To(matcher)
							} else {
								Expect(ValidateShoot(shoot)).NotTo(matcher)
							}
						},

						Entry("gardener.cloud/operation=rotate-credentials-start-without-workers-rollout", "gardener.cloud/operation", "rotate-credentials-start-without-workers-rollout"),
						Entry("gardener.cloud/operation=rotate-ca-start-without-workers-rollout", "gardener.cloud/operation", "rotate-ca-start-without-workers-rollout"),
						Entry("gardener.cloud/operation=rotate-serviceaccount-key-start-without-workers-rollout", "gardener.cloud/operation", "rotate-serviceaccount-key-start-without-workers-rollout"),
						Entry("maintenance.gardener.cloud/operation=rotate-credentials-start-without-workers-rollout", "maintenance.gardener.cloud/operation", "rotate-credentials-start-without-workers-rollout"),
						Entry("maintenance.gardener.cloud/operation=rotate-ca-start-without-workers-rollout", "maintenance.gardener.cloud/operation", "rotate-ca-start-without-workers-rollout"),
						Entry("maintenance.gardener.cloud/operation=rotate-serviceaccount-key-start-without-workers-rollout", "maintenance.gardener.cloud/operation", "rotate-serviceaccount-key-start-without-workers-rollout"),
					)
				}

				When("feature gate is disabled", func() {
					BeforeEach(func() {
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.CredentialsRotationWithoutWorkersRollout, false))
					})

					table(false)
				})

				When("feature gate is enabled", func() {
					BeforeEach(func() {
						DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.CredentialsRotationWithoutWorkersRollout, true))
					})

					table(true)
				})
			})

			DescribeTable("starting rotation of all credentials",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-credentials-start")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("shoot was never created successfully", false, core.ShootStatus{}),
				Entry("shoot is still being created", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateProcessing,
					},
				}),
				Entry("shoot was created successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("shoot is in reconciliation phase", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
				}),
				Entry("shoot is in deletion phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeDelete,
					},
				}),
				Entry("shoot is in migration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeMigrate,
					},
				}),
				Entry("shoot is in restoration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeRestore,
					},
				}),
				Entry("shoot was restored successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeRestore,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("ca rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("sa rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("sa rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("sa rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("when shoot spec encrypted resources and status encrypted resources are not equal", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					EncryptedResources: []string{"configmaps"},
				}),
				Entry("when AutoInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							AutoInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
				Entry("when ManualInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							ManualInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
			)

			DescribeTable("completing rotation of all credentials",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-credentials-complete")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPreparing,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparing,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("all rotation phases are prepared", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleting,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleting,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleted,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("sa rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleted,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("etcd key rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting CA rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-ca-start")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("shoot was never created successfully", false, core.ShootStatus{}),
				Entry("shoot is still being created", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateProcessing,
					},
				}),
				Entry("shoot was created successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("shoot is in reconciliation phase", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
				}),
				Entry("shoot is in deletion phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeDelete,
					},
				}),
				Entry("shoot is in migration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeMigrate,
					},
				}),
				Entry("shoot is in restoration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeRestore,
					},
				}),
				Entry("shoot was restored successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeRestore,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("ca rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("when AutoInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							AutoInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
				Entry("when ManualInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							ManualInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
			)

			DescribeTable("completing CA rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-ca-complete")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("ca rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("ca rotation phase is prepared", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("ca rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("ca rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting service account key rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-start")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("shoot was never created successfully", false, core.ShootStatus{}),
				Entry("shoot is still being created", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateProcessing,
					},
				}),
				Entry("shoot was created successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("shoot is in reconciliation phase", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
				}),
				Entry("shoot is in deletion phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeDelete,
					},
				}),
				Entry("shoot is in migration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeMigrate,
					},
				}),
				Entry("shoot is in restoration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeRestore,
					},
				}),
				Entry("shoot was restored successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeRestore,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("when AutoInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							AutoInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
				Entry("when ManualInPlaceUpdate workers rollout is pending", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							ManualInPlaceUpdate: []string{"worker-1"},
						},
					},
				}),
			)

			DescribeTable("completing service account key rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-serviceaccount-key-complete")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("rotation phase is preparing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is completing", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
			)

			DescribeTable("starting ETCD encryption key rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-etcd-encryption-key-start")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("shoot was never created successfully", false, core.ShootStatus{}),
				Entry("shoot is still being created", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateProcessing,
					},
				}),
				Entry("shoot was created successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("shoot is in reconciliation phase", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
				}),
				Entry("shoot is in deletion phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeDelete,
					},
				}),
				Entry("shoot is in migration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeMigrate,
					},
				}),
				Entry("shoot is in restoration phase", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeRestore,
					},
				}),
				Entry("shoot was restored successfully", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeRestore,
						State: core.LastOperationStateSucceeded,
					},
				}),
				Entry("rotation phase is prepare", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is complete", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
				Entry("when shoot spec encrypted resources and status encrypted resources are not equal", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					EncryptedResources: []string{"configmaps"},
				}),
			)

			DescribeTable("completing ETCD encryption key rotation",
				func(allowed bool, status core.ShootStatus) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-complete")
					shoot.Status = status

					matcher := BeEmpty()
					if !allowed {
						matcher = ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeForbidden),
							"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
						})))
					}

					Expect(ValidateShoot(shoot)).To(matcher)
				},

				Entry("rotation phase is prepare", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("rotation phase is prepared", true, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPrepared,
							},
						},
					},
				}),
				Entry("rotation phase is complete", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("rotation phase is completed", false, core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type: core.LastOperationTypeReconcile,
					},
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleted,
							},
						},
					},
				}),
			)

			It("should return an error if the operation annotation is invalid", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "foo-bar")
				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("metadata.annotations[gardener.cloud/operation]"),
				}))))
			})

			It("should return an error if the maintenance operation annotation is invalid", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "foo-bar")
				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
				}))))
			})

			It("should return an error if maintenance annotation is not allowed in this context", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-etcd-encryption-key-complete")
				shoot.Status = core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}
				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
				}))))
			})

			It("should return an error if both operation annotations have the same value", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-etcd-encryption-key-start")
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-etcd-encryption-key-start")
				shoot.Status = core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}
				Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("metadata.annotations"),
				}))))
			})

			It("should return nothing if maintenance annotation is valid", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "reconcile")
				Expect(ValidateShoot(shoot)).To(BeEmpty())
			})

			It("should return nothing if both operation annotations are valid and do not have the same value", func() {
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-serviceaccount-key-start")
				metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", "rotate-etcd-encryption-key-start")
				shoot.Status = core.ShootStatus{
					LastOperation: &core.LastOperation{
						Type:  core.LastOperationTypeCreate,
						State: core.LastOperationStateSucceeded,
					},
				}
				Expect(ValidateShoot(shoot)).To(BeEmpty())
			})

			DescribeTable("It should forbid setting certain operation annotations when shoot has a maintenance annotation",
				func(maintenanceOpAnnotation, operationAnnotation, errString string) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", maintenanceOpAnnotation)
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", operationAnnotation)

					shoot.Status = core.ShootStatus{
						LastOperation: &core.LastOperation{
							Type:  core.LastOperationTypeCreate,
							State: core.LastOperationStateSucceeded,
						},
					}

					if sets.New(v1beta1constants.OperationRotateCredentialsComplete,
						v1beta1constants.OperationRotateCAComplete,
						v1beta1constants.OperationRotateServiceAccountKeyComplete,
						v1beta1constants.OperationRotateETCDEncryptionKeyComplete).Has(maintenanceOpAnnotation) {
						shoot.Status.Credentials = &core.ShootCredentials{
							Rotation: &core.ShootCredentialsRotation{
								CertificateAuthorities: &core.CARotation{
									Phase: core.RotationPrepared,
								},
								ServiceAccountKey: &core.ServiceAccountKeyRotation{
									Phase: core.RotationPrepared,
								},
								ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
									Phase: core.RotationPrepared,
								},
							},
						}
					}

					Expect(ValidateShoot(shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": ContainSubstring(errString),
					}))))

					delete(shoot.Annotations, "maintenance.gardener.cloud/operation")
					delete(shoot.Annotations, "gardener.cloud/operation")
				},

				Entry("rotate-ca-start", "rotate-credentials-start", "rotate-ca-start", "operation 'rotate-ca-start' is not permitted when maintenance operation is 'rotate-credentials-start'"),
				Entry("rotate-ca-start-without-workers-rollout", "rotate-credentials-start", "rotate-ca-start-without-workers-rollout", "operation 'rotate-ca-start-without-workers-rollout' is not permitted when maintenance operation is 'rotate-credentials-start'"),
				Entry("rotate-serviceaccount-key-start", "rotate-credentials-start", "rotate-serviceaccount-key-start", "operation 'rotate-serviceaccount-key-start' is not permitted when maintenance operation is 'rotate-credentials-start'"),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout", "rotate-credentials-start", "rotate-serviceaccount-key-start-without-workers-rollout", "operation 'rotate-serviceaccount-key-start-without-workers-rollout' is not permitted when maintenance operation is 'rotate-credentials-start'"),
				Entry("rotate-etcd-encryption-key-start", "rotate-credentials-start", "rotate-etcd-encryption-key-start", "operation 'rotate-etcd-encryption-key-start' is not permitted when maintenance operation is 'rotate-credentials-start'"),

				Entry("rotate-ca-complete", "rotate-credentials-complete", "rotate-ca-complete", "operation 'rotate-ca-complete' is not permitted when maintenance operation is 'rotate-credentials-complete'"),
				Entry("rotate-serviceaccount-key-complete", "rotate-credentials-complete", "rotate-serviceaccount-key-complete", "operation 'rotate-serviceaccount-key-complete' is not permitted when maintenance operation is 'rotate-credentials-complete'"),
				Entry("rotate-etcd-encryption-key-complete", "rotate-credentials-complete", "rotate-etcd-encryption-key-complete", "operation 'rotate-etcd-encryption-key-complete' is not permitted when maintenance operation is 'rotate-credentials-complete'"),

				Entry("rotate-credentials-start", "rotate-ca-start", "rotate-credentials-start", "operation 'rotate-credentials-start' is not permitted when maintenance operation is 'rotate-ca-start'"),
				Entry("rotate-credentials-start", "rotate-serviceaccount-key-start", "rotate-credentials-start", "operation 'rotate-credentials-start' is not permitted when maintenance operation is 'rotate-serviceaccount-key-start'"),
				Entry("rotate-credentials-start", "rotate-etcd-encryption-key-start", "rotate-credentials-start", "operation 'rotate-credentials-start' is not permitted when maintenance operation is 'rotate-etcd-encryption-key-start'"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-ca-start", "rotate-credentials-start-without-workers-rollout", "operation 'rotate-credentials-start-without-workers-rollout' is not permitted when maintenance operation is 'rotate-ca-start'"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-serviceaccount-key-start", "rotate-credentials-start-without-workers-rollout", "operation 'rotate-credentials-start-without-workers-rollout' is not permitted when maintenance operation is 'rotate-serviceaccount-key-start'"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-etcd-encryption-key-start", "rotate-credentials-start-without-workers-rollout", "operation 'rotate-credentials-start-without-workers-rollout' is not permitted when maintenance operation is 'rotate-etcd-encryption-key-start'"),

				Entry("rotate-credentials-complete", "rotate-ca-complete", "rotate-credentials-complete", "operation 'rotate-credentials-complete' is not permitted when maintenance operation is 'rotate-ca-complete'"),
				Entry("rotate-credentials-complete", "rotate-serviceaccount-key-complete", "rotate-credentials-complete", "operation 'rotate-credentials-complete' is not permitted when maintenance operation is 'rotate-serviceaccount-key-complete'"),
				Entry("rotate-credentials-complete", "rotate-etcd-encryption-key-complete", "rotate-credentials-complete", "operation 'rotate-credentials-complete' is not permitted when maintenance operation is 'rotate-etcd-encryption-key-complete'"),
			)

			DescribeTable("forbid certain rotation operations when shoot is hibernated",
				func(operation string) {
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", operation)
					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": ContainSubstring("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
					delete(shoot.Annotations, "gardener.cloud/operation")

					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", operation)
					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						"Detail": ContainSubstring("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
					delete(shoot.Annotations, "maintenance.gardener.cloud/operation")
				},

				Entry("rotate-credentials-start", "rotate-credentials-start"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-credentials-start-without-workers-rollout"),
				Entry("rotate-credentials-complete", "rotate-credentials-complete"),
				Entry("rotate-etcd-encryption-key-start", "rotate-etcd-encryption-key-start"),
				Entry("rotate-etcd-encryption-key-complete", "rotate-etcd-encryption-key-complete"),
				Entry("rotate-serviceaccount-key-start", "rotate-serviceaccount-key-start"),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout", "rotate-serviceaccount-key-start-without-workers-rollout"),
				Entry("rotate-serviceaccount-key-complete", "rotate-serviceaccount-key-complete"),
				Entry("rotate-rollout-workers", "rotate-rollout-workers=worker-name"),
			)

			Context("trigger workers rollout", func() {
				It("should forbid triggering workers rollout when pool does not exist", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=foo")

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("worker pool name foo does not exist in .spec.provider.workers[]"),
					}))))
				})

				It("should forbid triggering workers rollout when rotation phase is not in 'WaitingForWorkersRollout'", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=worker-name")

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("either .status.credentials.rotation.certificateAuthorities.phase or .status.credentials.rotation.serviceAccountKey.phase must be in 'WaitingForWorkersRollout' in order to trigger workers rollout"),
					}))))
				})

				It("should forbid triggering workers rollout when shoot is hibernated", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=worker-name")
					shoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationWaitingForWorkersRollout,
							},
						},
					}
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
				})

				It("should forbid triggering workers rollout when shoot is waking up", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=worker-name")
					shoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationWaitingForWorkersRollout,
							},
						},
					}
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
					shoot.Status = core.ShootStatus{IsHibernated: true}

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
				})

				It("should forbid triggering workers rollout without stating any pool", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=")
					shoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationWaitingForWorkersRollout,
							},
						},
					}

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": Equal("must provide at least one pool name via rotate-rollout-workers=<poolName1>[,<poolName2>,...]"),
					}))))
				})

				It("should forbid triggering workers rollout with duplicate pool names", func() {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", "rotate-rollout-workers=worker-name,worker-name")
					shoot.Status.Credentials = &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							CertificateAuthorities: &core.CARotation{
								Phase: core.RotationWaitingForWorkersRollout,
							},
						},
					}

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeDuplicate),
						"Field":    Equal("metadata.annotations[gardener.cloud/operation]"),
						"BadValue": Equal("pool name worker-name was specified multiple times"),
					}))))
				})
			})

			DescribeTable("forbid certain rotation operations when shoot is waking up",
				func(operation string) {
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
					shoot.Status = core.ShootStatus{
						IsHibernated: true,
					}

					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "gardener.cloud/operation", operation)
					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[gardener.cloud/operation]"),
						"Detail": ContainSubstring("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
					delete(shoot.Annotations, "gardener.cloud/operation")

					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", operation)
					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("metadata.annotations[maintenance.gardener.cloud/operation]"),
						"Detail": ContainSubstring("operation is not permitted when shoot is hibernated or is waking up"),
					}))))
					delete(shoot.Annotations, "maintenance.gardener.cloud/operation")
				},

				Entry("rotate-credentials-start", "rotate-credentials-start"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-credentials-start-without-workers-rollout"),
				Entry("rotate-credentials-complete", "rotate-credentials-complete"),
				Entry("rotate-etcd-encryption-key-start", "rotate-etcd-encryption-key-start"),
				Entry("rotate-etcd-encryption-key-complete", "rotate-etcd-encryption-key-complete"),
				Entry("rotate-serviceaccount-key-start", "rotate-serviceaccount-key-start"),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout", "rotate-serviceaccount-key-start-without-workers-rollout"),
				Entry("rotate-serviceaccount-key-complete", "rotate-serviceaccount-key-complete"),
			)

			DescribeTable("not forbid certain rotation maintenance operations when shoot is in deletion",
				func(operation string) {
					shoot.DeletionTimestamp = &metav1.Time{}

					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", operation)
					Expect(ValidateShoot(shoot)).To(BeEmpty())
					delete(shoot.Annotations, "maintenance.gardener.cloud/operation")
				},

				Entry("rotate-credentials-start", "rotate-credentials-start"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-credentials-start-without-workers-rollout"),
				Entry("rotate-credentials-complete", "rotate-credentials-complete"),
				Entry("rotate-etcd-encryption-key-start", "rotate-etcd-encryption-key-start"),
				Entry("rotate-etcd-encryption-key-complete", "rotate-etcd-encryption-key-complete"),
				Entry("rotate-serviceaccount-key-start", "rotate-serviceaccount-key-start"),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout", "rotate-serviceaccount-key-start-without-workers-rollout"),
				Entry("rotate-serviceaccount-key-complete", "rotate-serviceaccount-key-complete"),
			)

			DescribeTable("forbid hibernating the shoot when certain rotation maintenance operations are set",
				func(operation string) {
					metav1.SetMetaDataAnnotation(&shoot.ObjectMeta, "maintenance.gardener.cloud/operation", operation)
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

					Expect(ValidateShoot(shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("spec.hibernation.enabled"),
						"Detail": ContainSubstring("shoot cannot be hibernated when maintenance.gardener.cloud/operation=" + operation + " annotation is set"),
					}))))
				},

				Entry("rotate-credentials-start", "rotate-credentials-start"),
				Entry("rotate-credentials-start-without-workers-rollout", "rotate-credentials-start-without-workers-rollout"),
				Entry("rotate-credentials-complete", "rotate-credentials-complete"),
				Entry("rotate-etcd-encryption-key-start", "rotate-etcd-encryption-key-start"),
				Entry("rotate-etcd-encryption-key-complete", "rotate-etcd-encryption-key-complete"),
				Entry("rotate-serviceaccount-key-start", "rotate-serviceaccount-key-start"),
				Entry("rotate-serviceaccount-key-start-without-workers-rollout", "rotate-serviceaccount-key-start-without-workers-rollout"),
				Entry("rotate-serviceaccount-key-complete", "rotate-serviceaccount-key-complete"),
			)

			DescribeTable("forbid hibernating the shoot when certain rotation operations are in progress",
				func(status core.ShootStatus) {
					shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}
					shoot.Status = status

					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}

					Expect(ValidateShootUpdate(shoot, oldShoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.hibernation.enabled"),
						"Detail": And(
							ContainSubstring("shoot cannot be hibernated"),
							Or(
								ContainSubstring("phase is %q", "Preparing"),
								ContainSubstring("phase is %q", "Completing"),
							),
						),
					}))))
				},
				Entry("ETCD encryption key rotation is in Preparing phase", core.ShootStatus{
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("ETCD encryption key rotation is in Completing phase", core.ShootStatus{
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ETCDEncryptionKey: &core.ETCDEncryptionKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
				Entry("ServiceAccount key rotation is in Preparing phase", core.ShootStatus{
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparing,
							},
						},
					},
				}),
				Entry("ServiceAccount key rotation is in Completing phase", core.ShootStatus{
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationCompleting,
							},
						},
					},
				}),
			)

			It("should forbid hibernating the shoot when ServiceAccount key rotation is in PreparingWithoutWorkersRollout phase", func() {
				shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}
				shoot.Status = core.ShootStatus{
					Credentials: &core.ShootCredentials{
						Rotation: &core.ShootCredentialsRotation{
							ServiceAccountKey: &core.ServiceAccountKeyRotation{
								Phase: core.RotationPreparingWithoutWorkersRollout,
							},
						},
					},
				}

				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}

				Expect(ValidateShootUpdate(shoot, oldShoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.hibernation.enabled"),
					"Detail": And(
						ContainSubstring("shoot cannot be hibernated"),
						ContainSubstring("phase is %q", "PreparingWithoutWorkersRollout"),
					),
				}))))
			})

			It("should forbid hibernation when the spec encryption config and status encryption config are different", func() {
				shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
				shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					EncryptionConfig: &core.EncryptionConfig{
						Resources: []string{"events", "configmaps"},
					},
				}
				shoot.Status.EncryptedResources = []string{"events"}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.hibernation.enabled"),
					"Detail": ContainSubstring("when spec.kubernetes.kubeAPIServer.encryptionConfig.resources and status.encryptedResources are not equal"),
				}))))
			})

			It("should allow hibernation when the spec encryption config and status encryption config are the same", func() {
				shoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(false)}
				shoot.Spec.Kubernetes.KubeAPIServer = &core.KubeAPIServerConfig{
					EncryptionConfig: &core.EncryptionConfig{
						Resources: []string{"events", "configmaps"},
					},
				}
				shoot.Status.EncryptedResources = []string{"configmaps", "events"}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Hibernation = &core.Hibernation{Enabled: ptr.To(true)}

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})
		})

		Context("scheduler name", func() {
			It("allow setting the default scheduler name when name was 'nil'", func() {
				shoot.Spec.SchedulerName = nil
				oldShoot := shoot.DeepCopy()
				shoot.Spec.SchedulerName = ptr.To("default-scheduler")

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, metav1.ObjectMeta{}, field.NewPath("spec"))

				Expect(errorList).To(BeEmpty())
			})

			It("forbid changing the scheduler name when name was 'nil'", func() {
				shoot.Spec.SchedulerName = nil
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SchedulerName = ptr.To("foo-scheduler")

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, metav1.ObjectMeta{}, field.NewPath("spec"))

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.schedulerName"),
				}))))
			})

			It("forbid changing the scheduler name when configured before", func() {
				shoot.Spec.SchedulerName = ptr.To("foo-scheduler")
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SchedulerName = ptr.To("bar-scheduler")

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, metav1.ObjectMeta{}, field.NewPath("spec"))

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.schedulerName"),
				}))))
			})
		})

		Context("node-local-dns update", func() {
			It("should forbid toggling the node-local-dns if one of the worker has pool updateStrategy AutoInPlaceUpdate/ManualInPlaceUpdate", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers[0])
				shoot.Spec.Provider.Workers[1].Name = "worker-2"
				shoot.Spec.Provider.Workers[1].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
				shoot.Spec.SystemComponents = &core.SystemComponents{
					NodeLocalDNS: &core.NodeLocalDNS{
						Enabled: false,
					},
				}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SystemComponents.NodeLocalDNS.Enabled = true

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.systemComponents.nodeLocalDNS"),
					"Detail": Equal("node-local-dns setting can not be changed if shoot has at least one worker pool with update strategy AutoInPlaceUpdate/ManualInPlaceUpdate"),
				}))))

				shoot.Spec.SystemComponents = &core.SystemComponents{
					NodeLocalDNS: &core.NodeLocalDNS{
						Enabled: true,
					},
				}

				newShoot = prepareShootForUpdate(shoot)
				newShoot.Spec.SystemComponents.NodeLocalDNS = nil

				Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.systemComponents.nodeLocalDNS"),
					"Detail": Equal("node-local-dns setting can not be changed if shoot has at least one worker pool with update strategy AutoInPlaceUpdate/ManualInPlaceUpdate"),
				}))))
			})

			It("should allow toggling the node-local-dns if none of the worker pool has updateStrategy AutoInPlaceUpdate/ManualInPlaceUpdate", func() {
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers[0])
				shoot.Spec.Provider.Workers[1].Name = "worker-2"
				shoot.Spec.SystemComponents = &core.SystemComponents{
					NodeLocalDNS: &core.NodeLocalDNS{
						Enabled: false,
					},
				}

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.SystemComponents.NodeLocalDNS.Enabled = true

				Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
			})
		})

		Describe("#ValidateProviderUpdate", func() {
			Context("worker pool updateStrategy", func() {
				It("should forbid changing the update strategy from AutoRollingUpdate to AutoInPlaceUpdate/ManualInPlaceUpdate", func() {
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].updateStrategy"),
						"Detail": Equal("update strategy cannot be changed from AutoRollingUpdate to AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))

					newShoot = prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].updateStrategy"),
						"Detail": Equal("update strategy cannot be changed from AutoRollingUpdate to AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))
				})

				It("should forbid changing the update strategy from AutoInPlaceUpdate/ManualInPlaceUpdate to AutoRollingUpdate", func() {
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoRollingUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].updateStrategy"),
						"Detail": Equal("update strategy cannot be changed from AutoInPlaceUpdate/ManualInPlaceUpdate to AutoRollingUpdate"),
					}))))

					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					newShoot = prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoRollingUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].updateStrategy"),
						"Detail": Equal("update strategy cannot be changed from AutoInPlaceUpdate/ManualInPlaceUpdate to AutoRollingUpdate"),
					}))))
				})

				It("should allow changing the update strategy from AutoInPlaceUpdate to ManualInPlaceUpdate", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
					newShoot.Spec.Provider.Workers[0].MaxSurge = ptr.To(intstr.FromInt32(0))
					newShoot.Spec.Provider.Workers[0].MaxUnavailable = ptr.To(intstr.FromInt32(1))

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should allow changing the update strategy from ManualInPlaceUpdate to AutoInPlaceUpdate", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.ManualInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})

				It("should forbid setting in-place update strategy for a new worker pool if the InPlaceNodeUpdates feature gate is disabled", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, false))
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, newShoot.Spec.Provider.Workers[0])
					newShoot.Spec.Provider.Workers[1].Name = "worker-2"
					newShoot.Spec.Provider.Workers[1].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[1].updateStrategy"),
						"Detail": Equal("can not configure `AutoInPlaceUpdate` or `ManualInPlaceUpdate` update strategies when the `InPlaceNodeUpdates` feature gate is disabled."),
					}))))
				})

				It("should allow using the in-place update strategy for an existing worker pool even if the InPlaceNodeUpdates feature gate is disabled", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, false))
					shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, shoot.Spec.Provider.Workers[0])
					shoot.Spec.Provider.Workers[1].Name = "worker-2"
					shoot.Spec.Provider.Workers[1].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					Expect(ValidateShootUpdate(newShoot, shoot)).To(BeEmpty())
				})
			})

			Context("worker pool update strategy is either AutoInplaceUpdate or ManualInPlaceUpdate", func() {
				It("should forbid changing the machine type", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].Machine.Type = "foo"

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].machine.type"),
						"Detail": Equal("machine type cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))
				})

				It("should forbid changing the machine image", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].Machine.Image.Name = "foo"

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].machine.image.name"),
						"Detail": Equal("machine image name cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))
				})

				It("should forbid changing the cri", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					shoot.Spec.Provider.Workers[0].CRI = &core.CRI{Name: "foo"}
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].CRI.Name = "bar"

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].cri.name"),
						"Detail": Equal("CRI name cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))
				})

				It("should forbid changing the volume details", func() {
					DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
					shoot.Spec.Provider.Workers[0].UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
					shoot.Spec.Provider.Workers[0].Volume = &core.Volume{Name: ptr.To("foo")}
					newShoot := prepareShootForUpdate(shoot)

					newShoot.Spec.Provider.Workers[0].Volume.Name = ptr.To("bar")

					Expect(ValidateShootUpdate(newShoot, shoot)).To(ContainElement(PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].volume"),
						"Detail": Equal("volume cannot be changed if update strategy is AutoInPlaceUpdate/ManualInPlaceUpdate"),
					}))))
				})
			})
		})
	})

	Describe("#ValidateShootStatus, #ValidateShootStatusUpdate", func() {
		var (
			shoot    *core.Shoot
			newShoot *core.Shoot
		)
		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec:   core.ShootSpec{},
				Status: core.ShootStatus{},
			}

			newShoot = prepareShootForUpdate(shoot)
		})

		Context("uid checks", func() {
			It("should allow setting the uid", func() {
				newShoot.Status.UID = "1234"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid changing the uid", func() {
				shoot.Status.UID = "1234"
				newShoot.Status.UID = "1235"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.uid"),
				}))
			})
		})

		Context("technical id checks", func() {
			It("should allow setting the technical id", func() {
				newShoot.Status.TechnicalID = "shoot--foo--bar"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid changing the technical id", func() {
				shoot.Status.TechnicalID = "shoot-foo-bar"
				newShoot.Status.TechnicalID = "shoot--foo--bar"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("status.technicalID"),
				}))
			})
		})

		Context("validate shoot cluster identity update", func() {
			clusterIdentity := "newClusterIdentity"
			It("should not fail to set the cluster identity if it is missing", func() {
				newShoot.Status.ClusterIdentity = &clusterIdentity
				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail to set the cluster identity if it is already set", func() {
				newShoot.Status.ClusterIdentity = &clusterIdentity
				shoot.Status.ClusterIdentity = ptr.To("oldClusterIdentity")
				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.clusterIdentity"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})
		})

		Context("validate shoot advertise address update", func() {
			It("should fail for empty name", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: ""},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("status.advertisedAddresses[0].name"),
					"Detail": ContainSubstring(`field must not be empty`),
				}))
			})

			It("should fail for duplicate name", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://foo.bar"},
					{Name: "a", URL: "https://foo.bar"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("status.advertisedAddresses[1].name"),
				}))
			})

			It("should fail for invalid URL", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "://foo.bar"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeRequired),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`url must be a valid URL:`),
				}))
			})

			It("should fail for http URL", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "http://foo.bar"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`'https' is the only allowed URL scheme`),
				}))
			})

			It("should fail for URL without host", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`host must be provided`),
				}))
			})

			It("should not fail for URL with path", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://foo.bar/baz"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(BeEmpty())
			})

			It("should fail for URL with user information", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://john:doe@foo.bar"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`user information is not permitted in the URL`),
				}))
			})

			It("should fail for URL with fragment", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://foo.bar#some-fragment"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`fragments are not permitted in the URL`),
				}))
			})

			It("should fail for URL with query parameters", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://foo.bar?some=query"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.advertisedAddresses[0].url"),
					"Detail": ContainSubstring(`query parameters are not permitted in the URL`),
				}))
			})

			It("should succeed correct addresses", func() {
				newShoot.Status.AdvertisedAddresses = []core.ShootAdvertisedAddress{
					{Name: "a", URL: "https://foo.bar"},
					{Name: "b", URL: "https://foo.bar:443"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("validate shoot networking status", func() {
			It("should allow valid networking configuration", func() {
				newShoot.Status.Networking = &core.NetworkingStatus{
					Nodes:       []string{"10.250.0.0/16"},
					Pods:        []string{"100.96.0.0/11"},
					Services:    []string{"100.64.0.0/13"},
					EgressCIDRs: []string{"1.2.3.4/32"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(BeEmpty())
			})

			It("should forbid invalid network CIDRs", func() {
				invalidCIDR := "invalid-cidr"

				newShoot.Status.Networking = &core.NetworkingStatus{
					Nodes:       []string{invalidCIDR},
					Pods:        []string{invalidCIDR},
					Services:    []string{invalidCIDR},
					EgressCIDRs: []string{invalidCIDR},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.nodes[0]"),
					"Detail": ContainSubstring("invalid CIDR address"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.pods[0]"),
					"Detail": ContainSubstring("invalid CIDR address"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.services[0]"),
					"Detail": ContainSubstring("invalid CIDR address"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.egressCIDRs[0]"),
					"Detail": ContainSubstring("invalid CIDR address"),
				}))
			})

			It("should forbid non-canonical CIDRs", func() {
				newShoot.Status.Networking = &core.NetworkingStatus{
					Nodes:       []string{"10.250.0.3/16"},
					Pods:        []string{"100.64.0.5/13"},
					Services:    []string{"100.96.0.4/11"},
					EgressCIDRs: []string{"1.2.3.4/24"},
				}

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.nodes[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.pods[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.services[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.networking.egressCIDRs[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})
		})
	})

	Describe("#ValidateWorker", func() {
		DescribeTable("validate worker machine",
			func(machine core.Machine, matcher gomegatypes.GomegaMatcher) {
				maxSurge := intstr.FromInt32(1)
				maxUnavailable := intstr.FromInt32(0)
				worker := core.Worker{
					Name:           "worker-name",
					Machine:        machine,
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				}
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)

				Expect(errList).To(matcher)
			},

			Entry("empty machine type",
				core.Machine{
					Type: "",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.type"),
				}))),
			),
			Entry("empty machine image name",
				core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.image.name"),
				}))),
			),
			Entry("empty machine image version",
				core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "",
					},
					Architecture: ptr.To("amd64"),
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.image.version"),
				}))),
			),
			Entry("nil machine architecture",
				core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: nil,
				},
				BeEmpty(),
			),
		)

		DescribeTable("reject when maxUnavailable and maxSurge are invalid",
			func(updateStrategy core.MachineUpdateStrategy, maxUnavailable, maxSurge intstr.IntOrString, expectType field.ErrorType) {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
						Type: "large",
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
						Architecture: ptr.To("amd64"),
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					UpdateStrategy: &updateStrategy,
				}
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// double zero values (percent or int)
			Entry("two zero integers", core.AutoRollingUpdate, intstr.FromInt32(0), intstr.FromInt32(0), field.ErrorTypeInvalid),
			Entry("zero int and zero percent", core.AutoRollingUpdate, intstr.FromInt32(0), intstr.FromString("0%"), field.ErrorTypeInvalid),
			Entry("zero percent and zero int", core.AutoRollingUpdate, intstr.FromString("0%"), intstr.FromInt32(0), field.ErrorTypeInvalid),
			Entry("two zero percents", core.AutoRollingUpdate, intstr.FromString("0%"), intstr.FromString("0%"), field.ErrorTypeInvalid),

			// greater than 100
			Entry("maxUnavailable greater than 100 percent", core.AutoRollingUpdate, intstr.FromString("101%"), intstr.FromString("100%"), field.ErrorTypeInvalid),

			// below zero tests
			Entry("values are not below zero", core.AutoRollingUpdate, intstr.FromInt32(-1), intstr.FromInt32(0), field.ErrorTypeInvalid),
			Entry("percentage is not less than zero", core.AutoRollingUpdate, intstr.FromString("-90%"), intstr.FromString("90%"), field.ErrorTypeInvalid),

			// manual in-place update tests
			Entry("maxSurge must be 0 in case of ManualInplaceUpdate update strategy", core.ManualInPlaceUpdate, intstr.FromInt32(1), intstr.FromInt32(1), field.ErrorTypeInvalid),
			Entry("maxUnavailable should not be 0 in case of ManualInplaceUpdate update strategy", core.ManualInPlaceUpdate, intstr.FromInt32(0), intstr.FromInt32(0), field.ErrorTypeInvalid),
		)

		DescribeTable("reject when labels are invalid",
			func(labels map[string]string, expectType field.ErrorType) {
				maxSurge := intstr.FromInt32(1)
				maxUnavailable := intstr.FromInt32(0)
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
						Type: "large",
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
						Architecture: ptr.To("amd64"),
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Labels:         labels,
				}
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", map[string]string{"/foo": "bar"}, field.ErrorTypeInvalid),
			Entry("too long name", map[string]string{"foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters": "baz"}, field.ErrorTypeInvalid),
			Entry("too many parts", map[string]string{"foo/bar/baz": "null"}, field.ErrorTypeInvalid),
			Entry("invalid name", map[string]string{"foo/bar%baz": "null"}, field.ErrorTypeInvalid),

			// invalid values
			Entry("too long", map[string]string{"foo": "somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters"}, field.ErrorTypeInvalid),
			Entry("invalid", map[string]string{"foo": "no/slashes/allowed"}, field.ErrorTypeInvalid),
		)

		DescribeTable("reject when annotations are invalid",
			func(annotations map[string]string, expectType field.ErrorType) {
				maxSurge := intstr.FromInt32(1)
				maxUnavailable := intstr.FromInt32(0)
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
						Type: "large",
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
						Architecture: ptr.To("amd64"),
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Annotations:    annotations,
				}
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", map[string]string{"/foo": "bar"}, field.ErrorTypeInvalid),
			Entry("too long name", map[string]string{"foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters": "baz"}, field.ErrorTypeInvalid),
			Entry("too many parts", map[string]string{"foo/bar/baz": "null"}, field.ErrorTypeInvalid),
			Entry("invalid name", map[string]string{"foo/bar%baz": "null"}, field.ErrorTypeInvalid),

			// invalid value
			Entry("too long", map[string]string{"foo": strings.Repeat("a", 262142)}, field.ErrorTypeTooLong),
		)

		DescribeTable("reject when taints are invalid",
			func(taints []corev1.Taint, expectType field.ErrorType) {
				maxSurge := intstr.FromInt32(1)
				maxUnavailable := intstr.FromInt32(0)
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
						Type: "large",
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
						Architecture: ptr.To("amd64"),
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Taints:         taints,
				}
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// invalid keys
			Entry("missing prefix", []corev1.Taint{{Key: "/foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("missing prefix", []corev1.Taint{{Key: "/foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("too long name", []corev1.Taint{{Key: "foo/somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("too many parts", []corev1.Taint{{Key: "foo/bar/baz", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("invalid name", []corev1.Taint{{Key: "foo/bar%baz", Value: "bar", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),

			// invalid values
			Entry("too long", []corev1.Taint{{Key: "foo", Value: "somethingthatiswaylongerthanthelimitofthiswhichissixtythreecharacters", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),
			Entry("invalid", []corev1.Taint{{Key: "foo", Value: "no/slashes/allowed", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeInvalid),

			// invalid effects
			Entry("no effect", []corev1.Taint{{Key: "foo", Value: "bar"}}, field.ErrorTypeRequired),
			Entry("non-existing", []corev1.Taint{{Key: "foo", Value: "bar", Effect: "does-not-exist"}}, field.ErrorTypeNotSupported),

			// uniqueness by key/effect
			Entry("not unique", []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}, {Key: "foo", Value: "baz", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeDuplicate),
		)

		It("should reject if volume is undefined and data volumes are defined", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			dataVolumes := []core.DataVolume{{Name: "vol1-name", VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("volume"),
			}))))
		})

		It("should reject if data volume size does not match size regex", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			name := "vol1-name"
			vol := core.Volume{Name: &name, VolumeSize: "75Gi"}
			dataVolumes := []core.DataVolume{{Name: name, VolumeSize: "75Gi"}, {Name: "vol2-name", VolumeSize: "12MiB"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("dataVolumes[1].size"),
				"BadValue": Equal("12MiB"),
			}))))
		})

		It("should reject if data volume name is invalid", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			name1 := "vol1-name-is-too-long-for-test"
			name2 := "not%dns/1123"
			vol := core.Volume{Name: &name1, VolumeSize: "75Gi"}
			dataVolumes := []core.DataVolume{{VolumeSize: "75Gi"}, {Name: name1, VolumeSize: "75Gi"}, {Name: name2, VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("dataVolumes[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal("dataVolumes[1].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("dataVolumes[2].name"),
				})),
			))
		})

		It("should accept if kubeletDataVolumeName refers to defined data volume", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			name := "vol1-name"
			vol := core.Volume{Name: &name, VolumeSize: "75Gi"}
			dataVolumes := []core.DataVolume{{Name: name, VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:              &maxSurge,
				MaxUnavailable:        &maxUnavailable,
				Volume:                &vol,
				DataVolumes:           dataVolumes,
				KubeletDataVolumeName: &name,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf())
		})

		It("should reject if kubeletDataVolumeName refers to undefined data volume", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			name1 := "vol1-name"
			name2 := "vol2-name"
			name3 := "vol3-name"
			vol := core.Volume{Name: &name1, VolumeSize: "75Gi"}
			dataVolumes := []core.DataVolume{{Name: name1, VolumeSize: "75Gi"}, {Name: name2, VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:              &maxSurge,
				MaxUnavailable:        &maxUnavailable,
				Volume:                &vol,
				DataVolumes:           dataVolumes,
				KubeletDataVolumeName: &name3,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("kubeletDataVolumeName"),
				})),
			))
		})

		It("should reject if data volume names are duplicated", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			name1 := "vol1-name"
			name2 := "vol2-name"
			vol := core.Volume{Name: &name1, VolumeSize: "75Gi"}
			dataVolumes := []core.DataVolume{{Name: name1, VolumeSize: "75Gi"}, {Name: name1, VolumeSize: "75Gi"}, {Name: name2, VolumeSize: "75Gi"}, {Name: name1, VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, nil, false)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("dataVolumes[1].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("dataVolumes[3].name"),
				})),
			))
		})

		It("should reject if kubelet feature gates are invalid", func() {
			maxSurge := intstr.FromInt32(1)
			maxUnavailable := intstr.FromInt32(0)
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
					Architecture: ptr.To("amd64"),
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Kubernetes: &core.WorkerKubernetes{
					Kubelet: &core.KubeletConfig{
						KubernetesConfig: core.KubernetesConfig{
							FeatureGates: map[string]bool{
								"AnyVolumeDataSource": true,
								"Foo":                 true,
							},
						},
					},
				},
			}
			errList := ValidateWorker(worker, core.Kubernetes{Version: "1.27.3"}, nil, false)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("kubernetes.kubelet.featureGates.Foo"),
				})),
			))
		})

		DescribeTable("validate CRI name depending on the kubernetes version",
			func(name core.CRIName, matcher gomegatypes.GomegaMatcher) {
				worker := core.Worker{
					Name: "worker",
					CRI:  &core.CRI{Name: name},
				}

				errList := ValidateCRI(worker.CRI, field.NewPath("cri"))

				Expect(errList).To(matcher)
			},

			Entry("containerd is a valid CRI name", core.CRINameContainerD, BeEmpty()),
			Entry("docker is NOT a valid CRI name", core.CRIName("docker"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("cri.name"),
			})))),
			Entry("not valid CRI name", core.CRIName("other"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("cri.name"),
			})))),
		)

		DescribeTable("validate architecture",
			func(arch *string, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateArchitecture(arch, field.NewPath("architecture"))
				Expect(errList).To(matcher)
			},

			Entry("amd64 is a valid architecture name", ptr.To(v1beta1constants.ArchitectureAMD64), BeEmpty()),
			Entry("arm64 is a valid architecture name", ptr.To(v1beta1constants.ArchitectureARM64), BeEmpty()),
			Entry("foo is an invalid architecture name", ptr.To("foo"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("architecture"),
			})))),
		)

		It("validate that container runtime has a type", func() {
			worker := core.Worker{
				Name: "worker",
				CRI: &core.CRI{
					Name:              core.CRINameContainerD,
					ContainerRuntimes: []core.ContainerRuntime{{Type: "gVisor"}, {Type: ""}},
				},
			}

			errList := ValidateCRI(worker.CRI, field.NewPath("cri"))
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("cri.containerruntimes[1].type"),
				})),
			))
		})

		It("validate duplicate container runtime types", func() {
			worker := core.Worker{
				Name: "worker",
				CRI: &core.CRI{
					Name:              core.CRINameContainerD,
					ContainerRuntimes: []core.ContainerRuntime{{Type: "gVisor"}, {Type: "gVisor"}},
				},
			}

			errList := ValidateCRI(worker.CRI, field.NewPath("cri"))
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("cri.containerruntimes[1].type"),
				})),
			))
		})

		Describe("taint validation", func() {
			var (
				worker     core.Worker
				kubernetes core.Kubernetes
				fldPath    *field.Path
			)

			BeforeEach(func() {
				worker = core.Worker{
					Name: "worker1",
					Machine: core.Machine{
						Type: "xlarge",
					},
				}
				fldPath = field.NewPath("workers").Index(0)
			})

			It("should allow worker without taints", func() {
				errList := ValidateWorker(worker, kubernetes, fldPath, false)

				Expect(errList).To(BeEmpty())
			})

			It("should allow valid taints", func() {
				worker.Taints = []corev1.Taint{{
					Key:    "my-taint-1",
					Effect: "NoSchedule",
				}, {
					Key:    "my-taint-2",
					Effect: "NoExecute",
				}}

				errList := ValidateWorker(worker, kubernetes, fldPath, false)

				Expect(errList).To(BeEmpty())
			})

			It("should forbid reserved taint keys", func() {
				worker.Taints = []corev1.Taint{{
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: "NoSchedule",
				}, {
					Key:    "node.gardener.cloud/critical-components-not-ready",
					Effect: "NoExecute",
				}, {
					Key:    "allowed-key",
					Effect: "NoExecute",
				}}

				errList := ValidateWorker(worker, kubernetes, fldPath, false)

				Expect(errList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("workers[0].taints[0].key"),
						"Detail": Equal("taint key is reserved by gardener"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal("workers[0].taints[1].key"),
						"Detail": Equal("taint key is reserved by gardener"),
					})),
				))
			})
		})

		Describe("#ValidateCloudProfileReference", func() {
			var fldPath *field.Path

			BeforeEach(func() {
				fldPath = field.NewPath("cloudProfile")
			})

			It("should not allow using no cloudProfile reference", func() {
				errList := ValidateCloudProfileReference(nil, nil, fldPath)

				Expect(errList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("cloudProfile.name"),
						"Detail": Equal("must specify a cloud profile"),
					}))))
			})

			It("should not allow using an empty cloudProfile reference", func() {
				cloudProfileReference := &core.CloudProfileReference{
					Kind: "",
					Name: "",
				}

				errList := ValidateCloudProfileReference(cloudProfileReference, nil, fldPath)

				Expect(errList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeRequired),
						"Field":  Equal("cloudProfile.name"),
						"Detail": Equal("must specify a cloud profile"),
					}))))
			})

			It("should not allow using other Kind apart from CloudProfile and NamespacedCloudProfile", func() {
				cloudProfileReference := &core.CloudProfileReference{
					Kind: "Secret",
					Name: "my-profile",
				}

				errList := ValidateCloudProfileReference(cloudProfileReference, nil, fldPath)

				Expect(errList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("cloudProfile.kind"),
						"Detail": Equal("supported values: \"CloudProfile\", \"NamespacedCloudProfile\""),
					}))))
			})

			It("should allow creation using a CloudProfile", func() {
				cloudProfileReference := &core.CloudProfileReference{
					Kind: "CloudProfile",
					Name: "my-profile",
				}

				errList := ValidateCloudProfileReference(cloudProfileReference, nil, fldPath)

				Expect(errList).To(BeEmpty())
			})

			It("should allow creation using a NamespacedCloudProfile", func() {
				cloudProfileReference := &core.CloudProfileReference{
					Kind: "NamespacedCloudProfile",
					Name: "my-profile",
				}

				errList := ValidateCloudProfileReference(cloudProfileReference, nil, fldPath)

				Expect(errList).To(BeEmpty())
			})
		})

		Describe("update strategy validation", func() {
			var (
				worker             core.Worker
				fldPath            *field.Path
				testUpdateStrategy core.MachineUpdateStrategy = "testStrategy"
			)

			BeforeEach(func() {
				worker = core.Worker{
					Name: "worker-1",
					Machine: core.Machine{
						Type: "xlarge",
					},
				}

				fldPath = field.NewPath("workers").Index(0)
			})

			It("should fail if the update strategy is not supported", func() {
				worker.UpdateStrategy = ptr.To(testUpdateStrategy)

				Expect(ValidateWorker(worker, core.Kubernetes{}, fldPath, false)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeNotSupported),
						"Field":  Equal("workers[0].updateStrategy"),
						"Detail": Equal("supported values: \"AutoInPlaceUpdate\", \"AutoRollingUpdate\", \"ManualInPlaceUpdate\""),
					})),
				))
			})

			It("should succeed if the update strategy is supported", func() {
				worker.UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)

				Expect(ValidateWorker(worker, core.Kubernetes{}, fldPath, false)).To(BeEmpty())
			})
		})

		Describe("#ValidateInPlaceUpdateStrategyOnCreation", func() {
			var (
				shoot = &core.Shoot{
					Spec: core.ShootSpec{
						Provider: core.Provider{
							Workers: []core.Worker{
								{
									Name: "worker-1",
									Machine: core.Machine{
										Type: "xlarge",
									},
									UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
								},
								{
									Name: "worker-2",
									Machine: core.Machine{
										Type: "xlarge",
									},
									UpdateStrategy: ptr.To(core.ManualInPlaceUpdate),
								},
							},
						},
					},
				}
			)

			It("should not allow to set update strategy to AutoInPlaceUpdate/ManualInPlaceUpdate if feature gate is disabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, false))

				Expect(ValidateInPlaceUpdateStrategyOnCreation(shoot)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[0].updateStrategy"),
						"Detail": Equal("can not configure `AutoInPlaceUpdate` or `ManualInPlaceUpdate` update strategies when the `InPlaceNodeUpdates` feature gate is disabled."),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.provider.workers[1].updateStrategy"),
						"Detail": Equal("can not configure `AutoInPlaceUpdate` or `ManualInPlaceUpdate` update strategies when the `InPlaceNodeUpdates` feature gate is disabled."),
					})),
				))
			})

			It("should allow to set update strategy to AutoInPlaceUpdate/ManualInPlaceUpdate if feature gate is enabled", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))

				Expect(ValidateInPlaceUpdateStrategyOnCreation(shoot)).To(BeEmpty())
			})
		})

		Describe("machine controller manager settings validation", func() {
			var (
				worker  core.Worker
				fldPath *field.Path
			)

			BeforeEach(func() {
				worker = core.Worker{
					Name: "worker-1",
					Machine: core.Machine{
						Type: "xlarge",
					},
				}

				fldPath = field.NewPath("workers").Index(0)
			})

			It("should succeed if MachineControllerManagerSettings is nil", func() {
				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, fldPath, false)
				Expect(errList).To(BeEmpty())
			})

			It("should allow setting DisableHealthTimeout to false for update strategy AutoRollingUpdate", func() {
				worker.MachineControllerManagerSettings = &core.MachineControllerManagerSettings{
					DisableHealthTimeout: ptr.To(false),
				}

				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, fldPath, false)
				Expect(errList).To(BeEmpty())
			})

			It("should forbid setting DisableHealthTimeout to true for update strategy AutoRollingUpdate", func() {
				worker.MachineControllerManagerSettings = &core.MachineControllerManagerSettings{
					DisableHealthTimeout: ptr.To(true),
				}

				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, fldPath, false)
				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("workers[0].machineControllerManagerSettings.disableHealthTimeout"),
					"Detail": Equal("can only be set to true when the update strategy is `AutoInPlaceUpdate` or `ManualInPlaceUpdate`"),
				}))))
			})

			It("should allow setting DisableHealthTimeout to false for update strategy AutoInPlaceUpdate", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
				worker.UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
				worker.MachineControllerManagerSettings = &core.MachineControllerManagerSettings{
					DisableHealthTimeout: ptr.To(false),
				}

				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, fldPath, false)
				Expect(errList).To(BeEmpty())
			})

			It("should allow setting DisableHealthTimeout to true for update strategy AutoInPlaceUpdate", func() {
				DeferCleanup(test.WithFeatureGate(features.DefaultFeatureGate, features.InPlaceNodeUpdates, true))
				worker.UpdateStrategy = ptr.To(core.AutoInPlaceUpdate)
				worker.MachineControllerManagerSettings = &core.MachineControllerManagerSettings{
					DisableHealthTimeout: ptr.To(true),
				}

				errList := ValidateWorker(worker, core.Kubernetes{Version: ""}, fldPath, false)
				Expect(errList).To(BeEmpty())
			})
		})
	})

	Describe("#ValidateWorkers", func() {
		It("should succeed checking workers", func() {
			workers := []core.Worker{
				{Name: "worker1"},
				{Name: "worker2"},
			}

			Expect(ValidateWorkers(workers, nil)).To(BeEmpty())
		})

		It("should fail because worker name is duplicated", func() {
			workers := []core.Worker{
				{Name: "worker1"},
				{Name: "worker2"},
				{Name: "worker1"},
			}

			Expect(ValidateWorkers(workers, field.NewPath("workers"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("workers[2].name"),
				})),
			))
		})
	})

	Describe("#ValidateSystemComponentWorkers", func() {
		const (
			zero = iota
			one
			two
			three
		)

		DescribeTable("validate that at least one active worker pool is configured",
			func(min1, max1, min2, max2 int, matcher gomegatypes.GomegaMatcher) {
				systemComponents := &core.WorkerSystemComponents{
					Allow: true,
				}
				workers := []core.Worker{
					{
						Name:             "one",
						Minimum:          int32(min1),
						Maximum:          int32(max1),
						SystemComponents: systemComponents,
					},
					{
						Name:             "two",
						Minimum:          int32(min2),
						Maximum:          int32(max2),
						SystemComponents: systemComponents,
					},
				}

				Expect(ValidateSystemComponentWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry("at least one worker pool min>0, max>0", zero, zero, one, one, BeEmpty()),
			Entry("all worker pools min=max=0", zero, zero, zero, zero, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("workers"),
					"Detail": ContainSubstring("at least one active (workers[i].maximum > 0) worker pool with systemComponents.allow=true needed"),
				})),
			)),
		)

		DescribeTable("validate that at least one worker pool is able to host system components",
			func(min1, max1, min2, max2 int, allowSystemComponents1, allowSystemComponents2 bool, matcher gomegatypes.GomegaMatcher) {
				workers := []core.Worker{
					{
						Name:    "one-active",
						Minimum: int32(min1),
						Maximum: int32(max1),
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents1,
						},
					},
					{
						Name:    "two-active",
						Minimum: int32(min2),
						Maximum: int32(max2),
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents2,
						},
					},
				}

				Expect(ValidateSystemComponentWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry("all worker pools min=max=0", zero, zero, zero, zero, true, true, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("workers"),
					"Detail": ContainSubstring("at least one active (workers[i].maximum > 0) worker pool with systemComponents.allow=true needed"),
				})),
			)),
			Entry("at least one worker pool allows system components", zero, zero, one, one, true, true, BeEmpty()),
		)

		DescribeTable("validate maximum node count",
			func(max1, max2 int, allowSystemComponents1, allowSystemComponents2 bool, zones1, zones2 []string, matcher gomegatypes.GomegaMatcher) {
				workers := []core.Worker{
					{
						Name:    "one-active",
						Minimum: one,
						Maximum: int32(max1),
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents1,
						},
						Zones: zones1,
					},
					{
						Name:    "two-active",
						Minimum: one,
						Maximum: int32(max2),
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents2,
						},
						Zones: zones2,
					},
				}

				Expect(ValidateSystemComponentWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry("maximum == len(zones)", three, one, true, false, []string{"1", "2", "3"}, []string{"1"}, BeEmpty()),
			Entry("maximum == len(zones) with multiple system component worker pools and smaller group first", one, three, true, true, []string{"1", "2", "3"}, []string{"1", "2", "3"}, BeEmpty()),
			Entry("maximum == len(zones) with multiple system component worker pools and smaller group last", three, one, true, true, []string{"1", "2", "3"}, []string{"1", "2", "3"}, BeEmpty()),
			Entry("maximum < len(zones)", two, one, true, false, []string{"1", "2", "3"}, []string{"1"}, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].maximum"),
				})),
			)),
			Entry("maximum < len(zones) with multiple system component worker pools in different zones", two, one, true, true, []string{"1", "2", "3"}, []string{"3", "4", "5"}, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].maximum"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[1].maximum"),
				})),
			)),
			Entry("maximum < len(zones) with multiple system component worker pools in same zones", two, one, true, false, []string{"1", "2", "3"}, []string{"3", "1", "2"}, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("workers[0].maximum"),
				})),
			)),
		)
	})

	Describe("#ValidateKubeletConfiguration", func() {
		validResourceQuantityValueMi := "100Mi"
		validResourceQuantityValueKi := "100"
		invalidResourceQuantityValue := "-100Mi"
		validPercentValue := "5%"
		invalidPercentValueLow := "-5%"
		invalidPercentValueHigh := "110%"
		invalidValue := "5X"

		DescribeTable("StreamingConnectionIdleTimeout",
			func(streamingConnectionIdleTimeout *metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					StreamingConnectionIdleTimeout: streamingConnectionIdleTimeout,
				}
				errList := ValidateKubeletConfig(kubeletConfig, "", nil)
				Expect(errList).To(matcher)
			},

			Entry("should allow empty streamingConnectionIdleTimeout", nil, BeEmpty()),
			Entry("should allow streamingConnectionIdleTimeout to be in the 30s - 4h range", &metav1.Duration{Duration: time.Minute * 5}, BeEmpty()),
			Entry("should not allow streamingConnectionIdleTimeout to be with default metav1.Duration value", &metav1.Duration{}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("streamingConnectionIdleTimeout"),
				"Detail": Equal("value must be between 30s and 4h"),
			})))),
			Entry("should not allow streamingConnectionIdleTimeout to be lower than 30s", &metav1.Duration{Duration: time.Second}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("streamingConnectionIdleTimeout"),
				"Detail": Equal("value must be between 30s and 4h"),
			})))),
			Entry("should not allow streamingConnectionIdleTimeout to be greater than 4h", &metav1.Duration{Duration: time.Minute * 241}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":   Equal(field.ErrorTypeInvalid),
				"Field":  Equal("streamingConnectionIdleTimeout"),
				"Detail": Equal("value must be between 30s and 4h"),
			})))),
		)

		DescribeTable("MemorySwap",
			func(allowSwap bool, swapBehavior *string, version string, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{}
				if swapBehavior != nil {
					kubeletConfig.MemorySwap = &core.MemorySwapConfiguration{SwapBehavior: (*core.SwapBehavior)(swapBehavior)}
				}

				kubeletConfig.FailSwapOn = ptr.To(true)

				if allowSwap {
					kubeletConfig.FeatureGates = map[string]bool{"NodeSwap": true}
					kubeletConfig.FailSwapOn = ptr.To(false)
				}

				errList := ValidateKubeletConfig(kubeletConfig, version, nil)
				Expect(errList).To(matcher)
			},

			Entry("should allow empty memory swap", false, nil, "1.26", BeEmpty()),
			Entry("should allow empty memory swap - NodeSwap set and FailSwap=false", true, nil, "1.26", BeEmpty()),
			Entry("should allow LimitedSwap behavior", true, ptr.To("LimitedSwap"), "1.26", BeEmpty()),
			Entry("should allow UnlimitedSwap behavior for Kubernetes versions < 1.30", true, ptr.To("UnlimitedSwap"), "1.29", BeEmpty()),
			Entry("should forbid UnlimitedSwap behavior for Kubernetes versions >= 1.30", true, ptr.To("UnlimitedSwap"), "1.30", ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("memorySwap.swapBehavior"),
					"Detail": Equal("supported values: \"NoSwap\", \"LimitedSwap\""),
				})),
			)),
			Entry("should forbid NoSwap behavior for Kubernetes versions < 1.30", true, ptr.To("NoSwap"), "1.29", ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeNotSupported),
					"Field":  Equal("memorySwap.swapBehavior"),
					"Detail": Equal("supported values: \"LimitedSwap\", \"UnlimitedSwap\""),
				})),
			)),
			Entry("should allow NoSwap behavior for Kubernetes versions >= 1.30", true, ptr.To("NoSwap"), "1.30", BeEmpty()),
			Entry("should forbid configuration of swap behaviour if either the feature gate NodeSwap is not set or FailSwap=true", false, ptr.To("LimitedSwap"), "1.26", ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("memorySwap"),
					"Detail": Equal("configuring swap behaviour is not available when the kubelet is configured with 'FailSwapOn=true'"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("memorySwap"),
					"Detail": Equal("configuring swap behaviour is not available when kubelet's 'NodeSwap' feature gate is not set"),
				}))),
			),
		)

		DescribeTable("EvictionHard & EvictionSoft",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree string, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					EvictionHard: &core.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
					EvictionSoft: &core.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantityValueMi, validResourceQuantityValueKi, validPercentValue, validPercentValue, validPercentValue, BeEmpty()),
			Entry("only allow resource.Quantity or percent value for any value", invalidValue, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow negative resource.Quantity", invalidResourceQuantityValue, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow negative percentages", invalidPercentValueLow, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
			Entry("do not allow percentages > 100", invalidPercentValueHigh, validPercentValue, validPercentValue, validPercentValue, validPercentValue, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionHard.memoryAvailable").String()),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoft.memoryAvailable").String()),
				})))),
		)

		Describe("pod pids limits", func() {
			It("should ensure pod pids limits are non-negative", func() {
				var podPIDsLimit int64 = -1
				kubeletConfig := core.KubeletConfig{
					PodPIDsLimit: &podPIDsLimit,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("podPIDsLimit"),
				}))))
			})

			It("should ensure pod pids limits are at least 100", func() {
				var podPIDsLimit int64 = 99
				kubeletConfig := core.KubeletConfig{
					PodPIDsLimit: &podPIDsLimit,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("podPIDsLimit"),
				}))))
			})

			It("should allow pod pids limits of at least 100", func() {
				var podPIDsLimit int64 = 100
				kubeletConfig := core.KubeletConfig{
					PodPIDsLimit: &podPIDsLimit,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(BeEmpty())
			})
		})

		validResourceQuantity := resource.MustParse(validResourceQuantityValueMi)
		invalidResourceQuantity := resource.MustParse(invalidResourceQuantityValue)

		DescribeTable("EvictionMinimumReclaim",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree resource.Quantity, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					EvictionMinimumReclaim: &core.KubeletConfigEvictionMinimumReclaim{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, BeEmpty()),
			Entry("only allow positive resource.Quantity for any value", invalidResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("evictionMinimumReclaim.memoryAvailable").String()),
			})))),
		)

		validDuration := metav1.Duration{Duration: 2 * time.Minute}
		invalidDuration := metav1.Duration{Duration: -2 * time.Minute}
		DescribeTable("KubeletConfigEvictionSoftGracePeriod",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					EvictionSoftGracePeriod: &core.KubeletConfigEvictionSoftGracePeriod{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, validDuration, validDuration, validDuration, validDuration, BeEmpty()),
			Entry("only allow positive Duration for any value", invalidDuration, validDuration, validDuration, validDuration, validDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoftGracePeriod.memoryAvailable").String()),
				})))),
		)

		DescribeTable("EvictionPressureTransitionPeriod",
			func(evictionPressureTransitionPeriod metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					EvictionPressureTransitionPeriod: &evictionPressureTransitionPeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, BeEmpty()),
			Entry("only allow positive Duration", invalidDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionPressureTransitionPeriod").String()),
				})),
			)),
		)

		Context("reserved", func() {
			DescribeTable("KubeReserved",
				func(cpu, memory, ephemeralStorage, pid *resource.Quantity, matcher gomegatypes.GomegaMatcher) {
					kubeletConfig := core.KubeletConfig{
						KubeReserved: &core.KubeletConfigReserved{
							CPU:              cpu,
							Memory:           memory,
							EphemeralStorage: ephemeralStorage,
							PID:              pid,
						},
					}
					Expect(ValidateKubeletConfig(kubeletConfig, "", nil)).To(matcher)
				},

				Entry("valid configuration (cpu)", &validResourceQuantity, nil, nil, nil, BeEmpty()),
				Entry("valid configuration (memory)", nil, &validResourceQuantity, nil, nil, BeEmpty()),
				Entry("valid configuration (storage)", nil, nil, &validResourceQuantity, nil, BeEmpty()),
				Entry("valid configuration (pid)", nil, nil, nil, &validResourceQuantity, BeEmpty()),
				Entry("valid configuration (all)", &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, BeEmpty()),
				Entry("only allow positive resource.Quantity for any value", &invalidResourceQuantity, &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("kubeReserved.cpu").String()),
				})))),
			)

			DescribeTable("SystemReserved",
				func(cpu, memory, ephemeralStorage, pid *resource.Quantity, k8sVersion string, matcher gomegatypes.GomegaMatcher) {
					kubeletConfig := core.KubeletConfig{
						SystemReserved: &core.KubeletConfigReserved{
							CPU:              cpu,
							Memory:           memory,
							EphemeralStorage: ephemeralStorage,
							PID:              pid,
						},
					}
					Expect(ValidateKubeletConfig(kubeletConfig, k8sVersion, nil)).To(matcher)
				},

				Entry("valid configuration (cpu)", &validResourceQuantity, nil, nil, nil, "1.30.0", BeEmpty()),
				Entry("valid configuration (memory)", nil, &validResourceQuantity, nil, nil, "1.30.0", BeEmpty()),
				Entry("valid configuration (storage)", nil, nil, &validResourceQuantity, nil, "1.30.0", BeEmpty()),
				Entry("valid configuration (pid)", nil, nil, nil, &validResourceQuantity, "1.30.0", BeEmpty()),
				Entry("valid configuration (all)", &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, "1.30.0", BeEmpty()),
				Entry("only allow positive resource.Quantity for any value", &invalidResourceQuantity, &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, "1.30.0", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("systemReserved.cpu").String()),
				})))),
				Entry("forbid string from kubernetes version 1.31", &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, &validResourceQuantity, "1.31.0", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("systemReserved").String()),
				})))),
			)
		})

		DescribeTable("ImageGCHighThresholdPercent",
			func(imageGCHighThresholdPercent int, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					ImageGCHighThresholdPercent: ptr.To(int32(imageGCHighThresholdPercent)),
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("0 <= value <= 100", 1, BeEmpty()),
			Entry("value < 0", -1, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageGCHighThresholdPercent").String()),
				})),
			)),
			Entry("value > 100", 101, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageGCHighThresholdPercent").String()),
				})),
			)),
		)

		DescribeTable("ImageGCLowThresholdPercent",
			func(imageGCLowThresholdPercent int, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					ImageGCLowThresholdPercent: ptr.To(int32(imageGCLowThresholdPercent)),
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("0 <= value <= 100", 1, BeEmpty()),
			Entry("value < 0", -1, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageGCLowThresholdPercent").String()),
				})),
			)),
			Entry("value > 100", 101, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageGCLowThresholdPercent").String()),
				})),
			)),
		)

		It("should prevent that imageGCLowThresholdPercent is not less than imageGCHighThresholdPercent", func() {
			kubeletConfig := core.KubeletConfig{
				ImageGCLowThresholdPercent:  ptr.To[int32](2),
				ImageGCHighThresholdPercent: ptr.To[int32](1),
			}

			errList := ValidateKubeletConfig(kubeletConfig, "", nil)

			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(field.NewPath("imageGCLowThresholdPercent").String()),
				})),
			))
		})

		DescribeTable("ImageMinimumGCAge",
			func(imageMinimumGCAge metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					ImageMinimumGCAge: &imageMinimumGCAge,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("should allow nil value", nil, BeEmpty()),
			Entry("should allow positive duration", metav1.Duration{Duration: time.Minute}, BeEmpty()),
			Entry("should not allow negative duration", metav1.Duration{Duration: -time.Minute}, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageMinimumGCAge").String()),
				})),
			)),
		)

		DescribeTable("ImageMaximumGCAge",
			func(imageMaximumGCAge *metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					ImageMaximumGCAge: imageMaximumGCAge,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("should allow nil value", nil, BeEmpty()),
			Entry("should allow positive duration", &metav1.Duration{Duration: time.Minute}, BeEmpty()),
			Entry("should not allow negative duration", &metav1.Duration{Duration: -time.Minute}, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imageMaximumGCAge").String()),
				})),
			)),
		)

		DescribeTable("EvictionMaxPodGracePeriod",
			func(evictionMaxPodGracePeriod int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					EvictionMaxPodGracePeriod: &evictionMaxPodGracePeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(90), BeEmpty()),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionMaxPodGracePeriod").String()),
				})),
			)),
		)

		DescribeTable("MaxPods",
			func(maxPods int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					MaxPods: &maxPods,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(110), BeEmpty()),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("maxPods").String()),
				})),
			)),
		)

		Describe("registryPullQPS, registryBurst", func() {
			It("should allow positive values", func() {
				Expect(ValidateKubeletConfig(core.KubeletConfig{
					RegistryPullQPS: ptr.To[int32](10),
					RegistryBurst:   ptr.To[int32](20),
				}, "", nil)).To(BeEmpty())
			})

			It("should not allow negative values", func() {
				Expect(ValidateKubeletConfig(core.KubeletConfig{
					RegistryPullQPS: ptr.To(int32(-10)),
					RegistryBurst:   ptr.To(int32(-20)),
				}, "", nil)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("registryPullQPS").String()),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("registryBurst").String()),
					})),
				))
			})
		})

		Describe("#ContainerLog", func() {
			It("should not accept invalid  containerLogMaxFiles", func() {
				maxSize := resource.MustParse("100Mi")
				kubeletConfig := core.KubeletConfig{
					ContainerLogMaxFiles: ptr.To[int32](1),
					ContainerLogMaxSize:  &maxSize,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("containerLogMaxFiles").String()),
					})),
				))
			})

			It("should accept valid containerLogMaxFiles and containerLogMaxSize", func() {
				maxSize := resource.MustParse("100Mi")
				kubeletConfig := core.KubeletConfig{
					ContainerLogMaxFiles: ptr.To[int32](5),
					ContainerLogMaxSize:  &maxSize,
				}

				errList := ValidateKubeletConfig(kubeletConfig, "", nil)

				Expect(errList).To(BeEmpty())
			})
		})

		Describe("maxParallelImagePulls", func() {
			It("should allow positive values", func() {
				Expect(ValidateKubeletConfig(core.KubeletConfig{
					MaxParallelImagePulls: ptr.To[int32](10),
				}, "", nil)).To(BeEmpty())
			})

			It("should not allow negative values", func() {
				Expect(ValidateKubeletConfig(core.KubeletConfig{
					MaxParallelImagePulls: ptr.To[int32](-10),
				}, "", nil)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("maxParallelImagePulls").String()),
					})),
				))
			})

			It("should not allow maxParallelImagePulls > 1 when serializeImagePulls is set to true", func() {
				Expect(ValidateKubeletConfig(core.KubeletConfig{
					MaxParallelImagePulls: ptr.To[int32](10),
					SerializeImagePulls:   ptr.To(true),
				}, "", nil)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeForbidden),
						"Field":  Equal(field.NewPath("maxParallelImagePulls").String()),
						"Detail": Equal("maxParallelImagePulls cannot be larger than 1 when serializeImagePulls is set to true"),
					})),
				))
			})
		})
	})

	Describe("#ValidateHibernationSchedules", func() {
		DescribeTable("validate hibernation schedules",
			func(schedules []core.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationSchedules(schedules, nil)).To(matcher)
			},
			Entry("valid schedules", []core.HibernationSchedule{{Start: ptr.To("1 * * * *"), End: ptr.To("2 * * * *")}}, BeEmpty()),
			Entry("nil schedules", nil, BeEmpty()),
			Entry("duplicate start and end value in same schedule",
				[]core.HibernationSchedule{{Start: ptr.To("* * * * *"), End: ptr.To("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("duplicate start and end value in different schedules",
				[]core.HibernationSchedule{{Start: ptr.To("1 * * * *"), End: ptr.To("2 * * * *")}, {Start: ptr.To("1 * * * *"), End: ptr.To("3 * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("invalid schedule",
				[]core.HibernationSchedule{{Start: ptr.To("foo"), End: ptr.To("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeInvalid),
				})))),
		)
	})

	Describe("#ValidateHibernationCronSpec", func() {
		DescribeTable("validate cron spec",
			func(seenSpecs sets.Set[string], spec string, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationCronSpec(seenSpecs, spec, nil)).To(matcher)
			},
			Entry("invalid spec", sets.New[string](), "foo", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeInvalid),
			})))),
			Entry("duplicate spec", sets.New("* * * * *"), "* * * * *", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeDuplicate),
			})))),
		)

		It("should add the inspected cron spec to the set if there were no issues", func() {
			var (
				s    = sets.New[string]()
				spec = "* * * * *"
			)
			Expect(ValidateHibernationCronSpec(s, spec, nil)).To(BeEmpty())
			Expect(s.Has(spec)).To(BeTrue())
		})

		It("should not add the inspected cron spec to the set if there were issues", func() {
			var (
				s    = sets.New[string]()
				spec = "foo"
			)
			Expect(ValidateHibernationCronSpec(s, spec, nil)).NotTo(BeEmpty())
			Expect(s.Has(spec)).To(BeFalse())
		})
	})

	Describe("#ValidateHibernationScheduleLocation", func() {
		DescribeTable("validate hibernation schedule location",
			func(location string, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationScheduleLocation(location, nil)).To(matcher)
			},
			Entry("utc location", "UTC", BeEmpty()),
			Entry("empty location -> utc", "", BeEmpty()),
			Entry("invalid location", "should not exist", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeInvalid),
			})))),
		)
	})

	Describe("#ValidateHibernationSchedule", func() {
		DescribeTable("validate schedule",
			func(seenSpecs sets.Set[string], schedule *core.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateHibernationSchedule(seenSpecs, schedule, nil)
				Expect(errList).To(matcher)
			},

			Entry("valid schedule", sets.New[string](), &core.HibernationSchedule{Start: ptr.To("1 * * * *"), End: ptr.To("2 * * * *")}, BeEmpty()),
			Entry("invalid start value", sets.New[string](), &core.HibernationSchedule{Start: ptr.To(""), End: ptr.To("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("start").String()),
			})))),
			Entry("invalid end value", sets.New[string](), &core.HibernationSchedule{Start: ptr.To("* * * * *"), End: ptr.To("")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("invalid location", sets.New[string](), &core.HibernationSchedule{Start: ptr.To("1 * * * *"), End: ptr.To("2 * * * *"), Location: ptr.To("foo")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("location").String()),
			})))),
			Entry("equal start and end value", sets.New[string](), &core.HibernationSchedule{Start: ptr.To("* * * * *"), End: ptr.To("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("nil start", sets.New[string](), &core.HibernationSchedule{End: ptr.To("* * * * *")}, BeEmpty()),
			Entry("nil end", sets.New[string](), &core.HibernationSchedule{Start: ptr.To("* * * * *")}, BeEmpty()),
			Entry("start and end nil", sets.New[string](), &core.HibernationSchedule{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeRequired),
				})))),
			Entry("invalid start and end value", sets.New[string](), &core.HibernationSchedule{Start: ptr.To(""), End: ptr.To("")},
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("start").String()),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(field.NewPath("end").String()),
					})),
				)),
		)
	})

	Describe("#ValidateFinalizersOnCreation", func() {
		It("should return error if the finalizers contain forbidden finalizers", func() {
			finalizers := []string{
				"some-finalizer",
				"gardener.cloud/reference-protection",
				"gardener",
				"random",
			}

			Expect(ValidateFinalizersOnCreation(finalizers, field.NewPath("metadata", "finalizers"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("metadata.finalizers[1]"),
					"Detail": ContainSubstring("finalizer %q cannot be added on creation", "gardener.cloud/reference-protection"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("metadata.finalizers[2]"),
					"Detail": ContainSubstring("finalizer %q cannot be added on creation", "gardener"),
				})),
			))
		})
	})

	Describe("#ValidateOIDCIssuerURL", func() {
		DescribeTable("test valid and invalid issuer URLs", func(issuerURL string, detail string) {
			errorList := ValidateOIDCIssuerURL(issuerURL, field.NewPath("issuerURL"))

			if detail == "" {
				Expect(errorList).To(BeEmpty())
			} else {
				Expect(errorList).To(ConsistOf(PointTo(
					MatchFields(IgnoreExtras, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("issuerURL"),
						"Detail": Equal(detail),
					}),
				)))
			}

		},
			Entry("should succeed if issuer URL is valid", "https://issuer.com/auth", ""),
			Entry("should fail if issuerURL URL scheme is not https", "http://issuer.com", "must have https scheme"),
			Entry("should fail if issuerURL URL contains a fragment", "https://issuer.com#fragment", "must not contain a fragment"),
			Entry("should fail if issuerURL URL contains a username and password", "https://user:pass@issuer.com", "must not contain a username or password"),
			Entry("should fail if issuerURL URL contains a query", "https://issuer.com?query=value", "must not contain a query"),
		)
	})

	Describe("#ValidateControlPlaneAutoscaling", func() {
		It("should succeed for valid resources", func() {
			autoscaling := &core.ControlPlaneAutoscaling{
				MinAllowed: map[corev1.ResourceName]resource.Quantity{
					"cpu":    {},
					"memory": {},
				},
			}

			Expect(ValidateControlPlaneAutoscaling(autoscaling, nil, nil)).To(BeEmpty())
		})

		It("should fail when minAllowed is not specified", func() {
			autoscaling := &core.ControlPlaneAutoscaling{}

			Expect(ValidateControlPlaneAutoscaling(autoscaling, nil, field.NewPath("autoscaling"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("autoscaling.minAllowed"),
				})),
			))
		})

		It("should fail for unsupported resources", func() {
			autoscaling := &core.ControlPlaneAutoscaling{
				MinAllowed: map[corev1.ResourceName]resource.Quantity{
					"storage": {},
				},
			}

			Expect(ValidateControlPlaneAutoscaling(autoscaling, nil, field.NewPath("autoscaling"))).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("autoscaling.minAllowed.storage"),
				})),
			))
		})

		When("minimum required values are configured", func() {
			var minRequired corev1.ResourceList

			BeforeEach(func() {
				minRequired = corev1.ResourceList{
					"cpu":    resource.MustParse("10m"),
					"memory": resource.MustParse("50Mi"),
				}
			})

			It("should succeed if value match", func() {
				autoscaling := &core.ControlPlaneAutoscaling{
					MinAllowed: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("10m"),
						"memory": resource.MustParse("50Mi"),
					},
				}

				Expect(ValidateControlPlaneAutoscaling(autoscaling, minRequired, nil)).To(BeEmpty())
			})

			It("should fail for values falling below minimum", func() {
				autoscaling := &core.ControlPlaneAutoscaling{
					MinAllowed: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("9m"),
						"memory": resource.MustParse("50Mi"),
					},
				}

				Expect(ValidateControlPlaneAutoscaling(autoscaling, minRequired, field.NewPath("autoscaling"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("autoscaling.minAllowed.cpu"),
						"BadValue": Equal(resource.MustParse("9m")),
					})),
				))
			})

			It("should fail for negative values", func() {
				autoscaling := &core.ControlPlaneAutoscaling{
					MinAllowed: map[corev1.ResourceName]resource.Quantity{
						"cpu":    resource.MustParse("-100m"),
						"memory": resource.MustParse("50Mi"),
					},
				}

				Expect(ValidateControlPlaneAutoscaling(autoscaling, nil, field.NewPath("autoscaling"))).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":     Equal(field.ErrorTypeInvalid),
						"Field":    Equal("autoscaling.minAllowed.cpu"),
						"BadValue": Equal("-100m"),
					})),
				))
			})
		})
	})

	Describe("#ValidateInPlaceUpdates", func() {
		var newShoot, oldShoot *core.Shoot

		BeforeEach(func() {
			oldShoot = &core.Shoot{
				Spec: core.ShootSpec{
					Kubernetes: core.Kubernetes{
						Version: "1.31.0",
						Kubelet: &core.KubeletConfig{
							EvictionHard: &core.KubeletConfigEviction{
								MemoryAvailable:  ptr.To("100Mi"),
								ImageFSAvailable: ptr.To("100Mi"),
							},
						},
					},
					Provider: core.Provider{
						Workers: []core.Worker{
							{
								Name:           "worker-1",
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
								Kubernetes: &core.WorkerKubernetes{
									Version: ptr.To("1.30.0"),
									Kubelet: &core.KubeletConfig{
										EvictionHard: &core.KubeletConfigEviction{
											MemoryAvailable:  ptr.To("200Mi"),
											ImageFSAvailable: ptr.To("200Mi"),
										},
									},
								},
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Version: "1.5.0",
									},
								},
							},
							{
								Name:           "worker-2",
								UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
								Machine: core.Machine{
									Image: &core.ShootMachineImage{
										Version: "1.5.0",
									},
								},
							},
						},
					},
				},
				Status: core.ShootStatus{
					InPlaceUpdates: &core.InPlaceUpdatesStatus{
						PendingWorkerUpdates: &core.PendingWorkerUpdates{
							AutoInPlaceUpdate:   []string{"worker-1"},
							ManualInPlaceUpdate: []string{"worker-2"},
						},
					},
				},
			}

			newShoot = oldShoot.DeepCopy()

			newShoot.Spec.Provider.Workers[0].Machine.Image.Version = "1.5.1"
			newShoot.Spec.Provider.Workers[1].Machine.Image.Version = "1.5.1"
		})

		It("should return no errors if force-update annotation is present", func() {
			newShoot.Annotations = map[string]string{
				v1beta1constants.GardenerOperation: v1beta1constants.ShootOperationForceInPlaceUpdate,
			}

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(BeEmpty())
		})

		It("should return no errors if there are no pending in-place updates", func() {
			newShoot.Status.InPlaceUpdates.PendingWorkerUpdates = nil

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(BeEmpty())
		})

		It("should return an error if the Kubernetes version is invalid", func() {
			oldShoot.Spec.Kubernetes.Version = "invalid-version"

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.version"),
					"Detail": Equal("failed to parse old control plane kubernetes version: Invalid Semantic Version"),
				})),
			))
		})

		It("should return an error if the new Kubernetes version is invalid", func() {
			newShoot.Spec.Kubernetes.Version = "invalid-version"

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.kubernetes.version"),
					"Detail": Equal("failed to parse new control plane kubernetes version: Invalid Semantic Version"),
				})),
			))
		})

		It("should return an error if the old worker Kubernetes version is invalid", func() {
			oldShoot.Spec.Provider.Workers[0].Kubernetes.Version = ptr.To("invalid-version")
			newShoot.Spec.Provider.Workers[1].Machine.Image.Version = oldShoot.Spec.Provider.Workers[1].Machine.Image.Version

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0]"),
					"Detail": ContainSubstring("failed to calculate effective kubernetes version for old worker"),
				})),
			))
		})

		It("should return an error if the new worker Kubernetes version is invalid", func() {
			newShoot.Spec.Provider.Workers[0].Kubernetes.Version = ptr.To("invalid-version")
			newShoot.Spec.Provider.Workers[1].Machine.Image.Version = oldShoot.Spec.Provider.Workers[1].Machine.Image.Version

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0]"),
					"Detail": ContainSubstring("failed to calculate effective kubernetes version for new worker"),
				})),
			))
		})

		It("should return an error if the worker pool is undergoing an in-place update and changes are made", func() {
			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[0]"),
					"Detail": Equal("the worker pool \"worker-1\" is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with 'gardener.cloud/operation=force-in-place-update'"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[1]"),
					"Detail": Equal("the worker pool \"worker-2\" is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with 'gardener.cloud/operation=force-in-place-update'"),
				})),
			))
		})

		It("should return an error if the worker pool is undergoing an in-place update and changes are made to the Shoot kubelet config", func() {
			newShoot.Spec.Kubernetes.Kubelet.EvictionHard.MemoryAvailable = ptr.To("150Mi")
			newShoot.Spec.Provider.Workers[0].Machine.Image.Version = oldShoot.Spec.Provider.Workers[0].Machine.Image.Version

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[1]"),
					"Detail": Equal("the worker pool \"worker-2\" is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with 'gardener.cloud/operation=force-in-place-update'"),
				})),
			))
		})

		It("should return an error if the worker pool is undergoing an in-place update and changes are made to the Shoot kubernetes version", func() {
			newShoot.Spec.Kubernetes.Version = "1.32.0"
			newShoot.Spec.Provider.Workers[0].Machine.Image.Version = oldShoot.Spec.Provider.Workers[0].Machine.Image.Version

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[1]"),
					"Detail": Equal("the worker pool \"worker-2\" is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with 'gardener.cloud/operation=force-in-place-update'"),
				})),
			))
		})

		It("should return no errors if the worker pool is not undergoing an in-place update", func() {
			newShoot.Spec.Provider.Workers[0] = oldShoot.Spec.Provider.Workers[0]
			newShoot.Spec.Provider.Workers[1] = oldShoot.Spec.Provider.Workers[1]

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(BeEmpty())
		})

		It("should return no errors if the worker pool is newly added", func() {
			oldShoot.Spec.Provider.Workers = []core.Worker{
				{
					Name:           "worker-2",
					UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Version: "1.5.0",
						},
					},
				},
				{
					Name:           "worker-3",
					UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Version: "1.5.0",
						},
					},
				},
			}

			newShoot.Spec.Provider.Workers = []core.Worker{
				{
					Name:           "worker-1",
					UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Version: "1.5.1",
						},
					},
				},
				{
					Name:           "worker-3",
					UpdateStrategy: ptr.To(core.AutoInPlaceUpdate),
					Machine: core.Machine{
						Image: &core.ShootMachineImage{
							Version: "1.5.1",
						},
					},
				},
			}

			newShoot.Status.InPlaceUpdates.PendingWorkerUpdates.AutoInPlaceUpdate = []string{"worker-1", "worker-2", "worker-3"}

			Expect(ValidateInPlaceUpdates(newShoot, oldShoot)).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":   Equal(field.ErrorTypeForbidden),
					"Field":  Equal("spec.provider.workers[1]"),
					"Detail": Equal("the worker pool \"worker-3\" is currently undergoing an in-place update. No changes are allowed to the worker pool, the Shoot Kubernetes version, or the Shoot kubelet configuration. You can force an update with annotating the Shoot with 'gardener.cloud/operation=force-in-place-update'"),
				})),
			))
		})
	})
})

func prepareShootForUpdate(shoot *core.Shoot) *core.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
