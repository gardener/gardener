// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package apis_test

import (
	"encoding/json"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"

	alicloudv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/v1alpha1"
	awsv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/v1alpha1"
	azurev1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/v1alpha1"
	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"
	openstackv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/v1alpha1"
	packetv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-packet/pkg/apis/packet/v1alpha1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var _ = Describe("roundtripper cloudprofile migration", func() {
	scheme := runtime.NewScheme()

	It("should add the conversion funcs to the scheme", func() {
		Expect(scheme.AddConversionFuncs(
			gardencorev1alpha1.Convert_v1alpha1_Shoot_To_garden_Shoot,
			gardencorev1alpha1.Convert_garden_Shoot_To_v1alpha1_Shoot,
			gardenv1beta1.Convert_v1beta1_Shoot_To_garden_Shoot,
			gardenv1beta1.Convert_garden_Shoot_To_v1beta1_Shoot,
		)).NotTo(HaveOccurred())
	})

	var (
		falseVar = false

		cloudProfileName  = "cloudprofile1"
		region            = "eu1"
		secretBindingName = "secretbinding1"
		seedName          = "seed1"

		addonKubernetesDashboardEnabled  = false
		addonKubernetesDashboardAuthMode = "token"
		addonNginxIngressEnabled         = true
		addonNginxIngressLBSourceRanges  = []string{"first", "second"}

		dnsDomain                  = "foo.example.com"
		dnsProvider1DomainsInclude = []string{"example.com"}
		dnsProvider1DomainsExclude = []string{"another-example.com"}
		dnsProvider1SecretName     = "my-credentials"
		dnsProvider1Type           = "provider1"
		dnsProvider1ZonesInclude   = []string{"first-zone"}
		dnsProvider1ZonesExclude   = []string{"second-zone"}
		dnsProvider2DomainsInclude = []string{"yet-another-example.com"}
		dnsProvider2DomainsExclude = []string{"yet-yet-another-example.com"}
		dnsProvider2SecretName     = "my-credentials2"
		dnsProvider2Type           = "provider2"
		dnsProvider2ZonesInclude   = []string{"third-zone"}
		dnsProvider2ZonesExclude   = []string{"fourth-zone"}
		dnsProviderMigrationJSON   = "[{\"Domains\":{\"Include\":[\"" + dnsProvider2DomainsInclude[0] + "\"],\"Exclude\":[\"" + dnsProvider2DomainsExclude[0] + "\"]},\"SecretName\":\"" + dnsProvider2SecretName + "\",\"Type\":\"" + dnsProvider2Type + "\",\"Zones\":{\"Include\":[\"" + dnsProvider2ZonesInclude[0] + "\"],\"Exclude\":[\"" + dnsProvider2ZonesExclude[0] + "\"]}}]"

		extension1Type           = "random-ext-1"
		extension1ProviderConfig = "some-provider-specific-data"

		hibernationEnabled           = false
		hibernationSchedule1Start    = "start-time"
		hibernationSchedule1End      = "end-time"
		hibernationSchedule1Location = "world"

		kubernetesAllowPrivilegedContainers                             = true
		kubernetesClusterAutoscalerScaleDownDelayAfterAdd               = metav1.Duration{Duration: 0}
		kubernetesClusterAutoscalerScaleDownDelayAfterDelete            = metav1.Duration{Duration: 1}
		kubernetesClusterAutoscalerScaleDownDelayAfterFailure           = metav1.Duration{Duration: 2}
		kubernetesClusterAutoscalerScaleDownUnneededTime                = metav1.Duration{Duration: 3}
		kubernetesClusterAutoscalerScaleDownUtilizationThreshold        = 1.15
		kubernetesClusterAutoscalerScanInterval                         = metav1.Duration{Duration: 4}
		kubernetesKubeAPIServerFeatureGates                             = map[string]bool{"foo": true}
		kubernetesKubeAPIServerAdmissionPlugin1Name                     = "admissionplugin1"
		kubernetesKubeAPIServerAdmissionPlugin1ProviderConfig           = "admission-plugin-config"
		kubernetesKubeAPIServerAPIAudiences                             = []string{"aud1,aud2"}
		kubernetesKubeAPIServerAuditPolicyConfigMapRefName              = "my-audit-policy-config"
		kubernetesKubeAPIServerEnableBasicAuthentication                = true
		kubernetesKubeAPIServerOIDCConfigCABundle                       = "some-ca-bundle"
		kubernetesKubeAPIServerOIDCConfigClientID                       = "client-id"
		kubernetesKubeAPIServerOIDCConfigGroupsClaim                    = "groups-claim"
		kubernetesKubeAPIServerOIDCConfigGroupsPrefix                   = "groups-prefix"
		kubernetesKubeAPIServerOIDCConfigIssuerURL                      = "issuer-url"
		kubernetesKubeAPIServerOIDCConfigRequiredClaims                 = map[string]string{"required": "claim"}
		kubernetesKubeAPIServerOIDCConfigSigningAlgs                    = []string{"algo1", "algo2"}
		kubernetesKubeAPIServerOIDCConfigUsernameClaim                  = "username-claim"
		kubernetesKubeAPIServerOIDCConfigUsernamePrefix                 = "username-prefix"
		kubernetesKubeAPIServerRuntimeConfig                            = map[string]bool{"runtime1": false}
		kubernetesKubeAPIServerServiceAccountConfigIssuer               = "sa-issuer"
		kubernetesKubeAPIServerServiceAccountConfigSigningKeySecretName = "my-signing-key-secret"
		kubernetesKubeControllerManagerNodeCIDRMaskSize                 = int32(24)
		kubernetesKubeControllerManagerNodeCIDRMaskSizeInt              = 24
		kubernetesKubeControllerManagerFeatureGates                     = map[string]bool{"kcm": false}
		kubernetesKubeControllerManagerHPAConfigCPUInitializationPeriod = time.Duration(10)
		kubernetesKubeControllerManagerHPAConfigDownscaleDelay          = time.Duration(11)
		kubernetesKubeControllerManagerHPAConfigDownscaleStabilization  = time.Duration(12)
		kubernetesKubeControllerManagerHPAConfigInitialReadinessDelay   = time.Duration(13)
		kubernetesKubeControllerManagerHPAConfigSyncPeriod              = time.Duration(14)
		kubernetesKubeControllerManagerHPAConfigTolerance               = 10.5
		kubernetesKubeControllerManagerHPAConfigUpscaleDelay            = time.Duration(15)
		kubernetesKubeSchedulerFeatureGates                             = map[string]bool{"ks": true}
		kubernetesKubeProxyFeatureGates                                 = map[string]bool{"kubeproxy": true}
		kubernetesKubeletFeatureGates                                   = map[string]bool{"kubelet": true}
		kubernetesKubeletCPUCFSQuota                                    = false
		kubernetesKubeletCPUManagerPolicy                               = "pol"
		kubernetesKubeletEvictionHardImageFSAvailable                   = "some-val1"
		kubernetesKubeletEvictionHardImageFSInodesFree                  = "some-val2"
		kubernetesKubeletEvictionHardMemoryAvailable                    = "some-val3"
		kubernetesKubeletEvictionHardNodeFSAvailable                    = "some-val4"
		kubernetesKubeletEvictionHardNodeFSInodesFree                   = "some-val5"
		kubernetesKubeletEvictionMaxPodGracePeriod                      = int32(4)
		kubernetesKubeletEvictionMinimumReclaimImageFSAvailable         = resource.MustParse("10Gi")
		kubernetesKubeletEvictionMinimumReclaimImageFSInodesFree        = resource.MustParse("11Gi")
		kubernetesKubeletEvictionMinimumReclaimMemoryAvailable          = resource.MustParse("12Gi")
		kubernetesKubeletEvictionMinimumReclaimNodeFSAvailable          = resource.MustParse("13Gi")
		kubernetesKubeletEvictionMinimumReclaimNodeFSInodesFree         = resource.MustParse("14Gi")
		kubernetesKubeletEvictionSoftImageFSAvailable                   = "some-val6"
		kubernetesKubeletEvictionSoftImageFSInodesFree                  = "some-val7"
		kubernetesKubeletEvictionSoftMemoryAvailable                    = "some-val8"
		kubernetesKubeletEvictionSoftNodeFSAvailable                    = "some-val9"
		kubernetesKubeletEvictionSoftNodeFSInodesFree                   = "some-val10"
		kubernetesKubeletEvictionPressureTransitionPeriod               = metav1.Duration{Duration: 100}
		kubernetesKubeletEvictionSoftGracePeriodImageFSAvailable        = metav1.Duration{Duration: 20}
		kubernetesKubeletEvictionSoftGracePeriodImageFSInodesFree       = metav1.Duration{Duration: 21}
		kubernetesKubeletEvictionSoftGracePeriodMemoryAvailable         = metav1.Duration{Duration: 22}
		kubernetesKubeletEvictionSoftGracePeriodNodeFSAvailable         = metav1.Duration{Duration: 23}
		kubernetesKubeletEvictionSoftGracePeriodNodeFSInodesFree        = metav1.Duration{Duration: 24}
		kubernetesKubeletMaxPods                                        = int32(200)
		kubernetesKubeletPodPIDsLimit                                   = int64(300)
		kubernetesVersion                                               = "1.2.3"

		networkingType           = "cni-plugin"
		networkingProviderConfig = "some-cni-config"
		networkingNodesCIDR      = "1.1.1.1/1"
		networkingPodsCIDR       = "2.2.2.2/2"
		networkingServicesCIDR   = "3.3.3.3/3"

		maintenanceAutoUpdateKubernetesVersion   = true
		maintenanceAutoUpdateMachineImageVersion = false
		maintenanceTimeWindowBegin               = "begin-time"
		maintenanceTimeWindowEnd                 = "end-time"

		worker1Annotations = map[string]string{"anno": "tation"}
		worker1CABundle    = "some-worker-ca-bundle"
		workerKubernetes   = &gardencorev1alpha1.WorkerKubernetes{
			Kubelet: &gardencorev1alpha1.KubeletConfig{
				KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
					FeatureGates: kubernetesKubeletFeatureGates,
				},
				CPUCFSQuota:      &kubernetesKubeletCPUCFSQuota,
				CPUManagerPolicy: &kubernetesKubeletCPUManagerPolicy,
				EvictionHard: &gardencorev1alpha1.KubeletConfigEviction{
					ImageFSAvailable:  &kubernetesKubeletEvictionHardImageFSAvailable,
					ImageFSInodesFree: &kubernetesKubeletEvictionHardImageFSInodesFree,
					MemoryAvailable:   &kubernetesKubeletEvictionHardMemoryAvailable,
					NodeFSAvailable:   &kubernetesKubeletEvictionHardNodeFSAvailable,
					NodeFSInodesFree:  &kubernetesKubeletEvictionHardNodeFSInodesFree,
				},
				EvictionMaxPodGracePeriod: &kubernetesKubeletEvictionMaxPodGracePeriod,
				EvictionMinimumReclaim: &gardencorev1alpha1.KubeletConfigEvictionMinimumReclaim{
					ImageFSAvailable:  &kubernetesKubeletEvictionMinimumReclaimImageFSAvailable,
					ImageFSInodesFree: &kubernetesKubeletEvictionMinimumReclaimImageFSInodesFree,
					MemoryAvailable:   &kubernetesKubeletEvictionMinimumReclaimMemoryAvailable,
					NodeFSAvailable:   &kubernetesKubeletEvictionMinimumReclaimNodeFSAvailable,
					NodeFSInodesFree:  &kubernetesKubeletEvictionMinimumReclaimNodeFSInodesFree,
				},
				EvictionPressureTransitionPeriod: &kubernetesKubeletEvictionPressureTransitionPeriod,
				EvictionSoft: &gardencorev1alpha1.KubeletConfigEviction{
					ImageFSAvailable:  &kubernetesKubeletEvictionSoftImageFSAvailable,
					ImageFSInodesFree: &kubernetesKubeletEvictionSoftImageFSInodesFree,
					MemoryAvailable:   &kubernetesKubeletEvictionSoftMemoryAvailable,
					NodeFSAvailable:   &kubernetesKubeletEvictionSoftNodeFSAvailable,
					NodeFSInodesFree:  &kubernetesKubeletEvictionSoftNodeFSInodesFree,
				},
				EvictionSoftGracePeriod: &gardencorev1alpha1.KubeletConfigEvictionSoftGracePeriod{
					ImageFSAvailable:  &kubernetesKubeletEvictionSoftGracePeriodImageFSAvailable,
					ImageFSInodesFree: &kubernetesKubeletEvictionSoftGracePeriodImageFSInodesFree,
					MemoryAvailable:   &kubernetesKubeletEvictionSoftGracePeriodMemoryAvailable,
					NodeFSAvailable:   &kubernetesKubeletEvictionSoftGracePeriodNodeFSAvailable,
					NodeFSInodesFree:  &kubernetesKubeletEvictionSoftGracePeriodNodeFSInodesFree,
				},
				MaxPods:      &kubernetesKubeletMaxPods,
				PodPIDsLimit: &kubernetesKubeletPodPIDsLimit,
			},
		}
		workerKubelet = &gardenv1beta1.KubeletConfig{
			KubernetesConfig: gardenv1beta1.KubernetesConfig{
				FeatureGates: kubernetesKubeletFeatureGates,
			},
			CPUCFSQuota:      &kubernetesKubeletCPUCFSQuota,
			CPUManagerPolicy: &kubernetesKubeletCPUManagerPolicy,
			EvictionHard: &gardenv1beta1.KubeletConfigEviction{
				ImageFSAvailable:  &kubernetesKubeletEvictionHardImageFSAvailable,
				ImageFSInodesFree: &kubernetesKubeletEvictionHardImageFSInodesFree,
				MemoryAvailable:   &kubernetesKubeletEvictionHardMemoryAvailable,
				NodeFSAvailable:   &kubernetesKubeletEvictionHardNodeFSAvailable,
				NodeFSInodesFree:  &kubernetesKubeletEvictionHardNodeFSInodesFree,
			},
			EvictionMaxPodGracePeriod: &kubernetesKubeletEvictionMaxPodGracePeriod,
			EvictionMinimumReclaim: &gardenv1beta1.KubeletConfigEvictionMinimumReclaim{
				ImageFSAvailable:  &kubernetesKubeletEvictionMinimumReclaimImageFSAvailable,
				ImageFSInodesFree: &kubernetesKubeletEvictionMinimumReclaimImageFSInodesFree,
				MemoryAvailable:   &kubernetesKubeletEvictionMinimumReclaimMemoryAvailable,
				NodeFSAvailable:   &kubernetesKubeletEvictionMinimumReclaimNodeFSAvailable,
				NodeFSInodesFree:  &kubernetesKubeletEvictionMinimumReclaimNodeFSInodesFree,
			},
			EvictionPressureTransitionPeriod: &kubernetesKubeletEvictionPressureTransitionPeriod,
			EvictionSoft: &gardenv1beta1.KubeletConfigEviction{
				ImageFSAvailable:  &kubernetesKubeletEvictionSoftImageFSAvailable,
				ImageFSInodesFree: &kubernetesKubeletEvictionSoftImageFSInodesFree,
				MemoryAvailable:   &kubernetesKubeletEvictionSoftMemoryAvailable,
				NodeFSAvailable:   &kubernetesKubeletEvictionSoftNodeFSAvailable,
				NodeFSInodesFree:  &kubernetesKubeletEvictionSoftNodeFSInodesFree,
			},
			EvictionSoftGracePeriod: &gardenv1beta1.KubeletConfigEvictionSoftGracePeriod{
				ImageFSAvailable:  &kubernetesKubeletEvictionSoftGracePeriodImageFSAvailable,
				ImageFSInodesFree: &kubernetesKubeletEvictionSoftGracePeriodImageFSInodesFree,
				MemoryAvailable:   &kubernetesKubeletEvictionSoftGracePeriodMemoryAvailable,
				NodeFSAvailable:   &kubernetesKubeletEvictionSoftGracePeriodNodeFSAvailable,
				NodeFSInodesFree:  &kubernetesKubeletEvictionSoftGracePeriodNodeFSInodesFree,
			},
			MaxPods:      &kubernetesKubeletMaxPods,
			PodPIDsLimit: &kubernetesKubeletPodPIDsLimit,
		}
		worker1Labels              = map[string]string{"lab": "els"}
		worker1Name                = "worker1"
		worker2Name                = "worker2"
		worker1MachineType         = "machinetype1"
		worker1MachineImageName    = "machineimage1"
		worker1MachineImageVersion = "machineimage1version"
		worker1Maximum             = int32(5)
		worker1Minimum             = int32(3)
		worker1MaxSurge            = intstr.IntOrString{
			Type:   intstr.String,
			StrVal: "foo",
		}
		worker1MaxUnavailable = intstr.IntOrString{
			Type:   intstr.Int,
			IntVal: 4,
		}
		worker1ProviderConfig = `"foo"`
		worker1Taints         = []corev1.Taint{{Key: "taint1key"}}

		globalMachineImageJSON = "{\"Name\":\"" + worker1MachineImageName + "\",\"ProviderConfig\":null,\"Version\":\"" + worker1MachineImageVersion + "\"}"

		defaultCoreShoot = &gardencorev1alpha1.Shoot{
			Spec: gardencorev1alpha1.ShootSpec{
				Addons: &gardencorev1alpha1.Addons{
					KubernetesDashboard: &gardencorev1alpha1.KubernetesDashboard{
						Addon:              gardencorev1alpha1.Addon{Enabled: addonKubernetesDashboardEnabled},
						AuthenticationMode: &addonKubernetesDashboardAuthMode,
					},
					NginxIngress: &gardencorev1alpha1.NginxIngress{
						Addon:                    gardencorev1alpha1.Addon{Enabled: addonNginxIngressEnabled},
						LoadBalancerSourceRanges: addonNginxIngressLBSourceRanges,
					},
				},
				CloudProfileName: cloudProfileName,
				DNS: &gardencorev1alpha1.DNS{
					Domain: &dnsDomain,
					Providers: []gardencorev1alpha1.DNSProvider{
						{
							Domains: &gardencorev1alpha1.DNSIncludeExclude{
								Include: dnsProvider1DomainsInclude,
								Exclude: dnsProvider1DomainsExclude,
							},
							SecretName: &dnsProvider1SecretName,
							Type:       &dnsProvider1Type,
							Zones: &gardencorev1alpha1.DNSIncludeExclude{
								Include: dnsProvider1ZonesInclude,
								Exclude: dnsProvider1ZonesExclude,
							},
						},
						{
							Domains: &gardencorev1alpha1.DNSIncludeExclude{
								Include: dnsProvider2DomainsInclude,
								Exclude: dnsProvider2DomainsExclude,
							},
							SecretName: &dnsProvider2SecretName,
							Type:       &dnsProvider2Type,
							Zones: &gardencorev1alpha1.DNSIncludeExclude{
								Include: dnsProvider2ZonesInclude,
								Exclude: dnsProvider2ZonesExclude,
							},
						},
					},
				},
				Extensions: []gardencorev1alpha1.Extension{
					{
						Type: extension1Type,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(extension1ProviderConfig),
							},
						},
					},
				},
				Hibernation: &gardencorev1alpha1.Hibernation{
					Enabled: &hibernationEnabled,
					Schedules: []gardencorev1alpha1.HibernationSchedule{
						{
							Start:    &hibernationSchedule1Start,
							End:      &hibernationSchedule1End,
							Location: &hibernationSchedule1Location,
						},
					},
				},
				Kubernetes: gardencorev1alpha1.Kubernetes{
					AllowPrivilegedContainers: &kubernetesAllowPrivilegedContainers,
					ClusterAutoscaler: &gardencorev1alpha1.ClusterAutoscaler{
						ScaleDownDelayAfterAdd:        &kubernetesClusterAutoscalerScaleDownDelayAfterAdd,
						ScaleDownDelayAfterDelete:     &kubernetesClusterAutoscalerScaleDownDelayAfterDelete,
						ScaleDownDelayAfterFailure:    &kubernetesClusterAutoscalerScaleDownDelayAfterFailure,
						ScaleDownUnneededTime:         &kubernetesClusterAutoscalerScaleDownUnneededTime,
						ScaleDownUtilizationThreshold: &kubernetesClusterAutoscalerScaleDownUtilizationThreshold,
						ScanInterval:                  &kubernetesClusterAutoscalerScanInterval,
					},
					KubeAPIServer: &gardencorev1alpha1.KubeAPIServerConfig{
						KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
							FeatureGates: kubernetesKubeAPIServerFeatureGates,
						},
						AdmissionPlugins: []gardencorev1alpha1.AdmissionPlugin{
							{
								Name: kubernetesKubeAPIServerAdmissionPlugin1Name,
								Config: &gardencorev1alpha1.ProviderConfig{
									RawExtension: runtime.RawExtension{
										Raw: []byte(kubernetesKubeAPIServerAdmissionPlugin1ProviderConfig),
									},
								},
							},
						},
						APIAudiences: kubernetesKubeAPIServerAPIAudiences,
						AuditConfig: &gardencorev1alpha1.AuditConfig{
							AuditPolicy: &gardencorev1alpha1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: kubernetesKubeAPIServerAuditPolicyConfigMapRefName,
								},
							},
						},
						EnableBasicAuthentication: &kubernetesKubeAPIServerEnableBasicAuthentication,
						OIDCConfig: &gardencorev1alpha1.OIDCConfig{
							CABundle:       &kubernetesKubeAPIServerOIDCConfigCABundle,
							ClientID:       &kubernetesKubeAPIServerOIDCConfigClientID,
							GroupsClaim:    &kubernetesKubeAPIServerOIDCConfigGroupsClaim,
							GroupsPrefix:   &kubernetesKubeAPIServerOIDCConfigGroupsPrefix,
							IssuerURL:      &kubernetesKubeAPIServerOIDCConfigIssuerURL,
							RequiredClaims: kubernetesKubeAPIServerOIDCConfigRequiredClaims,
							SigningAlgs:    kubernetesKubeAPIServerOIDCConfigSigningAlgs,
							UsernameClaim:  &kubernetesKubeAPIServerOIDCConfigUsernameClaim,
							UsernamePrefix: &kubernetesKubeAPIServerOIDCConfigUsernamePrefix,
						},
						RuntimeConfig: kubernetesKubeAPIServerRuntimeConfig,
						ServiceAccountConfig: &gardencorev1alpha1.ServiceAccountConfig{
							Issuer: &kubernetesKubeAPIServerServiceAccountConfigIssuer,
							SigningKeySecret: &corev1.LocalObjectReference{
								Name: kubernetesKubeAPIServerServiceAccountConfigSigningKeySecretName,
							},
						},
					},
					KubeControllerManager: &gardencorev1alpha1.KubeControllerManagerConfig{
						KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
							FeatureGates: kubernetesKubeControllerManagerFeatureGates,
						},
						HorizontalPodAutoscalerConfig: &gardencorev1alpha1.HorizontalPodAutoscalerConfig{
							CPUInitializationPeriod: &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigCPUInitializationPeriod},
							DownscaleDelay:          &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigDownscaleDelay},
							DownscaleStabilization:  &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigDownscaleStabilization},
							InitialReadinessDelay:   &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigInitialReadinessDelay},
							SyncPeriod:              &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigSyncPeriod},
							Tolerance:               &kubernetesKubeControllerManagerHPAConfigTolerance,
							UpscaleDelay:            &gardencorev1alpha1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigUpscaleDelay},
						},
						NodeCIDRMaskSize: &kubernetesKubeControllerManagerNodeCIDRMaskSize,
					},
					KubeScheduler: &gardencorev1alpha1.KubeSchedulerConfig{
						KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
							FeatureGates: kubernetesKubeSchedulerFeatureGates,
						},
					},
					KubeProxy: &gardencorev1alpha1.KubeProxyConfig{
						KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
							FeatureGates: kubernetesKubeProxyFeatureGates,
						},
					},
					Kubelet: &gardencorev1alpha1.KubeletConfig{
						KubernetesConfig: gardencorev1alpha1.KubernetesConfig{
							FeatureGates: kubernetesKubeletFeatureGates,
						},
						CPUCFSQuota:      &kubernetesKubeletCPUCFSQuota,
						CPUManagerPolicy: &kubernetesKubeletCPUManagerPolicy,
						EvictionHard: &gardencorev1alpha1.KubeletConfigEviction{
							ImageFSAvailable:  &kubernetesKubeletEvictionHardImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionHardImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionHardMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionHardNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionHardNodeFSInodesFree,
						},
						EvictionMaxPodGracePeriod: &kubernetesKubeletEvictionMaxPodGracePeriod,
						EvictionMinimumReclaim: &gardencorev1alpha1.KubeletConfigEvictionMinimumReclaim{
							ImageFSAvailable:  &kubernetesKubeletEvictionMinimumReclaimImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionMinimumReclaimImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionMinimumReclaimMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionMinimumReclaimNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionMinimumReclaimNodeFSInodesFree,
						},
						EvictionPressureTransitionPeriod: &kubernetesKubeletEvictionPressureTransitionPeriod,
						EvictionSoft: &gardencorev1alpha1.KubeletConfigEviction{
							ImageFSAvailable:  &kubernetesKubeletEvictionSoftImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionSoftImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionSoftMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionSoftNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionSoftNodeFSInodesFree,
						},
						EvictionSoftGracePeriod: &gardencorev1alpha1.KubeletConfigEvictionSoftGracePeriod{
							ImageFSAvailable:  &kubernetesKubeletEvictionSoftGracePeriodImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionSoftGracePeriodImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionSoftGracePeriodMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionSoftGracePeriodNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionSoftGracePeriodNodeFSInodesFree,
						},
						MaxPods:      &kubernetesKubeletMaxPods,
						PodPIDsLimit: &kubernetesKubeletPodPIDsLimit,
					},
					Version: kubernetesVersion,
				},
				Networking: gardencorev1alpha1.Networking{
					Type: networkingType,
					ProviderConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: []byte(networkingProviderConfig),
						},
					},
					Nodes:    &networkingNodesCIDR,
					Pods:     &networkingPodsCIDR,
					Services: &networkingServicesCIDR,
				},
				Maintenance: &gardencorev1alpha1.Maintenance{
					AutoUpdate: &gardencorev1alpha1.MaintenanceAutoUpdate{
						KubernetesVersion:   maintenanceAutoUpdateKubernetesVersion,
						MachineImageVersion: maintenanceAutoUpdateMachineImageVersion,
					},
					TimeWindow: &gardencorev1alpha1.MaintenanceTimeWindow{
						Begin: maintenanceTimeWindowBegin,
						End:   maintenanceTimeWindowEnd,
					},
				},
				Region:            region,
				SecretBindingName: secretBindingName,
				SeedName:          &seedName,
			},
		}

		defaultGardenShoot = &gardenv1beta1.Shoot{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
					garden.MigrationShootWorkers:      "{\"worker1\":{\"ProviderConfig\":\"foo\",\"Volume\":null,\"Zones\":[\"zone1\",\"zone2\"]}}",
				},
			},
			Spec: gardenv1beta1.ShootSpec{
				Addons: &gardenv1beta1.Addons{
					KubernetesDashboard: &gardenv1beta1.KubernetesDashboard{
						Addon:              gardenv1beta1.Addon{Enabled: addonKubernetesDashboardEnabled},
						AuthenticationMode: &addonKubernetesDashboardAuthMode,
					},
					NginxIngress: &gardenv1beta1.NginxIngress{
						Addon:                    gardenv1beta1.Addon{Enabled: addonNginxIngressEnabled},
						LoadBalancerSourceRanges: addonNginxIngressLBSourceRanges,
					},
				},
				Cloud: gardenv1beta1.Cloud{
					Profile: cloudProfileName,
					Region:  region,
					SecretBindingRef: corev1.LocalObjectReference{
						Name: secretBindingName,
					},
					Seed: &seedName,
				},
				DNS: &gardenv1beta1.DNS{
					Domain:         &dnsDomain,
					SecretName:     &dnsProvider1SecretName,
					Provider:       &dnsProvider1Type,
					IncludeDomains: dnsProvider1DomainsInclude,
					ExcludeDomains: dnsProvider1DomainsExclude,
					IncludeZones:   dnsProvider1ZonesInclude,
					ExcludeZones:   dnsProvider1ZonesExclude,
				},
				Extensions: []gardenv1beta1.Extension{
					{
						Type: extension1Type,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(extension1ProviderConfig),
							},
						},
					},
				},
				Hibernation: &gardenv1beta1.Hibernation{
					Enabled: &hibernationEnabled,
					Schedules: []gardenv1beta1.HibernationSchedule{
						{
							Start:    &hibernationSchedule1Start,
							End:      &hibernationSchedule1End,
							Location: &hibernationSchedule1Location,
						},
					},
				},
				Kubernetes: gardenv1beta1.Kubernetes{
					AllowPrivilegedContainers: &kubernetesAllowPrivilegedContainers,
					ClusterAutoscaler: &gardenv1beta1.ClusterAutoscaler{
						ScaleDownDelayAfterAdd:        &kubernetesClusterAutoscalerScaleDownDelayAfterAdd,
						ScaleDownDelayAfterDelete:     &kubernetesClusterAutoscalerScaleDownDelayAfterDelete,
						ScaleDownDelayAfterFailure:    &kubernetesClusterAutoscalerScaleDownDelayAfterFailure,
						ScaleDownUnneededTime:         &kubernetesClusterAutoscalerScaleDownUnneededTime,
						ScaleDownUtilizationThreshold: &kubernetesClusterAutoscalerScaleDownUtilizationThreshold,
						ScanInterval:                  &kubernetesClusterAutoscalerScanInterval,
					},
					KubeAPIServer: &gardenv1beta1.KubeAPIServerConfig{
						KubernetesConfig: gardenv1beta1.KubernetesConfig{
							FeatureGates: kubernetesKubeAPIServerFeatureGates,
						},
						AdmissionPlugins: []gardenv1beta1.AdmissionPlugin{
							{
								Name: kubernetesKubeAPIServerAdmissionPlugin1Name,
								Config: &gardencorev1alpha1.ProviderConfig{
									RawExtension: runtime.RawExtension{
										Raw: []byte(kubernetesKubeAPIServerAdmissionPlugin1ProviderConfig),
									},
								},
							},
						},
						APIAudiences: kubernetesKubeAPIServerAPIAudiences,
						AuditConfig: &gardenv1beta1.AuditConfig{
							AuditPolicy: &gardenv1beta1.AuditPolicy{
								ConfigMapRef: &corev1.ObjectReference{
									Name: kubernetesKubeAPIServerAuditPolicyConfigMapRefName,
								},
							},
						},
						EnableBasicAuthentication: &kubernetesKubeAPIServerEnableBasicAuthentication,
						OIDCConfig: &gardenv1beta1.OIDCConfig{
							CABundle:       &kubernetesKubeAPIServerOIDCConfigCABundle,
							ClientID:       &kubernetesKubeAPIServerOIDCConfigClientID,
							GroupsClaim:    &kubernetesKubeAPIServerOIDCConfigGroupsClaim,
							GroupsPrefix:   &kubernetesKubeAPIServerOIDCConfigGroupsPrefix,
							IssuerURL:      &kubernetesKubeAPIServerOIDCConfigIssuerURL,
							RequiredClaims: kubernetesKubeAPIServerOIDCConfigRequiredClaims,
							SigningAlgs:    kubernetesKubeAPIServerOIDCConfigSigningAlgs,
							UsernameClaim:  &kubernetesKubeAPIServerOIDCConfigUsernameClaim,
							UsernamePrefix: &kubernetesKubeAPIServerOIDCConfigUsernamePrefix,
						},
						RuntimeConfig: kubernetesKubeAPIServerRuntimeConfig,
						ServiceAccountConfig: &gardenv1beta1.ServiceAccountConfig{
							Issuer: &kubernetesKubeAPIServerServiceAccountConfigIssuer,
							SigningKeySecret: &corev1.LocalObjectReference{
								Name: kubernetesKubeAPIServerServiceAccountConfigSigningKeySecretName,
							},
						},
					},
					KubeControllerManager: &gardenv1beta1.KubeControllerManagerConfig{
						KubernetesConfig: gardenv1beta1.KubernetesConfig{
							FeatureGates: kubernetesKubeControllerManagerFeatureGates,
						},
						HorizontalPodAutoscalerConfig: &gardenv1beta1.HorizontalPodAutoscalerConfig{
							CPUInitializationPeriod: &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigCPUInitializationPeriod},
							DownscaleDelay:          &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigDownscaleDelay},
							DownscaleStabilization:  &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigDownscaleStabilization},
							InitialReadinessDelay:   &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigInitialReadinessDelay},
							SyncPeriod:              &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigSyncPeriod},
							Tolerance:               &kubernetesKubeControllerManagerHPAConfigTolerance,
							UpscaleDelay:            &gardenv1beta1.GardenerDuration{Duration: kubernetesKubeControllerManagerHPAConfigUpscaleDelay},
						},
						NodeCIDRMaskSize: &kubernetesKubeControllerManagerNodeCIDRMaskSizeInt,
					},
					KubeScheduler: &gardenv1beta1.KubeSchedulerConfig{
						KubernetesConfig: gardenv1beta1.KubernetesConfig{
							FeatureGates: kubernetesKubeSchedulerFeatureGates,
						},
					},
					KubeProxy: &gardenv1beta1.KubeProxyConfig{
						KubernetesConfig: gardenv1beta1.KubernetesConfig{
							FeatureGates: kubernetesKubeProxyFeatureGates,
						},
					},
					Kubelet: &gardenv1beta1.KubeletConfig{
						KubernetesConfig: gardenv1beta1.KubernetesConfig{
							FeatureGates: kubernetesKubeletFeatureGates,
						},
						CPUCFSQuota:      &kubernetesKubeletCPUCFSQuota,
						CPUManagerPolicy: &kubernetesKubeletCPUManagerPolicy,
						EvictionHard: &gardenv1beta1.KubeletConfigEviction{
							ImageFSAvailable:  &kubernetesKubeletEvictionHardImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionHardImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionHardMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionHardNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionHardNodeFSInodesFree,
						},
						EvictionMaxPodGracePeriod: &kubernetesKubeletEvictionMaxPodGracePeriod,
						EvictionMinimumReclaim: &gardenv1beta1.KubeletConfigEvictionMinimumReclaim{
							ImageFSAvailable:  &kubernetesKubeletEvictionMinimumReclaimImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionMinimumReclaimImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionMinimumReclaimMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionMinimumReclaimNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionMinimumReclaimNodeFSInodesFree,
						},
						EvictionPressureTransitionPeriod: &kubernetesKubeletEvictionPressureTransitionPeriod,
						EvictionSoft: &gardenv1beta1.KubeletConfigEviction{
							ImageFSAvailable:  &kubernetesKubeletEvictionSoftImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionSoftImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionSoftMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionSoftNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionSoftNodeFSInodesFree,
						},
						EvictionSoftGracePeriod: &gardenv1beta1.KubeletConfigEvictionSoftGracePeriod{
							ImageFSAvailable:  &kubernetesKubeletEvictionSoftGracePeriodImageFSAvailable,
							ImageFSInodesFree: &kubernetesKubeletEvictionSoftGracePeriodImageFSInodesFree,
							MemoryAvailable:   &kubernetesKubeletEvictionSoftGracePeriodMemoryAvailable,
							NodeFSAvailable:   &kubernetesKubeletEvictionSoftGracePeriodNodeFSAvailable,
							NodeFSInodesFree:  &kubernetesKubeletEvictionSoftGracePeriodNodeFSInodesFree,
						},
						MaxPods:      &kubernetesKubeletMaxPods,
						PodPIDsLimit: &kubernetesKubeletPodPIDsLimit,
					},
					Version: kubernetesVersion,
				},
				Networking: &gardenv1beta1.Networking{
					Type: networkingType,
					ProviderConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: []byte(networkingProviderConfig),
						},
					},
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
				},
				Maintenance: &gardenv1beta1.Maintenance{
					AutoUpdate: &gardenv1beta1.MaintenanceAutoUpdate{
						KubernetesVersion:   maintenanceAutoUpdateKubernetesVersion,
						MachineImageVersion: &maintenanceAutoUpdateMachineImageVersion,
					},
					TimeWindow: &gardenv1beta1.MaintenanceTimeWindow{
						Begin: maintenanceTimeWindowBegin,
						End:   maintenanceTimeWindowEnd,
					},
				},
			},
			Status: gardenv1beta1.ShootStatus{
				IsHibernated: &falseVar,
			},
		}
	)

	Describe("core.gardener.cloud/v1alpha1.Shoot roundtrip", func() {
		Context("AWS provider", func() {
			var (
				providerType = "aws"

				vpcID         = "1234"
				vpcCIDR       = "10.10.10.10/10"
				zone1Name     = "zone1"
				zone1Internal = "1.2.3.4/5"
				zone1Public   = "6.7.8.9/10"
				zone1Workers  = "11.12.13.14/15"
				zone2Name     = "zone2"
				zone2Internal = "16.17.18.19/20"
				zone2Public   = "21.22.23.24/25"
				zone2Workers  = "26.27.28.29/30"

				infrastructureConfig = &awsv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: awsv1alpha1.Networks{
						VPC: awsv1alpha1.VPC{
							ID:   &vpcID,
							CIDR: &vpcCIDR,
						},
						Zones: []awsv1alpha1.Zone{
							{
								Name:     zone1Name,
								Internal: zone1Internal,
								Public:   zone1Public,
								Workers:  zone1Workers,
							},
							{
								Name:     zone2Name,
								Internal: zone2Internal,
								Public:   zone2Public,
								Workers:  zone2Workers,
							},
						},
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &awsv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &awsv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker2VolumeType   = "voltype_premium"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
					{
						Name: worker2Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker2VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Cloud.AWS = &gardenv1beta1.AWSCloud{
				MachineImage: nil,
				Networks: gardenv1beta1.AWSNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: gardenv1beta1.AWSVPC{
						ID:   &vpcID,
						CIDR: &vpcCIDR,
					},
					Internal: []string{zone1Internal, zone2Internal},
					Public:   []string{zone1Public, zone2Public},
					Workers:  []string{zone1Workers, zone2Workers},
				},
				Workers: []gardenv1beta1.AWSWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
					{
						Worker: gardenv1beta1.Worker{
							Name:        worker2Name,
							MachineType: worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker2VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			expectedOut.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Azure provider", func() {
			var (
				providerType = "azure"

				resourceGroupName = "resourcegroup1"
				vnetName          = "vnet1"
				vnetCIDR          = "10.10.10.10/10"
				workerCIDR        = "11.11.11.11/11"

				infrastructureConfig = &azurev1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					ResourceGroup: &azurev1alpha1.ResourceGroup{
						Name: resourceGroupName,
					},
					Networks: azurev1alpha1.NetworkConfig{
						VNet: azurev1alpha1.VNet{
							Name: &vnetName,
							CIDR: &vnetCIDR,
						},
						Workers: workerCIDR,
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &azurev1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &azurev1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)
				worker1VolumeSize         = "20Gi"
				worker1VolumeType         = "voltype"
				worker2VolumeType         = "voltype_premium"
				workerMigrationJSON       = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":null}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":null}}"

				in          *gardencorev1alpha1.Shoot
				expectedOut *gardenv1beta1.Shoot
			)

			BeforeEach(func() {
				in = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()

				in.Spec.Provider = gardencorev1alpha1.Provider{
					Type: providerType,
					ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: controlPlaneConfigJSON,
						},
					},
					InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: infrastructureConfigJSON,
						},
					},
					Workers: []gardencorev1alpha1.Worker{
						{
							Annotations: worker1Annotations,
							CABundle:    &worker1CABundle,
							Kubernetes:  workerKubernetes,
							Labels:      worker1Labels,
							Name:        worker1Name,
							Machine: gardencorev1alpha1.Machine{
								Type: worker1MachineType,
								Image: &gardencorev1alpha1.ShootMachineImage{
									Name:    worker1MachineImageName,
									Version: worker1MachineImageVersion,
								},
							},
							Maximum:        worker1Maximum,
							Minimum:        worker1Minimum,
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							ProviderConfig: &gardencorev1alpha1.ProviderConfig{
								RawExtension: runtime.RawExtension{
									Raw: []byte(worker1ProviderConfig),
								},
							},
							Taints: worker1Taints,
							Volume: &gardencorev1alpha1.Volume{
								Size: worker1VolumeSize,
								Type: &worker1VolumeType,
							},
						},
						{
							Name: worker2Name,
							Machine: gardencorev1alpha1.Machine{
								Type: worker1MachineType,
								Image: &gardencorev1alpha1.ShootMachineImage{
									Name:    worker1MachineImageName,
									Version: worker1MachineImageVersion,
								},
							},
							Volume: &gardencorev1alpha1.Volume{
								Size: worker1VolumeSize,
								Type: &worker2VolumeType,
							},
						},
					},
				}

				expectedOut.Annotations = map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
					garden.MigrationShootWorkers:      workerMigrationJSON,
				}

				expectedOut.Spec.Cloud.Azure = &gardenv1beta1.AzureCloud{
					MachineImage: nil,
					ResourceGroup: &gardenv1beta1.AzureResourceGroup{
						Name: resourceGroupName,
					},
					Networks: gardenv1beta1.AzureNetworks{
						K8SNetworks: gardenv1beta1.K8SNetworks{
							Nodes:    &networkingNodesCIDR,
							Pods:     &networkingPodsCIDR,
							Services: &networkingServicesCIDR,
						},
						VNet: gardenv1beta1.AzureVNet{
							Name: &vnetName,
							CIDR: &vnetCIDR,
						},
						Workers: workerCIDR,
					},
					Workers: []gardenv1beta1.AzureWorker{
						{
							Worker: gardenv1beta1.Worker{
								Annotations:   worker1Annotations,
								AutoScalerMax: int(worker1Maximum),
								AutoScalerMin: int(worker1Minimum),
								CABundle:      &worker1CABundle,
								Kubelet:       workerKubelet,
								Labels:        worker1Labels,
								Name:          worker1Name,
								MachineType:   worker1MachineType,
								MachineImage: &gardenv1beta1.ShootMachineImage{
									Name:    worker1MachineImageName,
									Version: worker1MachineImageVersion,
								},
								MaxSurge:       &worker1MaxSurge,
								MaxUnavailable: &worker1MaxUnavailable,
								Taints:         worker1Taints,
							},
							VolumeSize: worker1VolumeSize,
							VolumeType: worker1VolumeType,
						},
						{
							Worker: gardenv1beta1.Worker{
								Name:        worker2Name,
								MachineType: worker1MachineType,
								MachineImage: &gardenv1beta1.ShootMachineImage{
									Name:    worker1MachineImageName,
									Version: worker1MachineImageVersion,
								},
							},
							VolumeSize: worker1VolumeSize,
							VolumeType: worker2VolumeType,
						},
					},
				}
				expectedOut.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
					KubernetesConfig: gardenv1beta1.KubernetesConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
				}
			})

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})

			It("should correctly convert with zones and service endpoints from core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				var (
					worker1Zones     = []string{"1", "2"}
					serviceEndpoints = []string{"Microsoft.Test"}

					infrastructureConfig = &azurev1alpha1.InfrastructureConfig{
						TypeMeta: metav1.TypeMeta{
							APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
							Kind:       "InfrastructureConfig",
						},
						ResourceGroup: &azurev1alpha1.ResourceGroup{
							Name: resourceGroupName,
						},
						Networks: azurev1alpha1.NetworkConfig{
							VNet: azurev1alpha1.VNet{
								Name: &vnetName,
								CIDR: &vnetCIDR,
							},
							Workers:          workerCIDR,
							ServiceEndpoints: serviceEndpoints,
						},
						Zoned: true,
					}
					infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)
				)

				in.Spec.Provider.InfrastructureConfig.RawExtension.Raw = infrastructureConfigJSON
				in.Spec.Provider.Workers[0].Zones = worker1Zones
				in.Spec.Provider.Workers[1].Zones = worker1Zones

				expectedOut.Annotations[garden.MigrationShootWorkers] = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"
				expectedOut.Spec.Cloud.Azure.Zones = worker1Zones
				expectedOut.Spec.Cloud.Azure.Networks.ServiceEndpoints = serviceEndpoints

				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})

		})

		Context("GCP provider", func() {
			var (
				providerType = "gcp"

				vpcName      = "vpcName"
				workerCIDR   = "2.3.4.5/6"
				internalCIDR = "7.8.9.10/11"
				zone1Name    = "zone1"
				zone2Name    = "zone2"

				infrastructureConfig = &gcpv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: gcpv1alpha1.NetworkConfig{
						VPC: &gcpv1alpha1.VPC{
							Name: vpcName,
						},
						Worker:   workerCIDR,
						Internal: &internalCIDR,
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &gcpv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &gcpv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker2VolumeType   = "voltype_premium"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
					{
						Name: worker2Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker2VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{
				MachineImage: nil,
				Networks: gardenv1beta1.GCPNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: &gardenv1beta1.GCPVPC{
						Name: vpcName,
					},
					Internal: &internalCIDR,
					Workers:  []string{workerCIDR},
				},
				Workers: []gardenv1beta1.GCPWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
					{
						Worker: gardenv1beta1.Worker{
							Name:        worker2Name,
							MachineType: worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker2VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			expectedOut.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("OpenStack provider", func() {
			var (
				providerType = "openstack"

				routerID                            = "routerID"
				floatingPoolName                    = "fip1"
				loadBalancerProvider                = "lb1"
				loadBalancerClass1Name              = "lbclass1"
				loadBalancerClass1FloatingSubnetID  = "lblcassfsubnet"
				loadBalancerClass1FloatingNetworkID = "lbclassfnetwork"
				loadBalancerClass1SubnetID          = "lbclasssubnet"
				workerCIDR                          = "2.3.4.5/6"
				zone1Name                           = "zone1"
				zone2Name                           = "zone2"

				infrastructureConfig = &openstackv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					FloatingPoolName: floatingPoolName,
					Networks: openstackv1alpha1.Networks{
						Router: &openstackv1alpha1.Router{
							ID: routerID,
						},
						Worker: workerCIDR,
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &openstackv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &openstackv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					LoadBalancerProvider: loadBalancerProvider,
					LoadBalancerClasses: []openstackv1alpha1.LoadBalancerClass{
						{
							Name:              loadBalancerClass1Name,
							FloatingSubnetID:  &loadBalancerClass1FloatingSubnetID,
							FloatingNetworkID: &loadBalancerClass1FloatingNetworkID,
							SubnetID:          &loadBalancerClass1SubnetID,
						},
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Zones:  worker1Zones,
					},
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Cloud.OpenStack = &gardenv1beta1.OpenStackCloud{
				FloatingPoolName:     floatingPoolName,
				LoadBalancerProvider: loadBalancerProvider,
				LoadBalancerClasses: []gardenv1beta1.OpenStackLoadBalancerClass{
					{
						Name:              loadBalancerClass1Name,
						FloatingSubnetID:  &loadBalancerClass1FloatingSubnetID,
						FloatingNetworkID: &loadBalancerClass1FloatingNetworkID,
						SubnetID:          &loadBalancerClass1SubnetID,
					},
				},
				MachineImage: nil,
				Networks: gardenv1beta1.OpenStackNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					Router: &gardenv1beta1.OpenStackRouter{
						ID: routerID,
					},
					Workers: []string{workerCIDR},
				},
				Workers: []gardenv1beta1.OpenStackWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			expectedOut.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Alicloud provider", func() {
			var (
				providerType = "alicloud"

				vpcID        = "1234"
				vpcCIDR      = "10.10.10.10/10"
				zone1Name    = "zone1"
				zone1Workers = "11.12.13.14/15"
				zone2Name    = "zone2"
				zone2Workers = "26.27.28.29/30"

				infrastructureConfig = &alicloudv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: alicloudv1alpha1.Networks{
						VPC: alicloudv1alpha1.VPC{
							ID:   &vpcID,
							CIDR: &vpcCIDR,
						},
						Zones: []alicloudv1alpha1.Zone{
							{
								Name:   zone1Name,
								Worker: zone1Workers,
							},
							{
								Name:   zone2Name,
								Worker: zone2Workers,
							},
						},
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &alicloudv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &alicloudv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker2VolumeType   = "voltype_premium"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
					{
						Name: worker2Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker2VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Cloud.Alicloud = &gardenv1beta1.Alicloud{
				MachineImage: nil,
				Networks: gardenv1beta1.AlicloudNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: gardenv1beta1.AlicloudVPC{
						ID:   &vpcID,
						CIDR: &vpcCIDR,
					},
					Workers: []string{zone1Workers, zone2Workers},
				},
				Workers: []gardenv1beta1.AlicloudWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
					{
						Worker: gardenv1beta1.Worker{
							Name:        worker2Name,
							MachineType: worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker2VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			expectedOut.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Packet provider", func() {
			var (
				providerType = "packet"

				infrastructureConfig = &packetv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				controlPlaneConfig = &packetv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				zone1Name = "zone1"
				zone2Name = "zone2"

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker2VolumeType   = "voltype_premium"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":{\"Type\":\"" + worker1VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}" +
					",\"worker2\":{\"ProviderConfig\":null,\"Volume\":{\"Type\":\"" + worker2VolumeType + "\",\"Size\":\"" + worker1VolumeSize + "\"},\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
					{
						Name: worker2Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker2VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Cloud.Packet = &gardenv1beta1.PacketCloud{
				MachineImage: nil,
				Networks: gardenv1beta1.PacketNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
				},
				Workers: []gardenv1beta1.PacketWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
					{
						Worker: gardenv1beta1.Worker{
							Name:        worker2Name,
							MachineType: worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker2VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Unknown provider", func() {
			var (
				providerType = "unknown"

				zone1Name = "zone1"
				zone2Name = "zone2"

				infrastructureConfigJSON = `{"some":"data"}`
				controlPlaneConfigJSON   = `{"other":"data"}`

				worker1VolumeSize = "20Gi"
				worker1VolumeType = "voltype"
				worker1Zones      = []string{zone1Name, zone2Name}

				in          = defaultCoreShoot.DeepCopy()
				expectedOut = defaultGardenShoot.DeepCopy()
			)

			in.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: []byte(controlPlaneConfigJSON),
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: []byte(infrastructureConfigJSON),
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			var provider garden.Provider
			if err := gardencorev1alpha1.Convert_v1alpha1_Provider_To_garden_Provider(&in.Spec.Provider, &provider, nil); err != nil {
				panic(err)
			}
			providerJSON, _ := json.Marshal(provider)

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootProvider:     string(providerJSON),
			}

			It("should correctly convert core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})
	})

	Describe("garden.sapcloud.io/v1beta1.Shoot roundtrip", func() {
		Context("AWS provider", func() {
			var (
				providerType = "aws"

				vpcID         = "1234"
				vpcCIDR       = "10.10.10.10/10"
				zone1Name     = "zone1"
				zone1Internal = "1.2.3.4/5"
				zone1Public   = "6.7.8.9/10"
				zone1Workers  = "11.12.13.14/15"
				zone2Name     = "zone2"
				zone2Internal = "16.17.18.19/20"
				zone2Public   = "21.22.23.24/25"
				zone2Workers  = "26.27.28.29/30"

				infrastructureConfig = &awsv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: awsv1alpha1.Networks{
						VPC: awsv1alpha1.VPC{
							ID:   &vpcID,
							CIDR: &vpcCIDR,
						},
						Zones: []awsv1alpha1.Zone{
							{
								Name:     zone1Name,
								Internal: zone1Internal,
								Public:   zone1Public,
								Workers:  zone1Workers,
							},
							{
								Name:     zone2Name,
								Internal: zone2Internal,
								Public:   zone2Public,
								Workers:  zone2Workers,
							},
						},
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &awsv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &awsv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultGardenShoot.DeepCopy()
				expectedOut = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.AWS = &gardenv1beta1.AWSCloud{
				MachineImage: &gardenv1beta1.ShootMachineImage{
					Name:    worker1MachineImageName,
					Version: worker1MachineImageVersion,
				},
				Networks: gardenv1beta1.AWSNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: gardenv1beta1.AWSVPC{
						ID:   &vpcID,
						CIDR: &vpcCIDR,
					},
					Internal: []string{zone1Internal, zone2Internal},
					Public:   []string{zone1Public, zone2Public},
					Workers:  []string{zone1Workers, zone2Workers},
				},
				Workers: []gardenv1beta1.AWSWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders:       dnsProviderMigrationJSON,
				garden.MigrationShootGlobalMachineImage: globalMachineImageJSON,
				garden.MigrationShootWorkers:            workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Azure provider", func() {
			var (
				providerType = "azure"

				resourceGroupName = "resourcegroup1"
				vnetName          = "vnet1"
				vnetCIDR          = "10.10.10.10/10"
				workerCIDR        = "11.11.11.11/11"

				infrastructureConfig = &azurev1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					ResourceGroup: &azurev1alpha1.ResourceGroup{
						Name: resourceGroupName,
					},
					Networks: azurev1alpha1.NetworkConfig{
						VNet: azurev1alpha1.VNet{
							Name: &vnetName,
							CIDR: &vnetCIDR,
						},
						Workers: workerCIDR,
					},
					Zoned: true,
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &azurev1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &azurev1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker1Zones        = []string{"zone1", "zone2"}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"
				in                  = defaultGardenShoot.DeepCopy()
				expectedOut         = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.Azure = &gardenv1beta1.AzureCloud{
				MachineImage: nil,
				ResourceGroup: &gardenv1beta1.AzureResourceGroup{
					Name: resourceGroupName,
				},
				Networks: gardenv1beta1.AzureNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VNet: gardenv1beta1.AzureVNet{
						Name: &vnetName,
						CIDR: &vnetCIDR,
					},
					Workers: workerCIDR,
				},
				Workers: []gardenv1beta1.AzureWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
				},
				Zones: worker1Zones,
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				garden.MigrationShootWorkers:      workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("GCP provider", func() {
			var (
				providerType = "gcp"

				vpcName      = "vpcName"
				workerCIDR   = "2.3.4.5/6"
				internalCIDR = "7.8.9.10/11"
				zone1Name    = "zone1"
				zone2Name    = "zone2"

				infrastructureConfig = &gcpv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: gcpv1alpha1.NetworkConfig{
						VPC: &gcpv1alpha1.VPC{
							Name: vpcName,
						},
						Worker:   workerCIDR,
						Internal: &internalCIDR,
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &gcpv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &gcpv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultGardenShoot.DeepCopy()
				expectedOut = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.GCP = &gardenv1beta1.GCPCloud{
				MachineImage: &gardenv1beta1.ShootMachineImage{
					Name:    worker1MachineImageName,
					Version: worker1MachineImageVersion,
				},
				Networks: gardenv1beta1.GCPNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: &gardenv1beta1.GCPVPC{
						Name: vpcName,
					},
					Internal: &internalCIDR,
					Workers:  []string{workerCIDR},
				},
				Workers: []gardenv1beta1.GCPWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders:       dnsProviderMigrationJSON,
				garden.MigrationShootGlobalMachineImage: globalMachineImageJSON,
				garden.MigrationShootWorkers:            workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("OpenStack provider", func() {
			var (
				providerType = "openstack"

				routerID                            = "routerID"
				floatingPoolName                    = "fip1"
				loadBalancerProvider                = "lb1"
				loadBalancerClass1Name              = "lbclass1"
				loadBalancerClass1FloatingSubnetID  = "lblcassfsubnet"
				loadBalancerClass1FloatingNetworkID = "lbclassfnetwork"
				loadBalancerClass1SubnetID          = "lbclasssubnet"
				workerCIDR                          = "2.3.4.5/6"
				zone1Name                           = "zone1"
				zone2Name                           = "zone2"

				infrastructureConfig = &openstackv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					FloatingPoolName: floatingPoolName,
					Networks: openstackv1alpha1.Networks{
						Router: &openstackv1alpha1.Router{
							ID: routerID,
						},
						Worker: workerCIDR,
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &openstackv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &openstackv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					LoadBalancerProvider: loadBalancerProvider,
					LoadBalancerClasses: []openstackv1alpha1.LoadBalancerClass{
						{
							Name:              loadBalancerClass1Name,
							FloatingSubnetID:  &loadBalancerClass1FloatingSubnetID,
							FloatingNetworkID: &loadBalancerClass1FloatingNetworkID,
							SubnetID:          &loadBalancerClass1SubnetID,
						},
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultGardenShoot.DeepCopy()
				expectedOut = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.OpenStack = &gardenv1beta1.OpenStackCloud{
				FloatingPoolName:     floatingPoolName,
				LoadBalancerProvider: loadBalancerProvider,
				LoadBalancerClasses: []gardenv1beta1.OpenStackLoadBalancerClass{
					{
						Name:              loadBalancerClass1Name,
						FloatingSubnetID:  &loadBalancerClass1FloatingSubnetID,
						FloatingNetworkID: &loadBalancerClass1FloatingNetworkID,
						SubnetID:          &loadBalancerClass1SubnetID,
					},
				},
				MachineImage: &gardenv1beta1.ShootMachineImage{
					Name:    worker1MachineImageName,
					Version: worker1MachineImageVersion,
				},
				Networks: gardenv1beta1.OpenStackNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					Router: &gardenv1beta1.OpenStackRouter{
						ID: routerID,
					},
					Workers: []string{workerCIDR},
				},
				Workers: []gardenv1beta1.OpenStackWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders:       dnsProviderMigrationJSON,
				garden.MigrationShootGlobalMachineImage: globalMachineImageJSON,
				garden.MigrationShootWorkers:            workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Zones:  worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Alicloud provider", func() {
			var (
				providerType = "alicloud"

				vpcID        = "1234"
				vpcCIDR      = "10.10.10.10/10"
				zone1Name    = "zone1"
				zone1Workers = "11.12.13.14/15"
				zone2Name    = "zone2"
				zone2Workers = "26.27.28.29/30"

				infrastructureConfig = &alicloudv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
					Networks: alicloudv1alpha1.Networks{
						VPC: alicloudv1alpha1.VPC{
							ID:   &vpcID,
							CIDR: &vpcCIDR,
						},
						Zones: []alicloudv1alpha1.Zone{
							{
								Name:   zone1Name,
								Worker: zone1Workers,
							},
							{
								Name:   zone2Name,
								Worker: zone2Workers,
							},
						},
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates = map[string]bool{"ccm": true}
				controlPlaneConfig                 = &alicloudv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
					CloudControllerManager: &alicloudv1alpha1.CloudControllerManagerConfig{
						FeatureGates: cloudControllerManagerFeatureGates,
					},
					Zone: zone1Name,
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultGardenShoot.DeepCopy()
				expectedOut = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.Alicloud = &gardenv1beta1.Alicloud{
				MachineImage: &gardenv1beta1.ShootMachineImage{
					Name:    worker1MachineImageName,
					Version: worker1MachineImageVersion,
				},
				Networks: gardenv1beta1.AlicloudNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
					VPC: gardenv1beta1.AlicloudVPC{
						ID:   &vpcID,
						CIDR: &vpcCIDR,
					},
					Workers: []string{zone1Workers, zone2Workers},
				},
				Workers: []gardenv1beta1.AlicloudWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootDNSProviders:       dnsProviderMigrationJSON,
				garden.MigrationShootGlobalMachineImage: globalMachineImageJSON,
				garden.MigrationShootWorkers:            workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Packet provider", func() {
			var (
				providerType = "packet"

				zone1Name = "zone1"
				zone2Name = "zone2"

				infrastructureConfig = &packetv1alpha1.InfrastructureConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
						Kind:       "InfrastructureConfig",
					},
				}
				infrastructureConfigJSON, _ = json.Marshal(infrastructureConfig)

				cloudControllerManagerFeatureGates  = map[string]bool{"ccm": true}
				cloudControllerManagerMigrationJSON = "{\"FeatureGates\":{\"ccm\":true}}"
				controlPlaneConfig                  = &packetv1alpha1.ControlPlaneConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
						Kind:       "ControlPlaneConfig",
					},
				}
				controlPlaneConfigJSON, _ = json.Marshal(controlPlaneConfig)

				worker1VolumeSize   = "20Gi"
				worker1VolumeType   = "voltype"
				worker1Zones        = []string{zone1Name, zone2Name}
				workerMigrationJSON = "{\"worker1\":{\"ProviderConfig\":" + worker1ProviderConfig + ",\"Volume\":null,\"Zones\":[\"" + worker1Zones[0] + "\",\"" + worker1Zones[1] + "\"]}}"

				in          = defaultGardenShoot.DeepCopy()
				expectedOut = defaultCoreShoot.DeepCopy()
			)

			in.Spec.Cloud.Packet = &gardenv1beta1.PacketCloud{
				MachineImage: &gardenv1beta1.ShootMachineImage{
					Name:    worker1MachineImageName,
					Version: worker1MachineImageVersion,
				},
				Networks: gardenv1beta1.PacketNetworks{
					K8SNetworks: gardenv1beta1.K8SNetworks{
						Nodes:    &networkingNodesCIDR,
						Pods:     &networkingPodsCIDR,
						Services: &networkingServicesCIDR,
					},
				},
				Workers: []gardenv1beta1.PacketWorker{
					{
						Worker: gardenv1beta1.Worker{
							Annotations:   worker1Annotations,
							AutoScalerMax: int(worker1Maximum),
							AutoScalerMin: int(worker1Minimum),
							CABundle:      &worker1CABundle,
							Kubelet:       workerKubelet,
							Labels:        worker1Labels,
							Name:          worker1Name,
							MachineType:   worker1MachineType,
							MachineImage: &gardenv1beta1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							Taints:         worker1Taints,
						},
						VolumeSize: worker1VolumeSize,
						VolumeType: worker1VolumeType,
					},
				},
				Zones: []string{zone1Name, zone2Name},
			}
			in.Spec.Kubernetes.CloudControllerManager = &gardenv1beta1.CloudControllerManagerConfig{
				KubernetesConfig: gardenv1beta1.KubernetesConfig{
					FeatureGates: cloudControllerManagerFeatureGates,
				},
			}

			expectedOut.Annotations = map[string]string{
				garden.MigrationShootCloudControllerManager: cloudControllerManagerMigrationJSON,
				garden.MigrationShootDNSProviders:           dnsProviderMigrationJSON,
				garden.MigrationShootGlobalMachineImage:     globalMachineImageJSON,
				garden.MigrationShootWorkers:                workerMigrationJSON,
			}
			expectedOut.Spec.Provider = gardencorev1alpha1.Provider{
				Type: providerType,
				ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: controlPlaneConfigJSON,
					},
				},
				InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
					RawExtension: runtime.RawExtension{
						Raw: infrastructureConfigJSON,
					},
				},
				Workers: []gardencorev1alpha1.Worker{
					{
						Annotations: worker1Annotations,
						CABundle:    &worker1CABundle,
						Kubernetes:  workerKubernetes,
						Labels:      worker1Labels,
						Name:        worker1Name,
						Machine: gardencorev1alpha1.Machine{
							Type: worker1MachineType,
							Image: &gardencorev1alpha1.ShootMachineImage{
								Name:    worker1MachineImageName,
								Version: worker1MachineImageVersion,
							},
						},
						Maximum:        worker1Maximum,
						Minimum:        worker1Minimum,
						MaxSurge:       &worker1MaxSurge,
						MaxUnavailable: &worker1MaxUnavailable,
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{
								Raw: []byte(worker1ProviderConfig),
							},
						},
						Taints: worker1Taints,
						Volume: &gardencorev1alpha1.Volume{
							Size: worker1VolumeSize,
							Type: &worker1VolumeType,
						},
						Zones: worker1Zones,
					},
				},
			}

			It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
				out1 := &garden.Shoot{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.Shoot{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.Shoot{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.Shoot{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations = out2.Annotations
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Unknown provider", func() {
			var providerType = "unknown"

			Describe("without provider migration annotation", func() {
				var (
					in          = defaultGardenShoot.DeepCopy()
					expectedOut = defaultCoreShoot.DeepCopy()
				)

				in.Annotations = map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				}
				expectedOut.Annotations = map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
				}

				It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
					out1 := &garden.Shoot{}
					Expect(scheme.Convert(in, out1, nil)).To(BeNil())

					out2 := &gardencorev1alpha1.Shoot{}
					Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
					Expect(out2).To(Equal(expectedOut))

					out3 := &garden.Shoot{}
					Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

					out4 := &gardenv1beta1.Shoot{}
					Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
					Expect(out4).To(Equal(in))
				})
			})

			Describe("with provider migration annotation", func() {
				var (
					zone1Name = "zone1"
					zone2Name = "zone2"

					infrastructureConfigJSON = `{"some":"data"}`
					controlPlaneConfigJSON   = `{"other":"data"}`

					worker1VolumeSize = "20Gi"
					worker1VolumeType = "voltype"
					worker1Zones      = []string{zone1Name, zone2Name}

					in          = defaultGardenShoot.DeepCopy()
					expectedOut = defaultCoreShoot.DeepCopy()
				)

				provider := gardencorev1alpha1.Provider{
					Type: providerType,
					ControlPlaneConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: []byte(controlPlaneConfigJSON),
						},
					},
					InfrastructureConfig: &gardencorev1alpha1.ProviderConfig{
						RawExtension: runtime.RawExtension{
							Raw: []byte(infrastructureConfigJSON),
						},
					},
					Workers: []gardencorev1alpha1.Worker{
						{
							Annotations: worker1Annotations,
							CABundle:    &worker1CABundle,
							Kubernetes:  workerKubernetes,
							Labels:      worker1Labels,
							Name:        worker1Name,
							Machine: gardencorev1alpha1.Machine{
								Type: worker1MachineType,
								Image: &gardencorev1alpha1.ShootMachineImage{
									Name:    worker1MachineImageName,
									Version: worker1MachineImageVersion,
								},
							},
							Maximum:        worker1Maximum,
							Minimum:        worker1Minimum,
							MaxSurge:       &worker1MaxSurge,
							MaxUnavailable: &worker1MaxUnavailable,
							ProviderConfig: &gardencorev1alpha1.ProviderConfig{
								RawExtension: runtime.RawExtension{
									Raw: []byte(worker1ProviderConfig),
								},
							},
							Taints: worker1Taints,
							Volume: &gardencorev1alpha1.Volume{
								Size: worker1VolumeSize,
								Type: &worker1VolumeType,
							},
							Zones: worker1Zones,
						},
					},
				}

				var gardenProvider garden.Provider
				if err := gardencorev1alpha1.Convert_v1alpha1_Provider_To_garden_Provider(&provider, &gardenProvider, nil); err != nil {
					panic(err)
				}
				providerJSON, _ := json.Marshal(gardenProvider)

				in.Annotations = map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
					garden.MigrationShootProvider:     string(providerJSON),
				}
				expectedOut.Annotations = map[string]string{
					garden.MigrationShootDNSProviders: dnsProviderMigrationJSON,
					garden.MigrationShootProvider:     string(providerJSON),
				}
				expectedOut.Spec.Provider = provider

				It("should correctly convert garden.sapcloud.io/v1beta1.Shoot -> core.gardener.cloud/v1alpha1.Shoot -> garden.sapcloud.io/v1beta1.Shoot", func() {
					out1 := &garden.Shoot{}
					Expect(scheme.Convert(in, out1, nil)).To(BeNil())

					out2 := &gardencorev1alpha1.Shoot{}
					Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
					Expect(out2).To(Equal(expectedOut))

					out3 := &garden.Shoot{}
					Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

					out4 := &gardenv1beta1.Shoot{}
					Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

					expectedOutAfterRoundTrip := in.DeepCopy()
					expectedOutAfterRoundTrip.Annotations = out2.Annotations
					Expect(out4).To(Equal(expectedOutAfterRoundTrip))
				})
			})
		})
	})
})
