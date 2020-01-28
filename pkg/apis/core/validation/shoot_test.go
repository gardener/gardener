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
	"strings"
	"time"

	"github.com/gardener/gardener/pkg/apis/core"
	. "github.com/gardener/gardener/pkg/apis/core/validation"
	. "github.com/gardener/gardener/pkg/utils/validation/gomega"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	gomegatypes "github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

var _ = Describe("Shoot Validation Tests", func() {
	Describe("#ValidateShoot, #ValidateShootUpdate", func() {
		var (
			shoot *core.Shoot

			domain          = "my-cluster.example.com"
			dnsProviderType = "some-provider"
			purpose         = core.ShootPurposeEvaluation
			addon           = core.Addon{
				Enabled: true,
			}

			maxSurge       = intstr.FromInt(1)
			maxUnavailable = intstr.FromInt(0)
			worker         = core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			}
			invalidWorker = core.Worker{
				Name: "",
				Machine: core.Machine{
					Type: "",
				},
				Minimum:        -1,
				Maximum:        -2,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			}
			invalidWorkerName = core.Worker{
				Name: "not_compliant",
				Machine: core.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			}
			invalidWorkerTooLongName = core.Worker{
				Name: "worker-name-is-too-long",
				Machine: core.Machine{
					Type: "large",
				},
				Minimum:        1,
				Maximum:        1,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			}
			workerAutoScalingInvalid = core.Worker{
				Name: "cpu-worker",
				Machine: core.Machine{
					Type: "large",
				},
				Minimum:        0,
				Maximum:        2,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
			}
			workerAutoScalingMinMaxZero = core.Worker{
				Name: "cpu-worker",
				Machine: core.Machine{
					Type: "large",
				},
				Minimum:        0,
				Maximum:        0,
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
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
					CloudProfileName:  "aws-profile",
					Region:            "eu-west-1",
					SecretBindingName: "my-secret",
					Purpose:           &purpose,
					DNS: &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type: &dnsProviderType,
							},
						},
						Domain: &domain,
					},
					Kubernetes: core.Kubernetes{
						Version: "1.11.2",
						KubeAPIServer: &core.KubeAPIServerConfig{
							OIDCConfig: &core.OIDCConfig{
								CABundle:       makeStringPointer("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       makeStringPointer("client-id"),
								GroupsClaim:    makeStringPointer("groups-claim"),
								GroupsPrefix:   makeStringPointer("groups-prefix"),
								IssuerURL:      makeStringPointer("https://some-endpoint.com"),
								UsernameClaim:  makeStringPointer("user-claim"),
								UsernamePrefix: makeStringPointer("user-prefix"),
							},
							AdmissionPlugins: []core.AdmissionPlugin{
								{
									Name: "PodNodeSelector",
									Config: &core.ProviderConfig{
										RawExtension: runtime.RawExtension{
											Raw: []byte(`podNodeSelectorPluginConfig:
  clusterDefaultNodeSelector: <node-selectors-labels>
  namespace1: <node-selectors-labels>
	namespace2: <node-selectors-labels>`),
										},
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
							EnableBasicAuthentication: makeBoolPointer(true),
						},
						KubeControllerManager: &core.KubeControllerManagerConfig{
							NodeCIDRMaskSize: makeInt32Pointer(22),
							HorizontalPodAutoscalerConfig: &core.HorizontalPodAutoscalerConfig{
								DownscaleDelay: makeDurationPointer(15 * time.Minute),
								SyncPeriod:     makeDurationPointer(30 * time.Second),
								Tolerance:      makeFloat64Pointer(0.1),
								UpscaleDelay:   makeDurationPointer(1 * time.Minute),
							},
						},
					},
					Networking: core.Networking{
						Type: "some-network-plugin",
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
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.provider.workers"),
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

		DescribeTable("purpose validation",
			func(purpose core.ShootPurpose, namespace string, matcher gomegatypes.GomegaMatcher) {
				shootCopy := shoot.DeepCopy()
				shootCopy.Namespace = namespace
				shootCopy.Spec.Purpose = &purpose
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

		It("should forbid unsupported addon configuration", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer("does-not-exist")

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
			}))))
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
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(core.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = makeBoolPointer(false)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
			}))))
		})

		It("should allow using basic auth mode for kubernetes dashboard when it's enabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = makeStringPointer(core.KubernetesDashboardAuthModeBasic)
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
			))
		})

		It("should forbid updating the seed, if it has been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.SeedName = makeStringPointer("another-seed")
			shoot.Spec.SeedName = makeStringPointer("first-seed")

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedName"),
				}))),
			)
		})

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

		It("should allow updating the seed if it has not been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.SeedName = makeStringPointer("another-seed")
			shoot.Spec.SeedName = nil

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(HaveLen(0))
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

			Context("CIDR", func() {
				It("should invalid k8s networks", func() {
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
			})

			It("should forbid non canonical CIDRs", func() {
				nodeCIDR := "10.250.0.3/16"
				podCIDR := "100.96.0.4/11"
				serviceCIDR := "100.64.0.5/13"

				shoot.Spec.Networking.Nodes = &nodeCIDR
				shoot.Spec.Networking.Services = &serviceCIDR
				shoot.Spec.Networking.Pods = &podCIDR

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

		It("should forbid an empty worker list", func() {
			shoot.Spec.Provider.Workers = []core.Worker{}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.provider.workers"),
			}))))
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
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.workers[0].name"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("spec.provider.workers[0].machine.type"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.workers[0].minimum"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.provider.workers[0].maximum"),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.provider.workers[0].maximum"),
				})),
			))
		})

		It("should enforce workers min > 0 if max > 0", func() {
			shoot.Spec.Provider.Workers = []core.Worker{workerAutoScalingInvalid, worker}

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeForbidden),
				"Field": Equal("spec.provider.workers[0].minimum"),
			}))))
		})

		It("should allow workers having min=max=0 if at least one pool is active", func() {
			shoot.Spec.Provider.Workers = []core.Worker{worker, workerAutoScalingMinMaxZero}

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

		Context("NodeCIDRMask validation", func() {
			var (
				defaultMaxPod           int32 = 110
				maxPod                  int32 = 260
				defaultNodeCIDRMaskSize int32 = 24
				testWorker              core.Worker
			)

			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = &defaultNodeCIDRMaskSize
				shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{MaxPods: &defaultMaxPod}
				testWorker = *worker.DeepCopy()
				testWorker.Name = "testworker"
			})

			It("should not return any errors", func() {
				worker.Kubernetes = &core.WorkerKubernetes{
					Kubelet: &core.KubeletConfig{
						MaxPods: &defaultMaxPod,
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			Context("Non-default max pod settings", func() {
				Context("one worker pool", func() {
					It("should deny NodeCIDR with too few ips", func() {
						testWorker.Kubernetes = &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								MaxPods: &maxPod,
							},
						}
						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, testWorker)

						errorList := ValidateShoot(shoot)

						Expect(errorList).To(ConsistOfFields(Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
							"Detail": ContainSubstring("kubelet or kube-controller configuration incorrect"),
						}))
					})
				})

				Context("multiple worker pools", func() {
					It("should deny NodeCIDR with too few ips", func() {
						testWorker.Kubernetes = &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								MaxPods: &maxPod,
							},
						}

						secondTestWorker := *testWorker.DeepCopy()
						secondTestWorker.Name = "testworker2"
						secondTestWorker.Kubernetes = &core.WorkerKubernetes{
							Kubelet: &core.KubeletConfig{
								MaxPods: &maxPod,
							},
						}

						shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, testWorker, secondTestWorker)

						errorList := ValidateShoot(shoot)

						Expect(errorList).To(ConsistOfFields(Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
							"Detail": ContainSubstring("kubelet or kube-controller configuration incorrect"),
						}))
					})
				})

				Context("Global default max pod", func() {
					It("should deny NodeCIDR with too few ips", func() {
						shoot.Spec.Kubernetes.Kubelet = &core.KubeletConfig{MaxPods: &maxPod}

						errorList := ValidateShoot(shoot)

						Expect(errorList).To(ConsistOfFields(Fields{
							"Type":   Equal(field.ErrorTypeInvalid),
							"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
							"Detail": ContainSubstring("kubelet or kube-controller configuration incorrect"),
						}))
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
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type: makeStringPointer(core.DNSUnmanaged),
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
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       makeStringPointer(core.DNSUnmanaged),
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
				shoot.Spec.DNS.Providers = []core.DNSProvider{
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
				newShoot.Spec.DNS = &core.DNS{
					Domain: makeStringPointer("some-domain.com"),
				}

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow assigning the dns domain (dns non-nil)", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.DNS = &core.DNS{}
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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(22)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})

			It("should succeed not changing immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(24)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should fail when nodeCIDRMaskSize is out of upper boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(32)

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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(0)

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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = makeInt32Pointer(22)

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("KubeProxy validation", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeProxy = &core.KubeProxyConfig{}
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

			It("should fail when using kubernetes version 1.14.2 and proxy mode is changed", func() {
				mode := core.ProxyMode("IPVS")
				kubernetesConfig := core.KubernetesConfig{}
				config := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &mode,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				shoot.Spec.Kubernetes.Version = "1.14.2"
				oldMode := core.ProxyMode("IPTables")
				oldConfig := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &oldMode,
				}
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, false, field.NewPath("spec"))

				Expect(errorList).ToNot(BeEmpty())
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeProxy.mode"),
					"Detail": Equal(`field is immutable`),
				}))
			})

			It("should be successful when using kubernetes version 1.14.1 and proxy mode stays the same", func() {
				mode := core.ProxyMode("IPVS")
				shoot.Spec.Kubernetes.Version = "1.14.1"
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(2))
			})
		})

		Context("ClusterAutoscaler validation", func() {
			DescribeTable("cluster autoscaler values",
				func(clusterAutoscaler core.ClusterAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateClusterAutoscaler(clusterAutoscaler, nil)).To(matcher)
				},
				Entry("valid", core.ClusterAutoscaler{}, BeEmpty()),
				Entry("valid with threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(0.5),
				}, BeEmpty()),
				Entry("invalid negative threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: makeFloat64Pointer(-0.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
				Entry("invalid > 1 threshold", core.ClusterAutoscaler{
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
			shoot.DeletionTimestamp = &deletionTimestamp
			newShoot.DeletionTimestamp = &deletionTimestamp
			newShoot.Spec.Maintenance.AutoUpdate.KubernetesVersion = false

			errorList := ValidateShootUpdate(newShoot, shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec"),
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

			Expect(errorList).To(HaveLen(0))
		})
	})

	Describe("#ValidateShootStatus, #ValidateShootStatusUpdate", func() {
		var shoot *core.Shoot
		BeforeEach(func() {
			shoot = &core.Shoot{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "shoot",
					Namespace: "my-namespace",
				},
				Spec:   core.ShootSpec{},
				Status: core.ShootStatus{},
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
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
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
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
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
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
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
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
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
		var (
			zero int32 = 0
			one  int32 = 1
		)

		DescribeTable("validate that at least one active worker pool is configured",
			func(min1, max1, min2, max2 int32, matcher gomegatypes.GomegaMatcher) {
				workers := []core.Worker{
					{
						Name:    "one",
						Minimum: min1,
						Maximum: max1,
					},
					{
						Name:    "two",
						Minimum: min2,
						Maximum: max2,
					},
				}

				errList := ValidateWorkers(workers, field.NewPath("workers"))

				Expect(errList).To(matcher)
			},

			Entry("at least one worker pool min>0, max>0", zero, zero, one, one, HaveLen(0)),
			Entry("all worker pools min=max=0", zero, zero, zero, zero, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
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
				kubeletConfig := core.KubeletConfig{
					EvictionMinimumReclaim: &core.KubeletConfigEvictionMinimumReclaim{
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
				kubeletConfig := core.KubeletConfig{
					EvictionSoftGracePeriod: &core.KubeletConfigEvictionSoftGracePeriod{
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
				kubeletConfig := core.KubeletConfig{
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
				})),
			)),
		)

		DescribeTable("validate the kubelet configuration - EvictionMaxPodGracePeriod",
			func(evictionMaxPodGracePeriod int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
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
				})),
			)),
		)

		DescribeTable("validate the kubelet configuration - MaxPods",
			func(maxPods int32, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
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
				})),
			)),
		)
	})

	Describe("#ValidateHibernationSchedules", func() {
		DescribeTable("validate hibernation schedules",
			func(schedules []core.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				Expect(ValidateHibernationSchedules(schedules, nil)).To(matcher)
			},
			Entry("valid schedules", []core.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}}, BeEmpty()),
			Entry("nil schedules", nil, BeEmpty()),
			Entry("duplicate start and end value in same schedule",
				[]core.HibernationSchedule{{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("duplicate start and end value in different schedules",
				[]core.HibernationSchedule{{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, {Start: makeStringPointer("1 * * * *"), End: makeStringPointer("3 * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("invalid schedule",
				[]core.HibernationSchedule{{Start: makeStringPointer("foo"), End: makeStringPointer("* * * * *")}},
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
			func(seenSpecs sets.String, schedule *core.HibernationSchedule, matcher gomegatypes.GomegaMatcher) {
				errList := ValidateHibernationSchedule(seenSpecs, schedule, nil)
				Expect(errList).To(matcher)
			},

			Entry("valid schedule", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *")}, BeEmpty()),
			Entry("invalid start value", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("start").String()),
			})))),
			Entry("invalid end value", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("invalid location", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer("1 * * * *"), End: makeStringPointer("2 * * * *"), Location: makeStringPointer("foo")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("location").String()),
			})))),
			Entry("equal start and end value", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer("* * * * *"), End: makeStringPointer("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("nil start", sets.NewString(), &core.HibernationSchedule{End: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("nil end", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer("* * * * *")}, BeEmpty()),
			Entry("start and end nil", sets.NewString(), &core.HibernationSchedule{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeRequired),
				})))),
			Entry("invalid start and end value", sets.NewString(), &core.HibernationSchedule{Start: makeStringPointer(""), End: makeStringPointer("")},
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

func prepareShootForUpdate(shoot *core.Shoot) *core.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
