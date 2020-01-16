// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener/pkg/apis/garden"
	. "github.com/gardener/gardener/pkg/apis/garden/validation"
	. "github.com/gardener/gardener/pkg/utils/validation/gomega"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"k8s.io/apimachinery/pkg/util/sets"
)

var _ = Describe("Shoot Validation Tests", func() {
	Describe("#ValidateShoot, #ValidateShootUpdate", func() {
		var (
			shoot *garden.Shoot

			domain          = "my-cluster.example.com"
			dnsProviderType = "some-provider"

			nodeCIDR    = "10.250.0.0/16"
			podCIDR     = "100.96.0.0/11"
			serviceCIDR = "100.64.0.0/13"
			invalidCIDR = "invalid-cidr"
			vpcCIDR     = "10.0.0.0/8"
			addon       = garden.Addon{
				Enabled: true,
			}
			k8sNetworks = garden.K8SNetworks{
				Nodes:    &nodeCIDR,
				Pods:     &podCIDR,
				Services: &serviceCIDR,
			}
			invalidK8sNetworks = garden.K8SNetworks{
				Nodes:    &invalidCIDR,
				Pods:     &invalidCIDR,
				Services: &invalidCIDR,
			}
			volumeType     = "default"
			maxSurge       = intstr.FromInt(1)
			maxUnavailable = intstr.FromInt(0)
			worker         = garden.Worker{
				Name: "worker-name",
				Machine: garden.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "40Gi",
					Type: &volumeType,
				},
			}
			invalidWorker = garden.Worker{
				Name: "",
				Machine: garden.Machine{
					Type: "",
				},
				Minimum:        -1,
				Maximum:        -2,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "",
				},
			}
			invalidWorkerName = garden.Worker{
				Name: "not_compliant",
				Machine: garden.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "40Gi",
					Type: &volumeType,
				},
			}
			invalidWorkerTooLongName = garden.Worker{
				Name: "worker-name-is-too-long",
				Machine: garden.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "40Gi",
					Type: &volumeType,
				},
			}
			workerAutoScalingInvalid = garden.Worker{
				Name: "cpu-worker",
				Machine: garden.Machine{
					Type: "large",
				},
				Minimum:        0,
				Maximum:        2,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "40Gi",
					Type: &volumeType,
				},
			}
			workerAutoScalingMinMaxZero = garden.Worker{
				Name: "cpu-worker",
				Machine: garden.Machine{
					Type: "large",
				},
				Minimum:        0,
				Maximum:        0,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume: &garden.Volume{
					Size: "40Gi",
					Type: &volumeType,
				},
			}
		)

		BeforeEach(func() {
			shoot = &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec: garden.ShootSpec{
					Addons: &garden.Addons{
						Kube2IAM: &garden.Kube2IAM{
							Addon: addon,
							Roles: []garden.Kube2IAMRole{
								{
									Name:        "iam-role",
									Description: "some-text",
									Policy:      `{"some-valid": "json-document"}`,
								},
							},
						},
						KubernetesDashboard: &garden.KubernetesDashboard{
							Addon: addon,
						},
						ClusterAutoscaler: &garden.AddonClusterAutoscaler{
							Addon: addon,
						},
						NginxIngress: &garden.NginxIngress{
							Addon: addon,
						},
						KubeLego: &garden.KubeLego{
							Addon: addon,
							Mail:  "info@example.com",
						},
					},
					Cloud: garden.Cloud{
						Profile: "aws-profile",
						Region:  "eu-west-1",
						SecretBindingRef: corev1.LocalObjectReference{
							Name: "my-secret",
						},
						AWS: &garden.AWSCloud{
							Networks: garden.AWSNetworks{
								K8SNetworks: k8sNetworks,
								Internal:    []string{"10.250.1.0/24"},
								Public:      []string{"10.250.2.0/24"},
								Workers:     []string{"10.250.3.0/24"},
								VPC: garden.AWSVPC{
									CIDR: &nodeCIDR,
								},
							},
							Workers: []garden.Worker{worker},
							Zones:   []string{"eu-west-1a"},
						},
					},
					CloudProfileName:  "aws-profile",
					Region:            "eu-west-1",
					SecretBindingName: "my-secret",
					DNS: &garden.DNS{
						Providers: []garden.DNSProvider{
							{
								Type: &dnsProviderType,
							},
						},
						Domain: &domain,
					},
					Kubernetes: garden.Kubernetes{
						Version: "1.11.2",
						KubeAPIServer: &garden.KubeAPIServerConfig{
							OIDCConfig: &garden.OIDCConfig{
								CABundle:       makeStringPointer("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       makeStringPointer("client-id"),
								GroupsClaim:    makeStringPointer("groups-claim"),
								GroupsPrefix:   makeStringPointer("groups-prefix"),
								IssuerURL:      makeStringPointer("https://some-endpoint.com"),
								UsernameClaim:  makeStringPointer("user-claim"),
								UsernamePrefix: makeStringPointer("user-prefix"),
							},
							AdmissionPlugins: []garden.AdmissionPlugin{
								{
									Name: "PodNodeSelector",
									Config: &garden.ProviderConfig{
										RawExtension: runtime.RawExtension{
											Raw: []byte(`podNodeSelectorPluginConfig:
  clusterDefaultNodeSelector: <node-selectors-labels>
  namespace1: <node-selectors-labels>
	namespace2: <node-selectors-labels>`),
										},
									},
								},
							},
							AuditConfig: &garden.AuditConfig{
								AuditPolicy: &garden.AuditPolicy{
									ConfigMapRef: &corev1.ObjectReference{
										Name: "audit-policy-config",
									},
								},
							},
							EnableBasicAuthentication: makeBoolPointer(true),
						},
						KubeControllerManager: &garden.KubeControllerManagerConfig{
							NodeCIDRMaskSize: makeIntPointer(22),
							HorizontalPodAutoscalerConfig: &garden.HorizontalPodAutoscalerConfig{
								DownscaleDelay: makeDurationPointer(15 * time.Minute),
								SyncPeriod:     makeDurationPointer(30 * time.Second),
								Tolerance:      makeFloat64Pointer(0.1),
								UpscaleDelay:   makeDurationPointer(1 * time.Minute),
							},
						},
					},
					Networking: garden.Networking{
						Type: "some-network-plugin",
					},
					Provider: garden.Provider{
						Type:    "aws",
						Workers: []garden.Worker{worker},
					},
					Maintenance: &garden.Maintenance{
						AutoUpdate: &garden.MaintenanceAutoUpdate{
							KubernetesVersion: true,
						},
						TimeWindow: &garden.MaintenanceTimeWindow{
							Begin: "220000+0100",
							End:   "230000+0100",
						},
					},
					Monitoring: &garden.Monitoring{
						Alerting: &garden.Alerting{},
					},
				},
			}
		})

		It("should forbid shoots containing two consecutive hyphens", func() {
			shoot.ObjectMeta.Name = "sho--ot"

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid shoots with a not DNS-1123 label compliant name", func() {
			shoot.ObjectMeta.Name = "shoot.test"

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("metadata.name"),
			}))
		})

		It("should forbid empty Shoot resources", func() {
			shoot := &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{},
				Spec:       garden.ShootSpec{},
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
					"Field": Equal("spec.cloud.profile"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.secretBindingRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.kubernetes.version"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloudProfileName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.secretBindingName"),
				})),
			))
		})

		It("should forbid unsupported addon configuration", func() {
			shoot.Spec.Addons.Kube2IAM.Roles = []garden.Kube2IAMRole{
				{
					Name:        "",
					Description: "",
					Policy:      "invalid-json",
				},
			}
			shoot.Spec.Addons.KubeLego.Mail = "some-invalid-email"
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer("does-not-exist")

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.addons.kube2iam.roles[0].name"),
			})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.addons.kube2iam.roles[0].description"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.addons.kube2iam.roles[0].policy"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.addons.kube-lego.mail"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
				})),
			))
		})

		It("should allow external traffic policies 'Cluster' for nginx-ingress", func() {
			v := corev1.ServiceExternalTrafficPolicyTypeCluster
			shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v
			errorList := ValidateShoot(shoot)
			Expect(errorList).To(BeEmpty())
		})

		It("should allow external traffic policies 'Local' for nginx-ingress", func() {
			v := corev1.ServiceExternalTrafficPolicyTypeLocal
			shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v
			errorList := ValidateShoot(shoot)
			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unsupported external traffic policies for nginx-ingress", func() {
			v := corev1.ServiceExternalTrafficPolicyType("something-else")
			shoot.Spec.Addons.NginxIngress.ExternalTrafficPolicy = &v

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.addons.nginx-ingress.externalTrafficPolicy"),
			}))))
		})

		It("should forbid using basic auth mode for kubernetes dashboard when it's disabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(garden.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = makeBoolPointer(false)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
			}))))
		})

		It("should allow using basic auth mode for kubernetes dashboard when it's enabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(garden.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = makeBoolPointer(true)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unsupported specification (provider independent)", func() {
			shoot.Spec.CloudProfileName = ""
			shoot.Spec.Region = ""
			shoot.Spec.SecretBindingName = ""
			shoot.Spec.SeedName = makeStringPointer("")
			shoot.Spec.Provider.Type = ""

			shoot.Spec.Cloud.Profile = ""
			shoot.Spec.Cloud.Region = ""
			shoot.Spec.Cloud.SecretBindingRef = corev1.LocalObjectReference{
				Name: "",
			}
			shoot.Spec.Cloud.Seed = makeStringPointer("")

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloudProfileName"),
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
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.profile"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.cloud.secretBindingRef.name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.seed"),
				})),
			))
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

		It("should forbid updating some cloud keys", func() {
			newShoot := prepareShootForUpdate(shoot)
			shoot.Spec.CloudProfileName = "another-profile"
			shoot.Spec.Region = "another-region"
			shoot.Spec.SecretBindingName = "another-reference"
			shoot.Spec.Provider.Type = "another-provider"

			newShoot.Spec.Cloud.Profile = "another-profile"
			newShoot.Spec.Cloud.Region = "another-region"
			newShoot.Spec.Cloud.SecretBindingRef = corev1.LocalObjectReference{
				Name: "another-reference",
			}

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloudProfileName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.secretBindingName"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.profile"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.region"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.secretBindingRef"),
				})),
			))
		})

		It("should forbid updating the seed, if it has been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Seed = makeStringPointer("another-seed")
			shoot.Spec.Cloud.Seed = makeStringPointer("first-seed")

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.cloud.seed"),
				}))),
			)
		})

		It("should forbid passing an extension w/o type information", func() {
			extension := garden.Extension{}
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.extensions[0].type"),
				}))))
		})

		It("should allow passing an extension w/ type information", func() {
			extension := garden.Extension{
				Type: "arbitrary",
			}
			shoot.Spec.Extensions = append(shoot.Spec.Extensions, extension)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the seed if it has not been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Cloud.Seed = makeStringPointer("another-seed")
			shoot.Spec.Cloud.Seed = nil

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(0))
		})

		Context("AWS specific validation", func() {
			var (
				fldPath  = "aws"
				awsCloud *garden.AWSCloud
			)

			BeforeEach(func() {
				awsCloud = &garden.AWSCloud{
					Networks: garden.AWSNetworks{
						K8SNetworks: k8sNetworks,
						Internal:    []string{"10.250.1.0/24"},
						Public:      []string{"10.250.2.0/24"},
						Workers:     []string{"10.250.3.0/24"},
						VPC: garden.AWSVPC{
							CIDR: &vpcCIDR,
						},
					},
					Workers: []garden.Worker{worker},
					Zones:   []string{"eu-west-1a"},
				}
				shoot.Spec.Cloud.AWS = awsCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {
				It("should forbid invalid VPC CIDRs", func() {
					shoot.Spec.Cloud.AWS.Networks.VPC.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.vpc.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid internal CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Internal = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid public CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Public = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Workers = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid internal CIDR which is not in VPC CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Internal = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid public CIDR which is not in VPC CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Public = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid workers CIDR which are not in VPC and Nodes CIDR", func() {
					shoot.Spec.Cloud.AWS.Networks.Workers = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VPC CIDR", func() {
					podCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VPC CIDR", func() {
					servicesCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid VPC CIDRs to overlap with other VPC CIDRs", func() {
					overlappingCIDR := "10.250.0.1/32"
					shoot.Spec.Cloud.AWS.Networks.Public = []string{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Internal = []string{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Workers = []string{overlappingCIDR}
					shoot.Spec.Cloud.AWS.Networks.Nodes = &overlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.internal[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.internal[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.public[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.public[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.workers[0]" ("10.250.0.1/32")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.public[0]"),
						"Detail": Equal(`must not be a subset of "spec.cloud.aws.networks.workers[0]" ("10.250.0.1/32")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.AWS.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.aws.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := "10.0.0.3/8"
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"

				shoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.2.7/24"}
				shoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.1.6/24"}
				shoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.3.8/24"}
				shoot.Spec.Cloud.AWS.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.AWS.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.AWS.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.AWS.Networks.VPC = garden.AWSVPC{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(7))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.vpc.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.internal[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.public[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.aws.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				w := invalidWorker.DeepCopy()
				w.Volume.Size = "hugo"
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{workerAutoScalingInvalid, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{worker, workerAutoScalingMinMaxZero}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid worker pools with too less volume size", func() {
				w := worker.DeepCopy()
				w.Volume.Size = "19Gi"
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.AWS.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.AWS.Workers = []garden.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.AWS.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid mutating values of existing networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS.Networks.Internal[0] = "10.250.10.0/24"
				newShoot.Spec.Cloud.AWS.Networks.Public[0] = "10.250.20.0/24"
				newShoot.Spec.Cloud.AWS.Networks.Workers[0] = "10.250.30.0/24"
				newShoot.Spec.Cloud.AWS.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid changing zone / networks order", func() {
				oldShoot := prepareShootForUpdate(shoot)
				// mutate old values
				oldShoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.10.0/24", "10.250.40.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.20.0/24", "10.250.50.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				oldShoot.Spec.Cloud.AWS.Zones = []string{"another-zone", "yet-another-zone"}

				newShoot := oldShoot.DeepCopy()
				oldShoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.40.0/24", "10.250.10.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.50.0/24", "10.250.20.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.60.0/24", "10.250.30.0/24"}
				oldShoot.Spec.Cloud.AWS.Zones = []string{"yet-another-zone", "another-zone"}
				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding networks and zones while changing the order of their old values", func() {
				oldShoot := prepareShootForUpdate(shoot)
				// mutate old values
				oldShoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.10.0/24", "10.250.40.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.20.0/24", "10.250.50.0/24"}
				oldShoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				oldShoot.Spec.Cloud.AWS.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.40.0/24", "10.250.10.0/24", "10.250.70.0/24"}
				newShoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.50.0/24", "10.250.20.0/24", "10.250.80.0/24"}
				newShoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.60.0/24", "10.250.30.0/24", "10.250.90.0/24"}
				newShoot.Spec.Cloud.AWS.Zones = []string{"another-zone", "zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding networks and zones while mutating their old values", func() {
				newShoot := prepareShootForUpdate(shoot)
				// mutate old values
				newShoot.Spec.Cloud.AWS.Networks.Internal = []string{"10.250.10.0/24", "10.250.40.0/24"}
				newShoot.Spec.Cloud.AWS.Networks.Public = []string{"10.250.20.0/24", "10.250.50.0/24"}
				newShoot.Spec.Cloud.AWS.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				newShoot.Spec.Cloud.AWS.Zones = []string{"another-zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid deleting networks and zones", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Cloud.AWS.Networks.Internal = append(oldShoot.Spec.Cloud.AWS.Networks.Internal, "10.250.10.0/24")
				oldShoot.Spec.Cloud.AWS.Networks.Public = append(oldShoot.Spec.Cloud.AWS.Networks.Public, "10.250.20.0/24")
				oldShoot.Spec.Cloud.AWS.Networks.Workers = append(oldShoot.Spec.Cloud.AWS.Networks.Workers, "10.250.30.0/24")
				oldShoot.Spec.Cloud.AWS.Zones = append(oldShoot.Spec.Cloud.AWS.Zones, "another-zone")

				newShoot := prepareShootForUpdate(oldShoot)
				deleteElement(&newShoot.Spec.Cloud.AWS.Networks.Internal)
				deleteElement(&newShoot.Spec.Cloud.AWS.Networks.Public)
				deleteElement(&newShoot.Spec.Cloud.AWS.Networks.Workers)
				deleteElement(&newShoot.Spec.Cloud.AWS.Zones)

				errorList := ValidateShootUpdate(newShoot, oldShoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should allow adding networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS.Networks.Internal = append(newShoot.Spec.Cloud.AWS.Networks.Internal, "10.250.10.0/24")
				newShoot.Spec.Cloud.AWS.Networks.Public = append(newShoot.Spec.Cloud.AWS.Networks.Public, "10.250.20.0/24")
				newShoot.Spec.Cloud.AWS.Networks.Workers = append(newShoot.Spec.Cloud.AWS.Networks.Workers, "10.250.30.0/24")
				newShoot.Spec.Cloud.AWS.Zones = append(newShoot.Spec.Cloud.AWS.Zones, "another-zone")

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).NotTo(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.internal", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.public", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the AWS section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.AWS = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}
							shoot.Spec.Cloud.AWS.Workers = append(shoot.Spec.Cloud.AWS.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.AWS.Workers = append(shoot.Spec.Cloud.AWS.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("Azure specific validation", func() {
			var (
				fldPath    = "azure"
				azureCloud *garden.AzureCloud
			)

			BeforeEach(func() {
				azureCloud = &garden.AzureCloud{
					Networks: garden.AzureNetworks{
						K8SNetworks: k8sNetworks,
						Workers:     "10.250.3.0/24",
						VNet: garden.AzureVNet{
							CIDR: &vpcCIDR,
						},
					},
					Workers: []garden.Worker{worker},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Azure = azureCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying a resource group configuration", func() {
				shoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
				}))
			})

			Context("VNet", func() {
				It("should forbid specifying a vnet name without resource group", func() {
					vnetName := "existing-vnet"
					shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{
						Name: &vnetName,
					}
					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet", fldPath)),
						"Detail": Equal("specifying an existing vnet require a vnet name and vnet resource group"),
					}))
				})

				It("should forbid specifying a vnet resource group without name", func() {
					vnetGroup := "existing-vnet-rg"
					shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{
						ResourceGroup: &vnetGroup,
					}
					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet", fldPath)),
						"Detail": Equal("specifying an existing vnet require a vnet name and vnet resource group"),
					}))
				})

				It("should forbid specifying existing vnet plus a vnet cidr", func() {
					vnetName := "existing-vnet"
					vnetGroup := "existing-vnet-rg"
					shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{
						Name:          &vnetName,
						ResourceGroup: &vnetGroup,
						CIDR:          &vpcCIDR,
					}
					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(fmt.Sprintf("spec.cloud.%s.networks.vnet.cidr", fldPath)),
						"Detail": Equal("specifying a cidr for an existing vnet is not possible"),
					}))
				})

				It("should pass if no vnet cidr is specified and default is applied", func() {
					nodesCIDR := "10.250.3.0/24"

					shoot.Spec.Networking.Nodes = &nodesCIDR
					shoot.Spec.Cloud.Azure.Networks = garden.AzureNetworks{
						Workers: "10.250.3.0/24",
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})
			})

			Context("Zoned", func() {
				var (
					specField = "spec.cloud.azure.zones"
					azZones   = []string{"1", "2"}
				)
				It("should forbid to move a zoned shoot into a non zoned shoot", func() {
					shoot.Spec.Cloud.Azure.Zones = azZones
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Cloud.Azure.Zones = []string{}

					errorList := ValidateShootUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(specField),
						"Detail": ContainSubstring(`Can't move from zoned cluster to non zoned cluster`),
					}))
				})

				It("should forbid to move a non zoned shoot into a zoned shoot", func() {
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Cloud.Azure.Zones = azZones

					errorList := ValidateShootUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal(specField),
						"Detail": ContainSubstring(`Can't move from non zoned cluster to zoned cluster`),
					}))
				})

				It("should forbid changing zone", func() {
					oldShoot := prepareShootForUpdate(shoot)
					// mutate old values
					oldShoot.Spec.Cloud.Azure.Zones = []string{"another-zone", "yet-another-zone"}

					newShoot := oldShoot.DeepCopy()
					newShoot.Spec.Cloud.Azure.Zones = []string{"yet-another-zone", "another-zone"}
					errorList := ValidateShootUpdate(newShoot, shoot)

					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
						})),
					))
				})

				It("should forbid adding zones while changing the order of old values", func() {
					oldShoot := prepareShootForUpdate(shoot)
					// mutate old values
					oldShoot.Spec.Cloud.Azure.Zones = []string{"zone", "another-zone"}

					newShoot := oldShoot.DeepCopy()
					newShoot.Spec.Cloud.Azure.Zones = []string{"another-zone", "zone", "yet-another-zone"}

					errorList := ValidateShootUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
						})),
					))
				})

				It("should forbid adding zones while mutating their old values", func() {
					newShoot := prepareShootForUpdate(shoot)
					newShoot.Spec.Cloud.Azure.Zones = []string{"another-zone", "yet-another-zone"}

					errorList := ValidateShootUpdate(newShoot, shoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
						})),
					))
				})

				It("should allow adding zones", func() {
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.Cloud.Azure.Zones = []string{"zone"}

					newShoot := prepareShootForUpdate(oldShoot)
					newShoot.Spec.Cloud.Azure.Zones = append(newShoot.Spec.Cloud.Azure.Zones, "another-zone")

					errorList := ValidateShootUpdate(newShoot, oldShoot)
					Expect(errorList).NotTo(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
						})),
					))
				})
				It("should forbid deleting  zones", func() {
					oldShoot := shoot.DeepCopy()
					oldShoot.Spec.Cloud.Azure.Zones = append(shoot.Spec.Cloud.Azure.Zones, "another-zone")

					newShoot := prepareShootForUpdate(oldShoot)
					deleteElement(&newShoot.Spec.Cloud.Azure.Zones)

					errorList := ValidateShootUpdate(newShoot, oldShoot)
					Expect(errorList).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
						})),
					))
				})
			})

			Context("CIDR", func() {

				It("should forbid invalid VNet CIDRs", func() {
					shoot.Spec.Cloud.Azure.Networks.VNet.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.vnet.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.Azure.Networks.Workers = invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers which are not in VNet anmd Nodes CIDR", func() {
					notOverlappingCIDR := "1.1.1.1/32"
					// shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Nodes = &notOverlappingCIDR
					shoot.Spec.Cloud.Azure.Networks.Workers = notOverlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal(`must be a subset of "spec.cloud.azure.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.workers"),
						"Detail": Equal(`must be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VNet CIDR", func() {
					podCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VNet CIDR", func() {
					servicesCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.azure.networks.vnet.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.Azure.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.azure.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := "10.0.0.3/8"
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"
				workers := "10.250.3.8/24"

				shoot.Spec.Cloud.Azure.Networks.Workers = workers
				shoot.Spec.Cloud.Azure.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.Azure.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Azure.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.Azure.Networks.VNet = garden.AzureVNet{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.networks.vnet.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.azure.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{invalidWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{workerAutoScalingInvalid, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{worker, workerAutoScalingMinMaxZero}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				w := worker.DeepCopy()
				w.Volume.Size = "30Gi"
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Azure.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Azure.Workers = []garden.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid updating resource group and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				cidr := "10.250.0.0/19"
				newShoot.Spec.Cloud.Azure.Networks.Nodes = &cidr
				newShoot.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{
					Name: "another-group",
				}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.resourceGroup.name", fldPath)),
					})),
				))
			})

			It("should forbid removing the Azure section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Azure = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Azure.Workers = append(shoot.Spec.Cloud.Azure.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Azure.Workers = append(shoot.Spec.Cloud.Azure.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("GCP specific validation", func() {
			var (
				fldPath  = "gcp"
				gcpCloud *garden.GCPCloud
				internal = "10.10.0.0/24"
			)

			BeforeEach(func() {
				gcpCloud = &garden.GCPCloud{
					Networks: garden.GCPNetworks{
						K8SNetworks: k8sNetworks,
						Internal:    &internal,
						Workers:     []string{"10.250.0.0/16"},
						VPC: &garden.GCPVPC{
							Name: "hugo",
						},
					},
					Workers: []garden.Worker{worker},
					Zones:   []string{"europe-west1-b"},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.GCP = gcpCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})

			Context("CIDR", func() {
				It("should forbid more than one CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []string{"10.250.0.1/32", "10.250.0.2/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers"),
						"Detail": Equal("must specify only one worker cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid internal CIDR", func() {
					invalidCIDR = "invalid-cidr"
					shoot.Spec.Cloud.GCP.Networks.Internal = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.GCP.Networks.Workers = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.gcp.networks.nodes" ("10.250.0.0/16")`),
					}))
				})

				It("should forbid Internal CIDR to overlap with Node - and Worker CIDR", func() {
					overlappingCIDR := "10.250.1.0/30"
					shoot.Spec.Cloud.GCP.Networks.Internal = &overlappingCIDR
					shoot.Spec.Cloud.GCP.Networks.Workers = []string{overlappingCIDR}
					shoot.Spec.Cloud.GCP.Networks.Nodes = &overlappingCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal(`must not be a subset of "spec.cloud.gcp.networks.nodes" ("10.250.1.0/30")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.internal"),
						"Detail": Equal(`must not be a subset of "spec.cloud.gcp.networks.workers[0]" ("10.250.1.0/30")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.GCP.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.gcp.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"
				internal := "10.10.0.4/24"
				shoot.Spec.Cloud.GCP.Networks.Internal = &internal
				shoot.Spec.Cloud.GCP.Networks.Workers = []string{"10.250.3.8/24"}
				shoot.Spec.Cloud.GCP.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.GCP.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.GCP.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.networks.internal[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.gcp.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{invalidWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{workerAutoScalingInvalid, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{worker, workerAutoScalingMinMaxZero}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				w := worker.DeepCopy()
				w.Volume.Size = "19Gi"
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.GCP.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.GCP.Workers = []garden.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.GCP.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid changing zone / networks order", func() {
				oldShoot := prepareShootForUpdate(shoot)
				// mutate old values
				oldShoot.Spec.Cloud.GCP.Zones = []string{"another-zone", "yet-another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.GCP.Zones = []string{"yet-another-zone", "another-zone"}
				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding zones while changing the order of old values", func() {
				oldShoot := prepareShootForUpdate(shoot)
				// mutate old values
				oldShoot.Spec.Cloud.GCP.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.GCP.Zones = []string{"another-zone", "zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding zones while mutating their old values", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP.Zones = []string{"another-zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should allow adding zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP.Zones = append(newShoot.Spec.Cloud.GCP.Zones, "another-zone")

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).NotTo(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid deleting  zones", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Cloud.GCP.Zones = append(shoot.Spec.Cloud.GCP.Zones, "another-zone")

				newShoot := prepareShootForUpdate(oldShoot)
				deleteElement(&newShoot.Spec.Cloud.GCP.Zones)

				errorList := ValidateShootUpdate(newShoot, oldShoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP.Networks.Workers[0] = "10.250.0.0/24"
				newShoot.Spec.Cloud.GCP.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the GCP section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.GCP = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.GCP.Workers = append(shoot.Spec.Cloud.GCP.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.GCP.Workers = append(shoot.Spec.Cloud.GCP.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("Alicloud specific validation", func() {
			var (
				fldPath  = "alicloud"
				alicloud *garden.Alicloud
			)

			BeforeEach(func() {
				alicloud = &garden.Alicloud{
					Networks: garden.AlicloudNetworks{
						K8SNetworks: k8sNetworks,
						VPC: garden.AlicloudVPC{
							CIDR: &vpcCIDR,
						},
						Workers: []string{"10.250.3.0/24"},
					},
					Workers: []garden.Worker{worker},
					Zones:   []string{"cn-beijing-f"},
				}

				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Alicloud = alicloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {

				It("should forbid invalid VPC CIDRs", func() {
					shoot.Spec.Cloud.Alicloud.Networks.VPC.CIDR = &invalidCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.vpc.cidr"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.nodes" ("10.250.0.0/16")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Node which are not in VPC CIDR", func() {
					notOverlappingCIDR := "1.1.1.1/32"
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Nodes = &notOverlappingCIDR
					shoot.Spec.Cloud.Alicloud.Networks.Workers = []string{notOverlappingCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.nodes"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Pod CIDR to overlap with VPC CIDR", func() {
					podCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Pods = &podCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.pods"),
						"Detail": Equal(`must not be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should forbid Services CIDR to overlap with VPC CIDR", func() {
					servicesCIDR := "10.0.0.1/32"
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks.Services = &servicesCIDR

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.services"),
						"Detail": Equal(`must not be a subset of "spec.cloud.alicloud.networks.vpc.cidr" ("10.0.0.0/8")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.Alicloud.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.alicloud.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				vpcCIDR := "10.0.0.3/8"
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"

				shoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.3.8/24"}
				shoot.Spec.Cloud.Alicloud.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.Alicloud.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Alicloud.Networks.Pods = &podCIDR
				shoot.Spec.Cloud.Alicloud.Networks.VPC = garden.AlicloudVPC{CIDR: &vpcCIDR}

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(5))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.networks.vpc.cidr"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.alicloud.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{invalidWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{workerAutoScalingInvalid, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{worker, workerAutoScalingMinMaxZero}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid worker pools with too less volume size", func() {
				w := worker.DeepCopy()
				w.Volume.Size = "10Gi"
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Alicloud.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Alicloud.Workers = []garden.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.Alicloud.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Alicloud.Networks.Workers[0] = "10.250.0.0/24"
				newShoot.Spec.Cloud.Alicloud.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid changing zone / networks order", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				oldShoot.Spec.Cloud.Alicloud.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.60.0/24", "10.250.30.0/24"}
				newShoot.Spec.Cloud.Alicloud.Zones = []string{"another-zone", "zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding networks and zones while changing the order of their old values", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				oldShoot.Spec.Cloud.Alicloud.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.60.0/24", "10.250.30.0/24", "10.250.80.0/24"}
				newShoot.Spec.Cloud.Alicloud.Zones = []string{"another-zone", "zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding networks and zones while mutating their old values", func() {
				newShoot := prepareShootForUpdate(shoot)
				// mutate old values
				newShoot.Spec.Cloud.Alicloud.Networks.Workers = []string{"10.250.30.0/24", "10.250.60.0/24"}
				newShoot.Spec.Cloud.Alicloud.Zones = []string{"another-zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should allow adding networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Alicloud.Networks.Workers = append(newShoot.Spec.Cloud.Alicloud.Networks.Workers, "10.250.30.0/24")
				newShoot.Spec.Cloud.Alicloud.Zones = append(newShoot.Spec.Cloud.Alicloud.Zones, "another-zone")

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).NotTo(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the Alicloud section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Alicloud = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Alicloud.Workers = append(shoot.Spec.Cloud.Alicloud.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Alicloud.Workers = append(shoot.Spec.Cloud.Alicloud.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		// BEGIN PACKET
		Context("Packet specific validation", func() {
			var (
				fldPath = "packet"
				packet  *garden.PacketCloud
			)

			BeforeEach(func() {
				packet = &garden.PacketCloud{
					Networks: garden.PacketNetworks{
						K8SNetworks: k8sNetworks,
					},
					Workers: []garden.Worker{worker},
					Zones:   []string{"EWR1"},
				}

				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.Packet = packet
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("CIDR", func() {
				It("should forbid invalid k8s networks", func() {
					shoot.Spec.Cloud.Packet.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.packet.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"

				shoot.Spec.Cloud.Packet.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.Packet.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(2))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.packet.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.packet.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{worker, worker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))))
			})

			It("should forbid invalid worker configuration", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{invalidWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(6))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[5]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))
			})

			It("should forbid worker pools with too less volume size", func() {
				w := worker.DeepCopy()
				w.Volume.Size = "10Gi"
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].volume.size", fldPath)),
				}))))
			})

			It("should forbid too long worker names", func() {
				shoot.Spec.Cloud.Packet.Workers[0] = invalidWorkerTooLongName

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				shoot.Spec.Cloud.Packet.Workers = []garden.Worker{invalidWorkerName}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.Packet.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				cidr := "10.250.0.0/24"
				newShoot.Spec.Cloud.Packet.Networks.Nodes = &cidr
				newShoot.Spec.Cloud.Packet.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.nodes", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the Packet section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.Packet = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Packet.Workers = append(shoot.Spec.Cloud.Packet.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}

							shoot.Spec.Cloud.Packet.Workers = append(shoot.Spec.Cloud.Packet.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})
		// END PACKET

		Context("OpenStack specific validation", func() {
			var (
				fldPath        = "openstack"
				openStackCloud *garden.OpenStackCloud
			)

			w := worker.DeepCopy()
			w.Volume = nil
			osWorker := *w

			BeforeEach(func() {
				openStackCloud = &garden.OpenStackCloud{
					FloatingPoolName:     "my-pool",
					LoadBalancerProvider: "haproxy",
					Networks: garden.OpenStackNetworks{
						K8SNetworks: k8sNetworks,
						Workers:     []string{"10.250.0.0/16"},
						Router: &garden.OpenStackRouter{
							ID: "router1234",
						},
					},
					Workers: []garden.Worker{osWorker},
					Zones:   []string{"europe-1a"},
				}
				shoot.Spec.Cloud.AWS = nil
				shoot.Spec.Cloud.OpenStack = openStackCloud
			})

			It("should not return any errors", func() {
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid invalid floating pool name configuration", func() {
				shoot.Spec.Cloud.OpenStack.FloatingPoolName = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.floatingPoolName", fldPath)),
				}))
			})

			It("should forbid invalid load balancer provider configuration", func() {
				shoot.Spec.Cloud.OpenStack.LoadBalancerProvider = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.loadBalancerProvider", fldPath)),
				}))
			})

			Context("CIDR", func() {
				It("should forbid more than one CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []string{"10.250.0.1/32", "10.250.0.2/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers"),
						"Detail": Equal("must specify only one worker cidr"),
					}))
				})

				It("should forbid invalid workers CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []string{invalidCIDR}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})

				It("should forbid workers CIDR which are not in Nodes CIDR", func() {
					shoot.Spec.Cloud.OpenStack.Networks.Workers = []string{"1.1.1.1/32"}

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
						"Detail": Equal(`must be a subset of "spec.cloud.openstack.networks.nodes" ("10.250.0.0/16")`),
					}))
				})

				It("should invalid k8s networks", func() {
					shoot.Spec.Cloud.OpenStack.Networks.K8SNetworks = invalidK8sNetworks

					errorList := ValidateShoot(shoot)

					Expect(errorList).To(ConsistOfFields(Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.nodes"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.pods"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}, Fields{
						"Type":   Equal(field.ErrorTypeInvalid),
						"Field":  Equal("spec.cloud.openstack.networks.services"),
						"Detail": Equal("invalid CIDR address: invalid-cidr"),
					}))
				})
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"

				shoot.Spec.Cloud.OpenStack.Networks.Workers = []string{"10.250.3.8/24"}
				shoot.Spec.Cloud.OpenStack.Networks.Nodes = &nodeCIDR
				shoot.Spec.Cloud.OpenStack.Networks.Services = &serviceCIDR
				shoot.Spec.Cloud.OpenStack.Networks.Pods = &podCIDR

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(4))

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.nodes"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.pods"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.services"),
					"Detail": Equal("must be valid canonical CIDR"),
				}, Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.cloud.openstack.networks.workers[0]"),
					"Detail": Equal("must be valid canonical CIDR"),
				}))
			})

			It("should forbid an empty worker list", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers", fldPath)),
				}))
			})

			It("should enforce unique worker names", func() {
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{osWorker, osWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[1]", fldPath)),
				}))
			})

			It("should forbid invalid worker configuration", func() {
				w := invalidWorker.DeepCopy()
				w.Volume = nil
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(5))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
				Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].machine.type", fldPath)),
				}))
				Expect(*errorList[2]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
				Expect(*errorList[3]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
				Expect(*errorList[4]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].maximum", fldPath)),
				}))
			})

			It("should enforce workers min > 0 if max > 0", func() {
				w := workerAutoScalingInvalid.DeepCopy()
				w.Volume = nil
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{*w, osWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].minimum", fldPath)),
				}))
			})

			It("should allow workers having min=max=0 if at least one pool is active", func() {
				w := workerAutoScalingMinMaxZero.DeepCopy()
				w.Volume = nil
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{*w, osWorker}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid too long worker names", func() {
				w := invalidWorkerTooLongName.DeepCopy()
				w.Volume = nil
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeTooLong),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid worker pools with names that are not DNS-1123 label compliant", func() {
				w := invalidWorkerName.DeepCopy()
				w.Volume = nil
				shoot.Spec.Cloud.OpenStack.Workers = []garden.Worker{*w}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.workers[0].name", fldPath)),
				}))
			})

			It("should forbid an empty zones list", func() {
				shoot.Spec.Cloud.OpenStack.Zones = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
				}))
			})

			It("should forbid updating networks and zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack.Networks.Workers[0] = "10.250.0.0/24"
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.networks.workers", fldPath)),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid changing zone order", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.Cloud.OpenStack.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone", "zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding networks  while changing the order of their old values", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.Cloud.OpenStack.Zones = []string{"zone", "another-zone"}

				newShoot := oldShoot.DeepCopy()
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone", "zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})
			It("should forbid adding zones while mutating their old values", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack.Zones = []string{"another-zone", "yet-another-zone"}

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should allow adding zones", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack.Zones = append(newShoot.Spec.Cloud.OpenStack.Zones, "another-zone")

				errorList := ValidateShootUpdate(newShoot, shoot)
				Expect(errorList).NotTo(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s.zones", fldPath)),
					})),
				))
			})

			It("should forbid removing the OpenStack section", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Cloud.OpenStack = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal(fmt.Sprintf("spec.cloud.%s", fldPath)),
					})),
				))
			})

			Context("NodeCIDRMask validation", func() {
				var (
					defaultMaxPod           int32 = 110
					maxPod                  int32 = 260
					defaultNodeCIDRMaskSize       = 24
					testWorker              garden.Worker
				)

				BeforeEach(func() {
					shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
					shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &defaultMaxPod}
					testWorker = *worker.DeepCopy()
					testWorker.Name = "testworker"
				})

				It("should not return any errors", func() {
					worker.Kubernetes = &garden.WorkerKubernetes{
						Kubelet: &garden.KubeletConfig{
							MaxPods: &defaultMaxPod,
						},
					}
					errorList := ValidateShoot(shoot)
					Expect(errorList).To(HaveLen(0))
				})

				Context("Non-default max pod settings", func() {
					Context("one worker pool", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}
							testWorker.Volume = nil

							shoot.Spec.Cloud.OpenStack.Workers = append(shoot.Spec.Cloud.OpenStack.Workers, testWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))

							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
					Context("multiple worker pools", func() {
						It("should deny NodeCIDR with too few ips", func() {
							testWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}
							testWorker.Volume = nil

							secondTestWorker := *testWorker.DeepCopy()
							secondTestWorker.Name = "testworker2"
							secondTestWorker.Kubernetes = &garden.WorkerKubernetes{
								Kubelet: &garden.KubeletConfig{
									MaxPods: &maxPod,
								},
							}
							secondTestWorker.Volume = nil

							shoot.Spec.Cloud.OpenStack.Workers = append(shoot.Spec.Cloud.OpenStack.Workers, testWorker, secondTestWorker)

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})

					Context("Global default max pod", func() {
						It("should deny NodeCIDR with too few ips", func() {
							shoot.Spec.Kubernetes.Kubelet = &garden.KubeletConfig{MaxPods: &maxPod}

							errorList := ValidateShoot(shoot)

							Expect(errorList).To(HaveLen(1))
							Expect(errorList).To(ConsistOfFields(Fields{
								"Type":   Equal(field.ErrorTypeInvalid),
								"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
								"Detail": ContainSubstring(`kubelet or kube-controller configuration incorrect`),
							}))
						})
					})
				})
			})
		})

		Context("dns section", func() {
			It("should forbid specifying a provider without a domain", func() {
				shoot.Spec.DNS.Domain = makeStringPointer("foo/bar.baz")
				shoot.Spec.DNS.Providers = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should allow specifying the 'unmanaged' provider without a domain", func() {
				shoot.Spec.DNS.Domain = nil
				shoot.Spec.DNS.Providers = []garden.DNSProvider{
					{
						Type: makeStringPointer(garden.DNSUnmanaged),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying invalid domain", func() {
				shoot.Spec.DNS.Providers = nil
				shoot.Spec.DNS.Domain = makeStringPointer("foo/bar.baz")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid specifying a secret name when provider equals 'unmanaged'", func() {
				shoot.Spec.DNS.Providers = []garden.DNSProvider{
					{
						Type:       makeStringPointer(garden.DNSUnmanaged),
						SecretName: makeStringPointer(""),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.providers[0].secretName"),
				}))))
			})

			It("should require a provider if a secret name is given", func() {
				shoot.Spec.DNS.Providers = []garden.DNSProvider{
					{
						SecretName: makeStringPointer(""),
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
				newShoot.Spec.DNS = &garden.DNS{
					Domain: makeStringPointer("some-domain.com"),
				}

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow assigning the dns domain (dns non-nil)", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.DNS = &garden.DNS{}
				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.DNS.Domain = makeStringPointer("some-domain.com")

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
				newShoot.Spec.DNS.Domain = makeStringPointer("another-domain.com")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should allow updating the dns providers if seed is assigned", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil
				oldShoot.Spec.DNS.Providers[0].Type = makeStringPointer("some-dns-provider")

				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.SeedName = makeStringPointer("seed")
				newShoot.Spec.DNS.Providers = nil

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid updating the dns provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Spec.DNS.Providers[0].Type = makeStringPointer("some-other-provider")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.providers[0].type"),
				}))))
			})

			It("should allow updating the dns secret name", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Providers[0].SecretName = makeStringPointer("my-dns-secret")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("OIDC validation", func() {
			It("should forbid unsupported OIDC configuration", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.CABundle = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsClaim = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsPrefix = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernameClaim = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernamePrefix = makeStringPointer("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.SigningAlgs = []string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.caBundle"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsClaim"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.groupsPrefix"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.signingAlgs"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernameClaim"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.usernamePrefix"),
				}))))
			})

			It("should forbid unsupported OIDC configuration (for K8S >= v1.10)", func() {
				shoot.Spec.Kubernetes.Version = "1.10.1"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.requiredClaims"),
				}))
			})

			It("should allow supported OIDC configuration (for K8S >= v1.11)", func() {
				shoot.Spec.Kubernetes.Version = "1.11.1"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{
					"some": "claim",
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("admission plugin validation", func() {
			It("should allow not specifying admission plugins", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying admission plugins without a name", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.AdmissionPlugins = []garden.AdmissionPlugin{
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
		})

		Context("KubeControllerManager validation < 1.12", func() {
			It("should forbid unsupported HPA configuration", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.SyncPeriod = makeDurationPointer(100 * time.Millisecond)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.Tolerance = makeFloat64Pointer(0)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = makeDurationPointer(-1 * time.Second)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.syncPeriod"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.tolerance"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.upscaleDelay"),
				}))))
			})

			It("should forbid unsupported HPA field configuration for versions < 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(5 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(5 * time.Minute)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleStabilization"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.initialReadinessDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.cpuInitializationPeriod"),
				}))))
			})
		})

		Context("KubeControllerManager validation in versions > 1.12", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.Version = "1.12.1"
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = nil
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = nil
			})

			It("should forbid unsupported HPA configuration in versions > 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(-1 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(-1 * time.Second)

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

			It("should fail when using configuration parameters from versions older than 1.12", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = makeDurationPointer(1 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = makeDurationPointer(1 * time.Second)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.upscaleDelay"),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeControllerManager.horizontalPodAutoscaler.downscaleDelay"),
				}))))
			})

			It("should succeed when using valid v1.12 configuration parameters", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleStabilization = makeDurationPointer(5 * time.Minute)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.InitialReadinessDelay = makeDurationPointer(30 * time.Second)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.CPUInitializationPeriod = makeDurationPointer(5 * time.Minute)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("KubeControllerManager configuration validation", func() {
			It("should fail updating immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(22)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})

			It("should succeed not changing immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(24)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should fail when nodeCIDRMaskSize is out of upper boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(32)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
				})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					}))))
			})

			It("should fail when nodeCIDRMaskSize is out of lower boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(0)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
				})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeInvalid),
						"Field": Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					}))))
			})

			It("should succeed when nodeCIDRMaskSize is within boundaries", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeIntPointer(22)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("KubeProxy validation", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeProxy = &garden.KubeProxyConfig{}
			})

			It("should succeed when using IPTables mode", func() {
				mode := garden.ProxyModeIPTables
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())

			})

			It("should succeed when using IPVS mode", func() {
				mode := garden.ProxyModeIPVS
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
				m := garden.ProxyMode("fooMode")
				shoot.Spec.Kubernetes.KubeProxy.Mode = &m
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeNotSupported),
					"Field": Equal("spec.kubernetes.kubeProxy.mode"),
				}))))
			})

			It("should fail when using kuberntes version 1.14.2 and proxy mode is changed", func() {
				mode := garden.ProxyMode("IPVS")
				kubernetesConfig := garden.KubernetesConfig{nil}
				config := garden.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &mode,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				shoot.Spec.Kubernetes.Version = "1.14.2"
				oldMode := garden.ProxyMode("IPTables")
				oldConfig := garden.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &oldMode,
				}
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig
				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, shoot.DeletionTimestamp != nil, field.NewPath("spec"))
				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeProxy.mode"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should be successful when using kuberntes version 1.14.1 and proxy mode stays the same", func() {
				mode := garden.ProxyMode("IPVS")
				shoot.Spec.Kubernetes.Version = "1.14.1"
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(2))
			})
		})

		Context("ClusterAutoscaler validation", func() {
			DescribeTable("cluster autoscaler values",
				func(clusterAutoscaler garden.ClusterAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateClusterAutoscaler(clusterAutoscaler, nil)).To(matcher)
				},
				Entry("valid", garden.ClusterAutoscaler{}, BeEmpty()),
				Entry("valid with threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(0.5),
				}, BeEmpty()),
				Entry("invalid negative threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(-0.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
				Entry("invalid > 1 threshold", garden.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(1.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
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

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(2))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.kubernetes.version"),
			}))
			Expect(*errorList[1]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		It("should forbid kubernetes version downgrades", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = "1.7.2"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		It("should forbid kubernetes version upgrades skipping a minor version", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.Kubernetes.Version = "1.10.1"

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.kubernetes.version"),
			}))
		})

		Context("networking section", func() {
			It("should forbid not specifying a networking type", func() {
				shoot.Spec.Networking.Type = ""

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.networking.type"),
				}))))
			})

			It("should forbid changing the networking type", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Networking.Type = "some-other-type"

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.networking.type"),
				}))))
			})
		})

		Context("maintenance section", func() {
			It("should forbid not specifying the maintenance section", func() {
				shoot.Spec.Maintenance = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance"),
				}))
			})

			It("should forbid not specifying the auto update section", func() {
				shoot.Spec.Maintenance.AutoUpdate = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance.autoUpdate"),
				}))
			})

			It("should forbid not specifying the time window section", func() {
				shoot.Spec.Maintenance.TimeWindow = nil

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))
			})

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
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should forbid time windows smaller than 30 minutes", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "225000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "231000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.maintenance.timeWindow"),
				}))))
			})

			It("should allow time windows which overlap over two days", func() {
				shoot.Spec.Maintenance.TimeWindow.Begin = "230000+0100"
				shoot.Spec.Maintenance.TimeWindow.End = "010000+0100"

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		It("should forbid updating the spec for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(1))
			Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
			}))
		})

		It("should allow updating the metadata for shoots with deletion timestamp", func() {
			newShoot := prepareShootForUpdate(shoot)
			deletionTimestamp := metav1.NewTime(time.Now())
			shoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.DeletionTimestamp = &deletionTimestamp
			newShoot.ObjectMeta.Labels = map[string]string{
				"new-key": "new-value",
			}

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(0))
		})
	})

	Describe("#ValidateShootStatus, #ValidateShootStatusUpdate", func() {
		var shoot *garden.Shoot
		BeforeEach(func() {
			shoot = &garden.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec:   garden.ShootSpec{},
				Status: garden.ShootStatus{},
			}
		})

		Context("uid checks", func() {
			It("should allow setting the uid", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.UID = types.UID("1234")

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changing the uid", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Status.UID = types.UID("1234")
				newShoot.Status.UID = types.UID("1235")

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
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.TechnicalID = "shoot--foo--bar"

				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid changing the technical id", func() {
				newShoot := prepareShootForUpdate(shoot)
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
	})

	Describe("#ValidateWorker", func() {
		DescribeTable("reject when maxUnavailable and maxSurge are invalid",
			func(maxUnavailable, maxSurge intstr.IntOrString, expectType field.ErrorType) {
				worker := garden.Worker{
					Name: "worker-name",
					Machine: garden.Machine{
						Type: "large",
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(expectType),
				}))))
			},

			// double zero values (percent or int)
			Entry("two zero integers", intstr.FromInt(0), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("zero int and zero percent", intstr.FromInt(0), intstr.FromString("0%"), field.ErrorTypeInvalid),
			Entry("zero percent and zero int", intstr.FromString("0%"), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("two zero percents", intstr.FromString("0%"), intstr.FromString("0%"), field.ErrorTypeInvalid),

			// greater than 100
			Entry("maxUnavailable greater than 100 percent", intstr.FromString("101%"), intstr.FromString("100%"), field.ErrorTypeInvalid),

			// below zero tests
			Entry("values are not below zero", intstr.FromInt(-1), intstr.FromInt(0), field.ErrorTypeInvalid),
			Entry("percentage is not less than zero", intstr.FromString("-90%"), intstr.FromString("90%"), field.ErrorTypeInvalid),
		)

		DescribeTable("reject when labels are invalid",
			func(labels map[string]string, expectType field.ErrorType) {
				maxSurge := intstr.FromInt(1)
				maxUnavailable := intstr.FromInt(0)
				worker := garden.Worker{
					Name: "worker-name",
					Machine: garden.Machine{
						Type: "large",
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Labels:         labels,
				}
				errList := ValidateWorker(worker, nil)

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
				maxSurge := intstr.FromInt(1)
				maxUnavailable := intstr.FromInt(0)
				worker := garden.Worker{
					Name: "worker-name",
					Machine: garden.Machine{
						Type: "large",
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Annotations:    annotations,
				}
				errList := ValidateWorker(worker, nil)

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
				maxSurge := intstr.FromInt(1)
				maxUnavailable := intstr.FromInt(0)
				worker := garden.Worker{
					Name: "worker-name",
					Machine: garden.Machine{
						Type: "large",
					},
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
					Taints:         taints,
				}
				errList := ValidateWorker(worker, nil)

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
			Entry("non-existing", []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffect("does-not-exist")}}, field.ErrorTypeNotSupported),

			// uniqueness by key/effect
			Entry("not unique", []corev1.Taint{{Key: "foo", Value: "bar", Effect: corev1.TaintEffectNoSchedule}, {Key: "foo", Value: "baz", Effect: corev1.TaintEffectNoSchedule}}, field.ErrorTypeDuplicate),
		)
	})

	Describe("#ValidateWorkers", func() {
		DescribeTable("validate that at least one active worker pool is configured",
			func(min1, max1, min2, max2 int, matcher gomegatypes.GomegaMatcher) {
				workers := []garden.Worker{
					{
						Minimum: min1,
						Maximum: max1,
					},
					{
						Minimum: min2,
						Maximum: max2,
					},
				}

				errList := ValidateWorkers(workers, nil)

				Expect(errList).To(matcher)
			},

			Entry("at least one worker pool min>0, max>0", 0, 0, 1, 1, HaveLen(0)),
			Entry("all worker pools min=max=0", 0, 0, 0, 0, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeForbidden),
			})))),
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

		DescribeTable("validate the kubelet configuration - EvictionHard & EvictionSoft",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree string, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionHard: &garden.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
					EvictionSoft: &garden.KubeletConfigEviction{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantityValueMi, validResourceQuantityValueKi, validPercentValue, validPercentValue, validPercentValue, HaveLen(0)),
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

		validResourceQuantity := resource.MustParse(validResourceQuantityValueMi)
		DescribeTable("validate the kubelet configuration - EvictionMinimumReclaim",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree resource.Quantity, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionMinimumReclaim: &garden.KubeletConfigEvictionMinimumReclaim{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, HaveLen(0)),
			Entry("only allow positive resource.Quantity for any value", resource.MustParse(invalidResourceQuantityValue), validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("evictionMinimumReclaim.memoryAvailable").String()),
			})))),
		)
		validDuration := metav1.Duration{Duration: 2 * time.Minute}
		invalidDuration := metav1.Duration{Duration: -2 * time.Minute}
		DescribeTable("validate the kubelet configuration - KubeletConfigEvictionSoftGracePeriod",
			func(memoryAvailable, imagefsAvailable, imagefsInodesFree, nodefsAvailable, nodefsInodesFree metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionSoftGracePeriod: &garden.KubeletConfigEvictionSoftGracePeriod{
						MemoryAvailable:   &memoryAvailable,
						ImageFSAvailable:  &imagefsAvailable,
						ImageFSInodesFree: &imagefsInodesFree,
						NodeFSAvailable:   &nodefsAvailable,
						NodeFSInodesFree:  &nodefsInodesFree,
					},
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, validDuration, validDuration, validDuration, validDuration, HaveLen(0)),
			Entry("only allow positive Duration for any value", invalidDuration, validDuration, validDuration, validDuration, validDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionSoftGracePeriod.memoryAvailable").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - EvictionPressureTransitionPeriod",
			func(evictionPressureTransitionPeriod metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionPressureTransitionPeriod: &evictionPressureTransitionPeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, HaveLen(0)),
			Entry("only allow positive Duration", invalidDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionPressureTransitionPeriod").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - EvictionMaxPodGracePeriod",
			func(evictionMaxPodGracePeriod int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					EvictionMaxPodGracePeriod: &evictionMaxPodGracePeriod,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(90), HaveLen(0)),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("evictionMaxPodGracePeriod").String()),
				})))),
		)

		DescribeTable("validate the kubelet configuration - MaxPods",
			func(maxPods int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := garden.KubeletConfig{
					MaxPods: &maxPods,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", int32(110), HaveLen(0)),
			Entry("only allow positive number", int32(-3), ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("maxPods").String()),
				})))),
		)
	})

	Describe("#ValidateHibernationSchedules", func() {
		DescribeTable("validate hibernation schedules",
			func(schedules []garden.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationSchedules(schedules, nil)).To(matcher)
			},
			Entry("valid schedules", []garden.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}}, BeEmpty()),
			Entry("nil schedules", nil, BeEmpty()),
			Entry("duplicate start and end value in same schedule",
				[]garden.HibernationSchedule{{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("duplicate start and end value in different schedules",
				[]garden.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, {Start: makeStringPointer("1 * * * *"), End: makeStringPointer("3 * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("invalid schedule",
				[]garden.HibernationSchedule{{Start: makeStringPointer("foo"), End: makeStringPointer("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeInvalid),
				})))),
		)
	})

	Describe("#ValidateHibernationCronSpec", func() {
		DescribeTable("validate cron spec",
			func(seenSpecs sets.String, spec string, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationCronSpec(seenSpecs, spec, nil)).To(matcher)
			},
			Entry("invalid spec", sets.NewString(), "foo", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeInvalid),
			})))),
			Entry("duplicate spec", sets.NewString("* * * * *"), "* * * * *", ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type": Equal(field.ErrorTypeDuplicate),
			})))),
		)

		It("should add the inspected cron spec to the set if there were no issues", func() {
			var (
				s    = sets.NewString()
				spec = "* * * * *"
			)
			Expect(ValidateHibernationCronSpec(s, spec, nil)).To(BeEmpty())
			Expect(s.Has(spec)).To(BeTrue())
		})

		It("should not add the inspected cron spec to the set if there were issues", func() {
			var (
				s    = sets.NewString()
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
			func(seenSpecs sets.String, schedule *garden.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateHibernationSchedule(seenSpecs, schedule, nil)
				Expect(errList).To(matcher)
			},

			Entry("valid schedule", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, BeEmpty()),
			Entry("invalid start value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("start").String()),
			})))),
			Entry("invalid end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("invalid location", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *"), Location: makeStringPointer("foo")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("location").String()),
			})))),
			Entry("equal start and end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("nil start", sets.NewString(), &garden.HibernationSchedule{End: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("nil end", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("start and end nil", sets.NewString(), &garden.HibernationSchedule{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeRequired),
				})))),
			Entry("invalid start and end value", sets.NewString(), &garden.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("")},
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
})

func deleteElement(slice *[]string) {
	sliceCopy := *slice
	sliceCopy = sliceCopy[:len(sliceCopy)-1]
	*slice = sliceCopy
}

func prepareShootForUpdate(shoot *garden.Shoot) *garden.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}

func makeDurationPointer(d time.Duration) *metav1.Duration {
	return &metav1.Duration{Duration: d}
}

func makeFloat64Pointer(f float64) *float64 {
	ptr := f
	return &ptr
}

func makeIntPointer(i int) *int {
	ptr := i
	return &ptr
}

func makeBoolPointer(i bool) *bool {
	ptr := i
	return &ptr
}

// Helper functions
func makeStringPointer(s string) *string {
	ptr := s
	return &ptr
}
