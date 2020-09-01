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
	"strconv"
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
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	apivalidation "k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("Shoot Validation Tests", func() {
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

			maxSurge         = intstr.FromInt(1)
			maxUnavailable   = intstr.FromInt(0)
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
				},
				Minimum:          1,
				Maximum:          1,
				MaxSurge:         &maxSurge,
				MaxUnavailable:   &maxUnavailable,
				SystemComponents: systemComponents,
			}
			invalidWorker = core.Worker{
				Name: "",
				Machine: core.Machine{
					Type: "",
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
					CloudProfileName:  "aws-profile",
					Region:            "eu-west-1",
					SecretBindingName: "my-secret",
					Purpose:           &purpose,
					DNS: &core.DNS{
						Providers: []core.DNSProvider{
							{
								Type:    &dnsProviderType,
								Primary: pointer.BoolPtr(true),
							},
						},
						Domain: &domain,
					},
					Kubernetes: core.Kubernetes{
						Version: "1.11.2",
						KubeAPIServer: &core.KubeAPIServerConfig{
							OIDCConfig: &core.OIDCConfig{
								CABundle:       pointer.StringPtr("-----BEGIN CERTIFICATE-----\nMIICRzCCAfGgAwIBAgIJALMb7ecMIk3MMA0GCSqGSIb3DQEBCwUAMH4xCzAJBgNV\nBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNVBAcMBkxvbmRvbjEYMBYGA1UE\nCgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1JVCBEZXBhcnRtZW50MRswGQYD\nVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwIBcNMTcwNDI2MjMyNjUyWhgPMjExNzA0\nMDIyMzI2NTJaMH4xCzAJBgNVBAYTAkdCMQ8wDQYDVQQIDAZMb25kb24xDzANBgNV\nBAcMBkxvbmRvbjEYMBYGA1UECgwPR2xvYmFsIFNlY3VyaXR5MRYwFAYDVQQLDA1J\nVCBEZXBhcnRtZW50MRswGQYDVQQDDBJ0ZXN0LWNlcnRpZmljYXRlLTAwXDANBgkq\nhkiG9w0BAQEFAANLADBIAkEAtBMa7NWpv3BVlKTCPGO/LEsguKqWHBtKzweMY2CV\ntAL1rQm913huhxF9w+ai76KQ3MHK5IVnLJjYYA5MzP2H5QIDAQABo1AwTjAdBgNV\nHQ4EFgQU22iy8aWkNSxv0nBxFxerfsvnZVMwHwYDVR0jBBgwFoAU22iy8aWkNSxv\n0nBxFxerfsvnZVMwDAYDVR0TBAUwAwEB/zANBgkqhkiG9w0BAQsFAANBAEOefGbV\nNcHxklaW06w6OBYJPwpIhCVozC1qdxGX1dg8VkEKzjOzjgqVD30m59OFmSlBmHsl\nnkVA6wyOSDYBf3o=\n-----END CERTIFICATE-----"),
								ClientID:       pointer.StringPtr("client-id"),
								GroupsClaim:    pointer.StringPtr("groups-claim"),
								GroupsPrefix:   pointer.StringPtr("groups-prefix"),
								IssuerURL:      pointer.StringPtr("https://some-endpoint.com"),
								UsernameClaim:  pointer.StringPtr("user-claim"),
								UsernamePrefix: pointer.StringPtr("user-prefix"),
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
							EnableBasicAuthentication: pointer.BoolPtr(true),
						},
						KubeControllerManager: &core.KubeControllerManagerConfig{
							NodeCIDRMaskSize: pointer.Int32Ptr(22),
							HorizontalPodAutoscalerConfig: &core.HorizontalPodAutoscalerConfig{
								DownscaleDelay: makeDurationPointer(15 * time.Minute),
								SyncPeriod:     makeDurationPointer(30 * time.Second),
								Tolerance:      pointer.Float64Ptr(0.1),
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
					Tolerations: []core.Toleration{
						{Key: "foo"},
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
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = pointer.StringPtr("does-not-exist")

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
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = pointer.StringPtr(core.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.BoolPtr(false)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal("spec.addons.kubernetes-dashboard.authenticationMode"),
			}))))
		})

		It("should allow using basic auth mode for kubernetes dashboard when it's enabled in kube-apiserver config", func() {
			shoot.Spec.Addons.KubernetesDashboard.AuthenticationMode = pointer.StringPtr(core.KubernetesDashboardAuthModeBasic)
			shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.BoolPtr(true)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should forbid unsupported specification (provider independent)", func() {
			shoot.Spec.CloudProfileName = ""
			shoot.Spec.Region = ""
			shoot.Spec.SecretBindingName = ""
			shoot.Spec.SeedName = pointer.StringPtr("")
			shoot.Spec.SeedSelector = &core.SeedSelector{
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"foo": "no/slash/allowed"}},
			}
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
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.seedSelector.matchLabels"),
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

		It("should forbid invalid tolerations", func() {
			shoot.Spec.Tolerations = []core.Toleration{
				{},
				{Key: "foo"},
				{Key: "foo"},
				{Key: "bar", Value: pointer.StringPtr("baz")},
				{Key: "bar", Value: pointer.StringPtr("baz")},
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
			newShoot.Spec.SeedName = pointer.StringPtr("another-seed")
			shoot.Spec.SeedName = pointer.StringPtr("first-seed")

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
			shoot.Spec.Resources = append(shoot.Spec.Resources, ref)

			errorList := ValidateShoot(shoot)

			Expect(errorList).To(BeEmpty())
		})

		It("should allow updating the seed if it has not been set previously", func() {
			newShoot := prepareShootForUpdate(shoot)
			newShoot.Spec.SeedName = pointer.StringPtr("another-seed")
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

			It("should forbid an empty worker list", func() {
				shoot.Spec.Provider.Workers = []core.Worker{}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.provider.workers"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeForbidden),
						"Field": Equal("spec.provider.workers"),
					})),
				))
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
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("spec.provider.workers[0].machine.image"),
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

			It("should allow adding a worker pool", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should allow removing a worker pool", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should allow swapping worker pools", func() {
				newShoot := prepareShootForUpdate(shoot)

				worker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				worker.Name = "second-worker"

				newShoot.Spec.Provider.Workers = append(newShoot.Spec.Provider.Workers, worker)
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, worker)

				newShoot.Spec.Provider.Workers = []core.Worker{newShoot.Spec.Provider.Workers[1], newShoot.Spec.Provider.Workers[0]}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should not allow update cri configurations enablement", func() {
				newShoot := prepareShootForUpdate(shoot)
				newWorker := *shoot.Spec.Provider.Workers[0].DeepCopy()
				newWorker.Name = "second-worker"
				newWorker.CRI = &core.CRI{Name: core.CRINameContainerD}
				shoot.Spec.Provider.Workers = append(shoot.Spec.Provider.Workers, newWorker)

				newShoot.Spec.Provider.Workers = []core.Worker{newWorker, shoot.Spec.Provider.Workers[0]}
				newShoot.Spec.Provider.Workers[0].CRI = nil
				newShoot.Spec.Provider.Workers[1].CRI = &core.CRI{Name: core.CRINameContainerD}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(2))
			})

			It("should not allow update cri name", func() {
				shoot.Spec.Provider.Workers[0].CRI = &core.CRI{Name: "test-cri"}
				newShoot := prepareShootForUpdate(shoot)

				newShoot.Spec.Provider.Workers[0].CRI = &core.CRI{Name: core.CRINameContainerD}

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(1))
			})
		})

		Context("dns section", func() {
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
						Type:    pointer.StringPtr(core.DNSUnmanaged),
						Primary: pointer.BoolPtr(true),
					},
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid specifying invalid domain", func() {
				shoot.Spec.DNS.Providers = nil
				shoot.Spec.DNS.Domain = pointer.StringPtr("foo/bar.baz")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should forbid specifying a secret name when provider equals 'unmanaged'", func() {
				shoot.Spec.DNS.Providers = []core.DNSProvider{
					{
						Type:       pointer.StringPtr(core.DNSUnmanaged),
						SecretName: pointer.StringPtr(""),
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
						SecretName: pointer.StringPtr(""),
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
					Domain: pointer.StringPtr("some-domain.com"),
				}

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should allow assigning the dns domain (dns non-nil)", func() {
				oldShoot := prepareShootForUpdate(shoot)
				oldShoot.Spec.DNS = &core.DNS{}
				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.DNS.Domain = pointer.StringPtr("some-domain.com")

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
				newShoot.Spec.DNS.Domain = pointer.StringPtr("another-domain.com")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.dns.domain"),
				}))))
			})

			It("should allow updating the dns providers if seed is assigned", func() {
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.SeedName = nil
				oldShoot.Spec.DNS.Providers[0].Type = pointer.StringPtr("some-dns-provider")

				newShoot := prepareShootForUpdate(oldShoot)
				newShoot.Spec.SeedName = pointer.StringPtr("seed")
				newShoot.Spec.DNS.Providers = nil

				errorList := ValidateShootUpdate(newShoot, oldShoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should forbid updating the primary dns provider type", func() {
				newShoot := prepareShootForUpdate(shoot)
				shoot.Spec.DNS.Providers[0].Type = pointer.StringPtr("some-other-provider")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid to unset the primary DNS provider type", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Providers[0].Type = nil

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid to remove the primary DNS provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Providers[0].Primary = pointer.BoolPtr(false)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers"),
				}))))
			})

			It("should forbid adding another primary provider", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.DNS.Providers = append(newShoot.Spec.DNS.Providers, core.DNSProvider{
					Primary: pointer.BoolPtr(true),
				})

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers[1].primary"),
				}))))
			})

			It("should having the a provider with invalid secretName", func() {
				var (
					invalidSecretName = "foo/bar"
				)

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

			It("should having the same provider multiple times", func() {

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
				newShoot.Spec.DNS.Providers[0].SecretName = pointer.StringPtr("my-dns-secret")

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid having more than one primary provider", func() {
				shoot.Spec.DNS.Providers = append(shoot.Spec.DNS.Providers, core.DNSProvider{
					Primary: pointer.BoolPtr(true),
				})

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.dns.providers[1].primary"),
				}))))
			})
		})

		Context("OIDC validation", func() {
			It("should forbid unsupported OIDC configuration", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.CABundle = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsClaim = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.GroupsPrefix = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernameClaim = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.UsernamePrefix = pointer.StringPtr("")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{}
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.SigningAlgs = []string{}

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

			DescribeTable("should forbid issuerURL to be empty string or nil, if clientID exists ", func(errorListSize int, issuerURL *string) {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = pointer.StringPtr("someClientID")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = issuerURL

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(errorListSize))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
				}))
			},
				Entry("should add error if clientID is set but issuerURL is nil ", 1, nil),
				Entry("should add error if clientID is set but issuerURL is empty string", 2, pointer.StringPtr("")),
			)

			It("should forbid issuerURL which is not HTTPS schema", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = pointer.StringPtr("http://issuer.com")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = pointer.StringPtr("someClientID")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(1))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.issuerURL"),
				}))
			})

			It("should not fail if both clientID and issuerURL are set", func() {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = pointer.StringPtr("https://issuer.com")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = pointer.StringPtr("someClientID")

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(BeEmpty())
			})

			DescribeTable("should forbid clientID to be empty string or nil, if issuerURL exists ", func(errorListSize int, clientID *string) {
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.IssuerURL = pointer.StringPtr("https://issuer.com")
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.ClientID = clientID

				errorList := ValidateShoot(shoot)
				Expect(errorList).To(HaveLen(errorListSize))
				Expect(*errorList[0]).To(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("spec.kubernetes.kubeAPIServer.oidcConfig.clientID"),
				}))
			},
				Entry("should add error if issuerURL is set but clientID is nil", 1, nil),
				Entry("should add error if issuerURL is set but clientID is empty string ", 2, pointer.StringPtr("")),
			)

			It("should allow supported OIDC configuration (for K8S >= v1.11)", func() {
				shoot.Spec.Kubernetes.Version = "1.11.1"
				shoot.Spec.Kubernetes.KubeAPIServer.OIDCConfig.RequiredClaims = map[string]string{
					"some": "claim",
				}

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})
		})

		Context("basic authentication", func() {
			BeforeEach(func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.DownscaleDelay = nil
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.UpscaleDelay = nil
			})

			It("should allow basic authentication when kubernetes <= 1.18", func() {
				shoot.Spec.Kubernetes.Version = "1.18.1"
				shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.BoolPtr(true)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(HaveLen(0))
			})

			It("should forbid basic authentication when kubernetes >= 1.19", func() {
				shoot.Spec.Kubernetes.Version = "1.19.1"
				shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.BoolPtr(true)

				errorList := ValidateShoot(shoot)

				Expect(errorList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeForbidden),
					"Field": Equal("spec.kubernetes.kubeAPIServer.enableBasicAuthentication"),
				}))))
			})

			It("should allow disabling basic authentication when kubernetes >= 1.19", func() {
				shoot.Spec.Kubernetes.Version = "1.19.1"
				shoot.Spec.Kubernetes.KubeAPIServer.EnableBasicAuthentication = pointer.BoolPtr(false)

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
					Default: pointer.Int32Ptr(0),
				}, BeEmpty()),
				Entry("valid (default>0)", &core.WatchCacheSizes{
					Default: pointer.Int32Ptr(42),
				}, BeEmpty()),
				Entry("invalid (default<0)", &core.WatchCacheSizes{
					Default: pointer.Int32Ptr(negativeSize),
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
						APIGroup:  pointer.StringPtr("apps"),
						Resource:  "deployments",
						CacheSize: 0,
					}},
				}, BeEmpty()),
				Entry("valid (apps/deployments=>0)", &core.WatchCacheSizes{
					Resources: []core.ResourceWatchCacheSize{{
						APIGroup:  pointer.StringPtr("apps"),
						Resource:  "deployments",
						CacheSize: 42,
					}},
				}, BeEmpty()),
				Entry("invalid (apps/deployments=<0)", &core.WatchCacheSizes{
					Resources: []core.ResourceWatchCacheSize{{
						APIGroup:  pointer.StringPtr("apps"),
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

		Context("KubeControllerManager validation < 1.12", func() {
			It("should forbid unsupported HPA configuration", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.SyncPeriod = makeDurationPointer(100 * time.Millisecond)
				shoot.Spec.Kubernetes.KubeControllerManager.HorizontalPodAutoscalerConfig.Tolerance = pointer.Float64Ptr(0)
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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(22)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("spec.kubernetes.kubeControllerManager.nodeCIDRMaskSize"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})

			It("should succeed not changing immutable fields", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(24)

				newShoot := prepareShootForUpdate(shoot)
				newShoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(24)

				errorList := ValidateShootUpdate(newShoot, shoot)

				Expect(errorList).To(BeEmpty())
			})

			It("should fail when nodeCIDRMaskSize is out of upper boundary", func() {
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(32)

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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(0)

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
				shoot.Spec.Kubernetes.KubeControllerManager.NodeCIDRMaskSize = pointer.Int32Ptr(22)

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

			It("should be successful when using kubernetes version 1.16.1 and proxy mode is changed", func() {
				mode := core.ProxyMode("IPVS")
				kubernetesConfig := core.KubernetesConfig{}
				config := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &mode,
				}
				shoot.Spec.Kubernetes.KubeProxy = &config
				shoot.Spec.Kubernetes.Version = "1.16.1"
				oldMode := core.ProxyMode("IPTables")
				oldConfig := core.KubeProxyConfig{
					KubernetesConfig: kubernetesConfig,
					Mode:             &oldMode,
				}
				shoot.Spec.Kubernetes.KubeProxy.Mode = &mode
				oldShoot := shoot.DeepCopy()
				oldShoot.Spec.Kubernetes.KubeProxy = &oldConfig

				errorList := ValidateShootSpecUpdate(&shoot.Spec, &oldShoot.Spec, false, field.NewPath("spec"))
				Expect(errorList).To(BeEmpty())
			})
		})

		Context("ClusterAutoscaler validation", func() {
			DescribeTable("cluster autoscaler values",
				func(clusterAutoscaler core.ClusterAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateClusterAutoscaler(clusterAutoscaler, nil)).To(matcher)
				},
				Entry("valid", core.ClusterAutoscaler{}, BeEmpty()),
				Entry("valid with threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: pointer.Float64Ptr(0.5),
				}, BeEmpty()),
				Entry("invalid negative threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: pointer.Float64Ptr(-0.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), -0.5, "can not be negative"))),
				Entry("invalid > 1 threshold", core.ClusterAutoscaler{
					ScaleDownUtilizationThreshold: pointer.Float64Ptr(1.5),
				}, ConsistOf(field.Invalid(field.NewPath("scaleDownUtilizationThreshold"), 1.5, "can not be greater than 1.0"))),
			)
		})

		var negativeDuration = metav1.Duration{Duration: -time.Second}

		Context("VerticalPodAutoscaler validation", func() {
			DescribeTable("verticalPod autoscaler values",
				func(verticalPodAutoscaler core.VerticalPodAutoscaler, matcher gomegatypes.GomegaMatcher) {
					Expect(ValidateVerticalPodAutoscaler(verticalPodAutoscaler, nil)).To(matcher)
				},
				Entry("valid", core.VerticalPodAutoscaler{}, BeEmpty()),
				Entry("invalid negative durations", core.VerticalPodAutoscaler{
					EvictAfterOOMThreshold: &negativeDuration,
					UpdaterInterval:        &negativeDuration,
					RecommenderInterval:    &negativeDuration,
				}, ConsistOf(
					field.Invalid(field.NewPath("evictAfterOOMThreshold"), negativeDuration, "can not be negative"),
					field.Invalid(field.NewPath("updaterInterval"), negativeDuration, "can not be negative"),
					field.Invalid(field.NewPath("recommenderInterval"), negativeDuration, "can not be negative"),
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

		Context("validate shoot cluster identity update", func() {
			var clusterIdentity = "newClusterIdentity"
			It("should not fail to set the cluster identity if it is missing", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.ClusterIdentity = &clusterIdentity
				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(0))
			})

			It("should fail to set the cluster identity if it is already set", func() {
				newShoot := prepareShootForUpdate(shoot)
				newShoot.Status.ClusterIdentity = &clusterIdentity
				shoot.Status.ClusterIdentity = pointer.StringPtr("oldClusterIdentity")
				errorList := ValidateShootStatusUpdate(newShoot.Status, shoot.Status)
				Expect(errorList).To(HaveLen(1))
				Expect(errorList).To(ConsistOfFields(Fields{
					"Type":   Equal(field.ErrorTypeInvalid),
					"Field":  Equal("status.clusterIdentity"),
					"Detail": ContainSubstring(`field is immutable`),
				}))
			})
		})
	})

	Describe("#ValidateWorker", func() {
		DescribeTable("validate worker machine",
			func(machine core.Machine, matcher gomegatypes.GomegaMatcher) {
				maxSurge := intstr.FromInt(1)
				maxUnavailable := intstr.FromInt(0)
				worker := core.Worker{
					Name:           "worker-name",
					Machine:        machine,
					MaxSurge:       &maxSurge,
					MaxUnavailable: &maxUnavailable,
				}
				errList := ValidateWorker(worker, nil)

				Expect(errList).To(matcher)
			},

			Entry("empty machine type",
				core.Machine{
					Type: "",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.type"),
				}))),
			),
			Entry("missing machine image",
				core.Machine{
					Type: "large",
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.image"),
				}))),
			),
			Entry("empty machine image name",
				core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "",
						Version: "1.0.0",
					},
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
				},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeRequired),
					"Field": Equal("machine.image.version"),
				}))),
			),
		)

		DescribeTable("reject when maxUnavailable and maxSurge are invalid",
			func(maxUnavailable, maxSurge intstr.IntOrString, expectType field.ErrorType) {
				worker := core.Worker{
					Name: "worker-name",
					Machine: core.Machine{
						Type: "large",
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
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
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
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
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
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
						Image: &core.ShootMachineImage{
							Name:    "image-name",
							Version: "1.0.0",
						},
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

		It("should reject if volume is undefined and data volumes are defined", func() {
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
			dataVolumes := []core.DataVolume{{Name: "vol1-name", VolumeSize: "75Gi"}}
			worker := core.Worker{
				Name: "worker-name",
				Machine: core.Machine{
					Type: "large",
					Image: &core.ShootMachineImage{
						Name:    "image-name",
						Version: "1.0.0",
					},
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, nil)
			Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeRequired),
				"Field": Equal("volume"),
			}))))
		})

		It("should reject if data volume size does not match size regex", func() {
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
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
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, nil)
			Expect(errList).To(ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":     Equal(field.ErrorTypeInvalid),
				"Field":    Equal("dataVolumes[1].size"),
				"BadValue": Equal("12MiB"),
			}))))
		})

		It("should reject if data volume name is invalid", func() {
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
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
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, nil)
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
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
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
				},
				MaxSurge:              &maxSurge,
				MaxUnavailable:        &maxUnavailable,
				Volume:                &vol,
				DataVolumes:           dataVolumes,
				KubeletDataVolumeName: &name,
			}
			errList := ValidateWorker(worker, nil)
			Expect(errList).To(ConsistOf())
		})

		It("should reject if kubeletDataVolumeName refers to undefined data volume", func() {
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
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
				},
				MaxSurge:              &maxSurge,
				MaxUnavailable:        &maxUnavailable,
				Volume:                &vol,
				DataVolumes:           dataVolumes,
				KubeletDataVolumeName: &name3,
			}
			errList := ValidateWorker(worker, nil)
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal("kubeletDataVolumeName"),
				})),
			))
		})

		It("should reject if data volume names are duplicated", func() {
			maxSurge := intstr.FromInt(1)
			maxUnavailable := intstr.FromInt(0)
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
				},
				MaxSurge:       &maxSurge,
				MaxUnavailable: &maxUnavailable,
				Volume:         &vol,
				DataVolumes:    dataVolumes,
			}
			errList := ValidateWorker(worker, nil)
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

		DescribeTable("validate that CRI name is valid",
			func(name core.CRIName, matcher gomegatypes.GomegaMatcher) {
				worker := core.Worker{
					Name: "worker",
					CRI:  &core.CRI{Name: name}}

				errList := ValidateCRI(worker.CRI, field.NewPath("cri"))

				Expect(errList).To(matcher)
			},

			Entry("valid CRI name", core.CRINameContainerD, HaveLen(0)),
			Entry("not valid CRI name", core.CRIName("other"), ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeNotSupported),
				"Field": Equal("cri.name"),
			})))),
		)

		It("validate that container runtime has a type", func() {
			worker := core.Worker{
				Name: "worker",
				CRI: &core.CRI{Name: core.CRINameContainerD,
					ContainerRuntimes: []core.ContainerRuntime{{Type: "gVisor"}, {Type: ""}}},
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
				CRI: &core.CRI{Name: core.CRINameContainerD,
					ContainerRuntimes: []core.ContainerRuntime{{Type: "gVisor"}, {Type: "gVisor"}}},
			}

			errList := ValidateCRI(worker.CRI, field.NewPath("cri"))
			Expect(errList).To(ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeDuplicate),
					"Field": Equal("cri.containerruntimes[1].type"),
				})),
			))
		})
	})

	Describe("#ValidateWorkers", func() {
		var (
			zero int32 = 0
			one  int32 = 1
		)

		DescribeTable("validate that at least one active worker pool is configured",
			func(min1, max1, min2, max2 int32, matcher gomegatypes.GomegaMatcher) {
				systemComponents := &core.WorkerSystemComponents{
					Allow: true,
				}
				workers := []core.Worker{
					{
						Name:             "one",
						Minimum:          min1,
						Maximum:          max1,
						SystemComponents: systemComponents,
					},
					{
						Name:             "two",
						Minimum:          min2,
						Maximum:          max2,
						SystemComponents: systemComponents,
					},
				}

				Expect(ValidateWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry("at least one worker pool min>0, max>0", zero, zero, one, one, HaveLen(0)),
			Entry("all worker pools min=max=0", zero, zero, zero, zero, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeForbidden),
				})),
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeForbidden),
				})),
			)),
		)

		DescribeTable("validate that at least one worker pool is able to host system components",
			func(min1, max1, min2, max2 int32, allowSystemComponents1, allowSystemComponents2 bool, taints1, taints2 []corev1.Taint, matcher gomegatypes.GomegaMatcher) {
				workers := []core.Worker{
					{
						Name:    "one-active",
						Minimum: min1,
						Maximum: max1,
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents1,
						},
						Taints: taints1,
					},
					{
						Name:    "two-active",
						Minimum: min2,
						Maximum: max2,
						SystemComponents: &core.WorkerSystemComponents{
							Allow: allowSystemComponents2,
						},
						Taints: taints2,
					},
				}

				Expect(ValidateWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry("all worker pools min=max=0", zero, zero, zero, zero, true, true, nil, nil, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeForbidden),
				})), PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeForbidden),
				})),
			)),
			Entry("at least one worker pool allows system components", zero, zero, one, one, true, true, nil, nil, HaveLen(0)),
			Entry("one active but taints prevent scheduling", one, one, zero, zero, true, true, []corev1.Taint{{Effect: corev1.TaintEffectNoSchedule}}, nil, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeForbidden),
				})),
			)),
		)

		DescribeTable("ensure that at least one worker pool exists that either has no taints or only those with `PreferNoSchedule` effect",
			func(matcher gomegatypes.GomegaMatcher, taints ...[]corev1.Taint) {
				var (
					workers          []core.Worker
					systemComponents = &core.WorkerSystemComponents{
						Allow: true,
					}
				)

				for i, t := range taints {
					workers = append(workers, core.Worker{
						Name:             "pool-" + strconv.Itoa(i),
						Minimum:          1,
						Maximum:          2,
						Taints:           t,
						SystemComponents: systemComponents,
					})
				}

				Expect(ValidateWorkers(workers, field.NewPath("workers"))).To(matcher)
			},

			Entry(
				"no pools with taints",
				HaveLen(0),
				[]corev1.Taint{},
			),
			Entry(
				"all pools with PreferNoSchedule taints",
				HaveLen(0),
				[]corev1.Taint{{Effect: corev1.TaintEffectPreferNoSchedule}},
			),
			Entry(
				"at least one pools with either no or PreferNoSchedule taints (1)",
				HaveLen(0),
				[]corev1.Taint{{Effect: corev1.TaintEffectNoExecute}},
				[]corev1.Taint{{Effect: corev1.TaintEffectPreferNoSchedule}},
			),
			Entry(
				"at least one pools with either no or PreferNoSchedule taints (2)",
				HaveLen(0),
				[]corev1.Taint{{Effect: corev1.TaintEffectNoSchedule}},
				[]corev1.Taint{{Effect: corev1.TaintEffectPreferNoSchedule}},
			),
			Entry(
				"all pools with NoSchedule taints",
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
				),
				[]corev1.Taint{{Effect: corev1.TaintEffectNoSchedule}},
			),
			Entry(
				"all pools with NoExecute taints",
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
				),
				[]corev1.Taint{{Effect: corev1.TaintEffectNoExecute}},
			),
			Entry(
				"all pools with either NoSchedule or NoExecute taints",
				ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type": Equal(field.ErrorTypeForbidden),
					})),
				),
				[]corev1.Taint{{Effect: corev1.TaintEffectNoExecute}},
				[]corev1.Taint{{Effect: corev1.TaintEffectNoSchedule}},
			),
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

		Describe("pod pids limits", func() {
			It("should ensure pod pids limits are non-negative", func() {
				var podPIDsLimit int64 = -1
				kubeletConfig := core.KubeletConfig{
					PodPIDsLimit: &podPIDsLimit,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

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

				errList := ValidateKubeletConfig(kubeletConfig, nil)

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

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(BeEmpty())
			})
		})

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

		Context("validate the kubelet configuration - reserved", func() {

			DescribeTable("validate the kubelet configuration - KubeReserved",
				func(cpu, memory, epehemeralStorage, pid resource.Quantity, matcher gomegatypes.GomegaMatcher) {
					kubeletConfig := core.KubeletConfig{
						KubeReserved: &core.KubeletConfigReserved{
							CPU:              &cpu,
							Memory:           &memory,
							EphemeralStorage: &epehemeralStorage,
							PID:              &pid,
						},
					}

					errList := ValidateKubeletConfig(kubeletConfig, nil)

					Expect(errList).To(matcher)
				},

				Entry("valid configuration", validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, HaveLen(0)),
				Entry("only allow positive resource.Quantity for any value", resource.MustParse(invalidResourceQuantityValue), validResourceQuantity, validResourceQuantity, validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("kubeReserved.cpu").String()),
				})))),
			)

			DescribeTable("validate the kubelet configuration - SystemReserved",
				func(cpu, memory, epehemeralStorage, pid resource.Quantity, matcher gomegatypes.GomegaMatcher) {
					kubeletConfig := core.KubeletConfig{
						SystemReserved: &core.KubeletConfigReserved{
							CPU:              &cpu,
							Memory:           &memory,
							EphemeralStorage: &epehemeralStorage,
							PID:              &pid,
						},
					}

					errList := ValidateKubeletConfig(kubeletConfig, nil)

					Expect(errList).To(matcher)
				},

				Entry("valid configuration", validResourceQuantity, validResourceQuantity, validResourceQuantity, validResourceQuantity, HaveLen(0)),
				Entry("only allow positive resource.Quantity for any value", resource.MustParse(invalidResourceQuantityValue), validResourceQuantity, validResourceQuantity, validResourceQuantity, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("systemReserved.cpu").String()),
				})))),
			)
		})

		DescribeTable("validate the kubelet configuration - ImagePullProgressDeadline",
			func(imagePullProgressDeadline metav1.Duration, matcher gomegatypes.GomegaMatcher) {
				kubeletConfig := core.KubeletConfig{
					ImagePullProgressDeadline: &imagePullProgressDeadline,
				}

				errList := ValidateKubeletConfig(kubeletConfig, nil)

				Expect(errList).To(matcher)
			},

			Entry("valid configuration", validDuration, HaveLen(0)),
			Entry("only allow positive Duration", invalidDuration, ConsistOf(
				PointTo(MatchFields(IgnoreExtras, Fields{
					"Type":  Equal(field.ErrorTypeInvalid),
					"Field": Equal(field.NewPath("imagePullProgressDeadline").String()),
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
			Entry("valid schedules", []core.HibernationSchedule{{Start: pointer.StringPtr("1 * * * *"), End: pointer.StringPtr("2 * * * *")}}, BeEmpty()),
			Entry("nil schedules", nil, BeEmpty()),
			Entry("duplicate start and end value in same schedule",
				[]core.HibernationSchedule{{Start: pointer.StringPtr("* * * * *"), End: pointer.StringPtr("* * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("duplicate start and end value in different schedules",
				[]core.HibernationSchedule{{Start: pointer.StringPtr("1 * * * *"), End: pointer.StringPtr("2 * * * *")}, {Start: pointer.StringPtr("1 * * * *"), End: pointer.StringPtr("3 * * * *")}},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeDuplicate),
				})))),
			Entry("invalid schedule",
				[]core.HibernationSchedule{{Start: pointer.StringPtr("foo"), End: pointer.StringPtr("* * * * *")}},
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

			Entry("valid schedule", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr("1 * * * *"), End: pointer.StringPtr("2 * * * *")}, BeEmpty()),
			Entry("invalid start value", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr(""), End: pointer.StringPtr("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("start").String()),
			})))),
			Entry("invalid end value", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr("* * * * *"), End: pointer.StringPtr("")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("invalid location", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr("1 * * * *"), End: pointer.StringPtr("2 * * * *"), Location: pointer.StringPtr("foo")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeInvalid),
				"Field": Equal(field.NewPath("location").String()),
			})))),
			Entry("equal start and end value", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr("* * * * *"), End: pointer.StringPtr("* * * * *")}, ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
				"Type":  Equal(field.ErrorTypeDuplicate),
				"Field": Equal(field.NewPath("end").String()),
			})))),
			Entry("nil start", sets.NewString(), &core.HibernationSchedule{End: pointer.StringPtr("* * * * *")}, BeEmpty()),
			Entry("nil end", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr("* * * * *")}, BeEmpty()),
			Entry("start and end nil", sets.NewString(), &core.HibernationSchedule{},
				ConsistOf(PointTo(MatchFields(IgnoreExtras, Fields{
					"Type": Equal(field.ErrorTypeRequired),
				})))),
			Entry("invalid start and end value", sets.NewString(), &core.HibernationSchedule{Start: pointer.StringPtr(""), End: pointer.StringPtr("")},
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

func prepareShootForUpdate(shoot *core.Shoot) *core.Shoot {
	s := shoot.DeepCopy()
	s.ResourceVersion = "1"
	return s
}
