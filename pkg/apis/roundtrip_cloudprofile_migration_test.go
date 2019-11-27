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
	"strconv"
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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("roundtripper cloudprofile migration", func() {
	scheme := runtime.NewScheme()

	It("should add the conversion funcs to the scheme", func() {
		Expect(scheme.AddConversionFuncs(
			gardencorev1alpha1.Convert_v1alpha1_CloudProfile_To_garden_CloudProfile,
			gardencorev1alpha1.Convert_garden_CloudProfile_To_v1alpha1_CloudProfile,
			gardenv1beta1.Convert_v1beta1_CloudProfile_To_garden_CloudProfile,
			gardenv1beta1.Convert_garden_CloudProfile_To_v1beta1_CloudProfile,
		)).NotTo(HaveOccurred())
	})

	var (
		dnsProvider1                        = "dnsprov1"
		dnsProvider2                        = "dnsprov2"
		caBundle                            = "some-ca-bundle"
		kubernetesVersion1                  = "1.2.1"
		kubernetesVersion2                  = "1.2.0"
		kubernetesVersion2ExpirationDate    = metav1.NewTime(time.Date(2020, 2, 2, 2, 2, 2, 0, time.UTC).Local())
		machineImage1Name                   = "mach1"
		machineImage1Version1               = "24.0"
		machineImage1Version2               = "23.8"
		machineImage1Version2ExpirationDate = metav1.NewTime(time.Date(2030, 3, 3, 3, 3, 3, 0, time.UTC).Local())
		machineType1CPU                     = "200m"
		machineType1GPU                     = "2"
		machineType1Memory                  = "3Gi"
		machineType1CPUQuantity, _          = resource.ParseQuantity(machineType1CPU)
		machineType1GPUQuantity, _          = resource.ParseQuantity(machineType1GPU)
		machineType1MemoryQuantity, _       = resource.ParseQuantity(machineType1Memory)
		machineType1Name                    = "machtype1"
		machineType1Usable                  = true
		region1Name                         = "europe"
		region2Name                         = "asia"
		region1Zone1                        = "europe-first"
		seedSelector                        = metav1.LabelSelector{
			MatchLabels: map[string]string{"foo": "bar"},
		}
		seedSelectorJSON, _ = json.Marshal(seedSelector)
		volumeType1Class    = "std"
		volumeType1Name     = "voltype1"
		volumeType1Usable   = false
	)

	Describe("core.gardener.cloud/v1alpha1.CloudProfile roundtrip", func() {
		Context("AWS provider", func() {
			var (
				providerConfig = &awsv1alpha1.CloudProfileConfig{
					MachineImages: []awsv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "aws"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						AWS: &gardenv1beta1.AWSProfile{
							Constraints: gardenv1beta1.AWSConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Azure provider", func() {
			var (
				countUpdateDomainRegion = "region1"
				countUpdateDomain       = 4
				countFaultDomainRegion  = "region2"
				countFaultDomain        = 8

				providerConfig = &azurev1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "azure.provider.extensions.gardener.cloud/v1alpha1",
						Kind:       "CloudProfileConfig",
					},
					MachineImages: []azurev1alpha1.MachineImages{
						{Name: "foo"},
					},
					CountUpdateDomains: []azurev1alpha1.DomainCount{
						{Region: countUpdateDomainRegion, Count: countUpdateDomain},
					},
					CountFaultDomains: []azurev1alpha1.DomainCount{
						{Region: countFaultDomainRegion, Count: countFaultDomain},
					},
				}

				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "azure"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
							{
								Name: region2Name,
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Azure: &gardenv1beta1.AzureProfile{
							Constraints: gardenv1beta1.AzureConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
									{
										Region: region2Name,
									},
								},
							},
							CountUpdateDomains: []gardenv1beta1.AzureDomainCount{
								{Region: countUpdateDomainRegion, Count: countUpdateDomain},
							},
							CountFaultDomains: []gardenv1beta1.AzureDomainCount{
								{Region: countFaultDomainRegion, Count: countFaultDomain},
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("GCP provider", func() {
			var (
				providerConfig = &gcpv1alpha1.CloudProfileConfig{
					MachineImages: []gcpv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "gcp"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						GCP: &gardenv1beta1.GCPProfile{
							Constraints: gardenv1beta1.GCPConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("OpenStack provider", func() {
			var (
				machineType1VolumeSize, _ = resource.ParseQuantity("20Gi")
				machineType1VolumeType    = "hdd"
				machineType1StorageClass  = "premium"

				floatingPool1Name                      = "fip1"
				floatingPool1LBClass1Name              = "fip1classname"
				floatingPool1LBClass1FloatingNetworkID = "fip11234"
				floatingPool1LBClass1FloatingSubnetID  = "fip15678"
				floatingPool1LBClass1SubnetID          = "fip19101112"
				lbProvider1                            = "lbprov1"

				dnsServers     = []string{"1.2.3.4", "5.6.7.8"}
				dhcpDomain     = "dhcpdomain"
				keyStoneURL    = "url-for-keystone"
				requestTimeout = "30s"

				providerConfig = &openstackv1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "openstack.provider.extensions.gardener.cloud/v1alpha1",
						Kind:       "CloudProfileConfig",
					},
					Constraints: openstackv1alpha1.Constraints{
						FloatingPools: []openstackv1alpha1.FloatingPool{
							{
								Name: floatingPool1Name,
								LoadBalancerClasses: []openstackv1alpha1.LoadBalancerClass{
									{
										Name:              floatingPool1LBClass1Name,
										FloatingNetworkID: &floatingPool1LBClass1FloatingNetworkID,
										FloatingSubnetID:  &floatingPool1LBClass1FloatingSubnetID,
										SubnetID:          &floatingPool1LBClass1SubnetID,
									},
								},
							},
						},
						LoadBalancerProviders: []openstackv1alpha1.LoadBalancerProvider{
							{Name: lbProvider1},
						},
					},
					DNSServers:  dnsServers,
					DHCPDomain:  &dhcpDomain,
					KeyStoneURL: keyStoneURL,
					MachineImages: []openstackv1alpha1.MachineImages{
						{Name: "foo"},
					},
					RequestTimeout: &requestTimeout,
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "openstack"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
								Storage: &gardencorev1alpha1.MachineTypeStorage{
									Class: machineType1StorageClass,
									Size:  machineType1VolumeSize,
									Type:  machineType1VolumeType,
								},
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						OpenStack: &gardenv1beta1.OpenStackProfile{
							Constraints: gardenv1beta1.OpenStackConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								FloatingPools: []gardenv1beta1.OpenStackFloatingPool{
									{
										Name: floatingPool1Name,
										LoadBalancerClasses: []gardenv1beta1.OpenStackLoadBalancerClass{
											{
												Name:              floatingPool1LBClass1Name,
												FloatingNetworkID: &floatingPool1LBClass1FloatingNetworkID,
												FloatingSubnetID:  &floatingPool1LBClass1FloatingSubnetID,
												SubnetID:          &floatingPool1LBClass1SubnetID,
											},
										},
									},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								LoadBalancerProviders: []gardenv1beta1.OpenStackLoadBalancerProvider{
									{Name: lbProvider1},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.OpenStackMachineType{
									{
										MachineType: gardenv1beta1.MachineType{
											CPU:    machineType1CPUQuantity,
											GPU:    machineType1GPUQuantity,
											Memory: machineType1MemoryQuantity,
											Name:   machineType1Name,
											Usable: &machineType1Usable,
											Storage: &gardenv1beta1.MachineTypeStorage{
												Class: machineType1StorageClass,
												Size:  machineType1VolumeSize,
												Type:  machineType1VolumeType,
											},
										},
										VolumeSize: machineType1VolumeSize,
										VolumeType: machineType1VolumeType,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
							DNSServers:     dnsServers,
							DHCPDomain:     &dhcpDomain,
							KeyStoneURL:    keyStoneURL,
							RequestTimeout: &requestTimeout,
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Alicloud provider", func() {
			var (
				region1Zone2 = "zone2"

				providerConfig = &alicloudv1alpha1.CloudProfileConfig{
					MachineImages: []alicloudv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "alicloud"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{
										Name:                    region1Zone1,
										UnavailableMachineTypes: []string{machineType1Name},
									},
									{
										Name:                   region1Zone2,
										UnavailableVolumeTypes: []string{volumeType1Name},
									},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Alicloud: &gardenv1beta1.AlicloudProfile{
							Constraints: gardenv1beta1.AlicloudConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.AlicloudMachineType{
									{
										MachineType: gardenv1beta1.MachineType{
											CPU:    machineType1CPUQuantity,
											GPU:    machineType1GPUQuantity,
											Memory: machineType1MemoryQuantity,
											Name:   machineType1Name,
											Usable: &machineType1Usable,
										},
										Zones: []string{region1Zone2},
									},
								},
								VolumeTypes: []gardenv1beta1.AlicloudVolumeType{
									{
										VolumeType: gardenv1beta1.VolumeType{
											Class:  volumeType1Class,
											Name:   volumeType1Name,
											Usable: &volumeType1Usable,
										},
										Zones: []string{region1Zone1},
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1, region1Zone2},
									},
								},
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Packet provider", func() {
			var (
				providerConfig = &packetv1alpha1.CloudProfileConfig{
					MachineImages: []packetv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "packet"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Packet: &gardenv1beta1.PacketProfile{
							Constraints: gardenv1beta1.PacketConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})

		Context("Unknown provider", func() {
			var (
				providerConfigJSON = `{"apiVersion":"some-unknown.provider.extensions.gardener.cloud/v1alpha1","kind":"CloudProfileConfig"}`
				providerType       = "unknown"

				in = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders: dnsProvider1 + "," + dnsProvider2,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: []byte(providerConfigJSON)},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}

				expectedOut = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileType:           providerType,
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				kubernetesJSON, _ := json.Marshal(out1.Spec.Kubernetes)
				expectedOut.Annotations[garden.MigrationCloudProfileKubernetes] = string(kubernetesJSON)
				machineImagesJSON, _ := json.Marshal(out1.Spec.MachineImages)
				expectedOut.Annotations[garden.MigrationCloudProfileMachineImages] = string(machineImagesJSON)
				machineTypesJSON, _ := json.Marshal(out1.Spec.MachineTypes)
				expectedOut.Annotations[garden.MigrationCloudProfileMachineTypes] = string(machineTypesJSON)
				regionsJSON, _ := json.Marshal(out1.Spec.Regions)
				expectedOut.Annotations[garden.MigrationCloudProfileRegions] = string(regionsJSON)
				volumeTypesJSON, _ := json.Marshal(out1.Spec.VolumeTypes)
				expectedOut.Annotations[garden.MigrationCloudProfileVolumeTypes] = string(volumeTypesJSON)

				out2 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

				expectedOutAfterRoundTrip := in.DeepCopy()
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileType] = providerType
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileProviderConfig] = string(providerConfigJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileSeedSelector] = string(seedSelectorJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileKubernetes] = string(kubernetesJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileMachineImages] = string(machineImagesJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileMachineTypes] = string(machineTypesJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileRegions] = string(regionsJSON)
				expectedOutAfterRoundTrip.Annotations[garden.MigrationCloudProfileVolumeTypes] = string(volumeTypesJSON)
				Expect(out4).To(Equal(expectedOutAfterRoundTrip))
			})
		})
	})

	Describe("garden.sapcloud.io/v1beta1.CloudProfile roundtrip", func() {
		Context("AWS provider", func() {
			var (
				providerConfig = &awsv1alpha1.CloudProfileConfig{
					MachineImages: []awsv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "aws"

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						AWS: &gardenv1beta1.AWSProfile{
							Constraints: gardenv1beta1.AWSConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("Azure provider", func() {
			var (
				countUpdateDomainRegion = "region1"
				countUpdateDomain       = 4
				countFaultDomainRegion  = "region2"
				countFaultDomain        = 8

				providerConfig = &azurev1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "azure.provider.extensions.gardener.cloud/v1alpha1",
						Kind:       "CloudProfileConfig",
					},
					MachineImages: []azurev1alpha1.MachineImages{
						{Name: "foo"},
					},
					CountUpdateDomains: []azurev1alpha1.DomainCount{
						{Region: countUpdateDomainRegion, Count: countUpdateDomain},
					},
					CountFaultDomains: []azurev1alpha1.DomainCount{
						{Region: countFaultDomainRegion, Count: countFaultDomain},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "azure"

				regionsAnnotation = `[{"Name":"` + region1Name + `","Zones":[{"Name":"` + region1Zone1 + `"}]}]`

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileRegions:        regionsAnnotation,
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Azure: &gardenv1beta1.AzureProfile{
							Constraints: gardenv1beta1.AzureConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
							CountUpdateDomains: []gardenv1beta1.AzureDomainCount{
								{Region: countUpdateDomainRegion, Count: countUpdateDomain},
							},
							CountFaultDomains: []gardenv1beta1.AzureDomainCount{
								{Region: countFaultDomainRegion, Count: countFaultDomain},
							},
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileRegions:        regionsAnnotation,
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("GCP provider", func() {
			var (
				providerConfig = &gcpv1alpha1.CloudProfileConfig{
					MachineImages: []gcpv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "gcp"

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						GCP: &gardenv1beta1.GCPProfile{
							Constraints: gardenv1beta1.GCPConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("OpenStack provider", func() {
			var (
				machineType1VolumeSize, _ = resource.ParseQuantity("20Gi")
				machineType1VolumeType    = "hdd"

				floatingPool1Name                      = "fip1"
				floatingPool1LBClass1Name              = "fip1classname"
				floatingPool1LBClass1FloatingNetworkID = "fip11234"
				floatingPool1LBClass1FloatingSubnetID  = "fip15678"
				floatingPool1LBClass1SubnetID          = "fip19101112"
				lbProvider1                            = "lbprov1"

				dnsServers     = []string{"1.2.3.4", "5.6.7.8"}
				dhcpDomain     = "dhcpdomain"
				keyStoneURL    = "url-for-keystone"
				requestTimeout = "30s"

				providerConfig = &openstackv1alpha1.CloudProfileConfig{
					TypeMeta: metav1.TypeMeta{
						APIVersion: "openstack.provider.extensions.gardener.cloud/v1alpha1",
						Kind:       "CloudProfileConfig",
					},
					Constraints: openstackv1alpha1.Constraints{
						FloatingPools: []openstackv1alpha1.FloatingPool{
							{
								Name: floatingPool1Name,
								LoadBalancerClasses: []openstackv1alpha1.LoadBalancerClass{
									{
										Name:              floatingPool1LBClass1Name,
										FloatingNetworkID: &floatingPool1LBClass1FloatingNetworkID,
										FloatingSubnetID:  &floatingPool1LBClass1FloatingSubnetID,
										SubnetID:          &floatingPool1LBClass1SubnetID,
									},
								},
							},
						},
						LoadBalancerProviders: []openstackv1alpha1.LoadBalancerProvider{
							{Name: lbProvider1},
						},
					},
					DNSServers:  dnsServers,
					DHCPDomain:  &dhcpDomain,
					KeyStoneURL: keyStoneURL,
					MachineImages: []openstackv1alpha1.MachineImages{
						{Name: "foo"},
					},
					RequestTimeout: &requestTimeout,
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "openstack"

				volumeTypesAnnotation = `[{"Class":"` + volumeType1Class + `","Name":"` + volumeType1Name + `","Usable":` + strconv.FormatBool(volumeType1Usable) + `}]`

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileVolumeTypes:    string(volumeTypesAnnotation),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						OpenStack: &gardenv1beta1.OpenStackProfile{
							Constraints: gardenv1beta1.OpenStackConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								FloatingPools: []gardenv1beta1.OpenStackFloatingPool{
									{
										Name: floatingPool1Name,
										LoadBalancerClasses: []gardenv1beta1.OpenStackLoadBalancerClass{
											{
												Name:              floatingPool1LBClass1Name,
												FloatingNetworkID: &floatingPool1LBClass1FloatingNetworkID,
												FloatingSubnetID:  &floatingPool1LBClass1FloatingSubnetID,
												SubnetID:          &floatingPool1LBClass1SubnetID,
											},
										},
									},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								LoadBalancerProviders: []gardenv1beta1.OpenStackLoadBalancerProvider{
									{Name: lbProvider1},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.OpenStackMachineType{
									{
										MachineType: gardenv1beta1.MachineType{
											CPU:    machineType1CPUQuantity,
											GPU:    machineType1GPUQuantity,
											Memory: machineType1MemoryQuantity,
											Name:   machineType1Name,
											Usable: &machineType1Usable,
											Storage: &gardenv1beta1.MachineTypeStorage{
												Size: machineType1VolumeSize,
												Type: machineType1VolumeType,
											},
										},
										VolumeSize: machineType1VolumeSize,
										VolumeType: machineType1VolumeType,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
							DNSServers:     dnsServers,
							DHCPDomain:     &dhcpDomain,
							KeyStoneURL:    keyStoneURL,
							RequestTimeout: &requestTimeout,
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileVolumeTypes:    string(volumeTypesAnnotation),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
								Storage: &gardencorev1alpha1.MachineTypeStorage{
									Size: machineType1VolumeSize,
									Type: machineType1VolumeType,
								},
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("Alicloud provider", func() {
			var (
				region1Zone2 = "zone2"

				providerConfig = &alicloudv1alpha1.CloudProfileConfig{
					MachineImages: []alicloudv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "alicloud"

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Alicloud: &gardenv1beta1.AlicloudProfile{
							Constraints: gardenv1beta1.AlicloudConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.AlicloudMachineType{
									{
										MachineType: gardenv1beta1.MachineType{
											CPU:    machineType1CPUQuantity,
											GPU:    machineType1GPUQuantity,
											Memory: machineType1MemoryQuantity,
											Name:   machineType1Name,
											Usable: &machineType1Usable,
										},
										Zones: []string{region1Zone2},
									},
								},
								VolumeTypes: []gardenv1beta1.AlicloudVolumeType{
									{
										VolumeType: gardenv1beta1.VolumeType{
											Class:  volumeType1Class,
											Name:   volumeType1Name,
											Usable: &volumeType1Usable,
										},
										Zones: []string{region1Zone1},
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1, region1Zone2},
									},
								},
							},
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{
										Name:                    region1Zone1,
										UnavailableMachineTypes: []string{machineType1Name},
									},
									{
										Name:                   region1Zone2,
										UnavailableVolumeTypes: []string{volumeType1Name},
									},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("Packet provider", func() {
			var (
				providerConfig = &packetv1alpha1.CloudProfileConfig{
					MachineImages: []packetv1alpha1.MachineImages{
						{Name: "foo"},
					},
				}
				providerConfigJSON, _ = json.Marshal(providerConfig)
				providerType          = "packet"

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
						Packet: &gardenv1beta1.PacketProfile{
							Constraints: gardenv1beta1.PacketConstraints{
								DNSProviders: []gardenv1beta1.DNSProviderConstraint{
									{Name: dnsProvider1},
									{Name: dnsProvider2},
								},
								Kubernetes: gardenv1beta1.KubernetesConstraints{
									Versions: []string{kubernetesVersion1, kubernetesVersion2},
									OfferedVersions: []gardenv1beta1.KubernetesVersion{
										{Version: kubernetesVersion1},
										{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
									},
								},
								MachineImages: []gardenv1beta1.MachineImage{
									{
										Name: machineImage1Name,
										Versions: []gardenv1beta1.MachineImageVersion{
											{Version: machineImage1Version1},
											{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
										},
									},
								},
								MachineTypes: []gardenv1beta1.MachineType{
									{
										CPU:    machineType1CPUQuantity,
										GPU:    machineType1GPUQuantity,
										Memory: machineType1MemoryQuantity,
										Name:   machineType1Name,
										Usable: &machineType1Usable,
									},
								},
								VolumeTypes: []gardenv1beta1.VolumeType{
									{
										Class:  volumeType1Class,
										Name:   volumeType1Name,
										Usable: &volumeType1Usable,
									},
								},
								Zones: []gardenv1beta1.Zone{
									{
										Region: region1Name,
										Names:  []string{region1Zone1},
									},
								},
							},
						},
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: providerConfigJSON},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})

		Context("Unknown provider", func() {
			var (
				providerConfigJSON = `{"apiVersion":"some-unknown.provider.extensions.gardener.cloud/v1alpha1","kind":"CloudProfileConfig"}`
				providerType       = "unknown"

				kubernetesVersion2ExpirationDateJSON, _    = json.Marshal(kubernetesVersion2ExpirationDate)
				kubernetesAnnotation                       = `{"Versions":[{"Version":"` + kubernetesVersion1 + `"},{"Version":"` + kubernetesVersion2 + `","ExpirationDate":` + string(kubernetesVersion2ExpirationDateJSON) + `}]}`
				machineImage1Version2ExpirationDateJSON, _ = json.Marshal(machineImage1Version2ExpirationDate)
				machineImagesAnnotation                    = `[{"Name":"` + machineImage1Name + `","Versions":[{"Version":"` + machineImage1Version1 + `"},{"Version":"` + machineImage1Version2 + `","ExpirationDate":` + string(machineImage1Version2ExpirationDateJSON) + `}]}]`
				machineTypesAnnotation                     = `[{"CPU":"` + machineType1CPU + `","GPU":"` + machineType1GPU + `","Memory":"` + machineType1Memory + `","Name":"` + machineType1Name + `","Usable":` + strconv.FormatBool(machineType1Usable) + `}]`
				regionsAnnotation                          = `[{"Name":"` + region1Name + `","Zones":[{"Name":"` + region1Zone1 + `"}]}]`
				volumeTypesAnnotation                      = `[{"Class":"` + volumeType1Class + `","Name":"` + volumeType1Name + `","Usable":` + strconv.FormatBool(volumeType1Usable) + `}]`

				in = &gardenv1beta1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileType:           providerType,
							garden.MigrationCloudProfileKubernetes:     string(kubernetesAnnotation),
							garden.MigrationCloudProfileMachineImages:  string(machineImagesAnnotation),
							garden.MigrationCloudProfileMachineTypes:   string(machineTypesAnnotation),
							garden.MigrationCloudProfileRegions:        regionsAnnotation,
							garden.MigrationCloudProfileVolumeTypes:    string(volumeTypesAnnotation),
						},
					},
					Spec: gardenv1beta1.CloudProfileSpec{
						CABundle: &caBundle,
					},
				}

				expectedOut = &gardencorev1alpha1.CloudProfile{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationCloudProfileDNSProviders:   dnsProvider1 + "," + dnsProvider2,
							garden.MigrationCloudProfileProviderConfig: string(providerConfigJSON),
							garden.MigrationCloudProfileSeedSelector:   string(seedSelectorJSON),
							garden.MigrationCloudProfileType:           providerType,
							garden.MigrationCloudProfileKubernetes:     string(kubernetesAnnotation),
							garden.MigrationCloudProfileMachineImages:  string(machineImagesAnnotation),
							garden.MigrationCloudProfileMachineTypes:   string(machineTypesAnnotation),
							garden.MigrationCloudProfileRegions:        regionsAnnotation,
							garden.MigrationCloudProfileVolumeTypes:    string(volumeTypesAnnotation),
						},
					},
					Spec: gardencorev1alpha1.CloudProfileSpec{
						CABundle: &caBundle,
						Kubernetes: gardencorev1alpha1.KubernetesSettings{
							Versions: []gardencorev1alpha1.ExpirableVersion{
								{Version: kubernetesVersion1},
								{Version: kubernetesVersion2, ExpirationDate: &kubernetesVersion2ExpirationDate},
							},
						},
						MachineImages: []gardencorev1alpha1.MachineImage{
							{
								Name: machineImage1Name,
								Versions: []gardencorev1alpha1.ExpirableVersion{
									{Version: machineImage1Version1},
									{Version: machineImage1Version2, ExpirationDate: &machineImage1Version2ExpirationDate},
								},
							},
						},
						MachineTypes: []gardencorev1alpha1.MachineType{
							{
								CPU:    machineType1CPUQuantity,
								GPU:    machineType1GPUQuantity,
								Memory: machineType1MemoryQuantity,
								Name:   machineType1Name,
								Usable: &machineType1Usable,
							},
						},
						ProviderConfig: &gardencorev1alpha1.ProviderConfig{
							RawExtension: runtime.RawExtension{Raw: []byte(providerConfigJSON)},
						},
						Regions: []gardencorev1alpha1.Region{
							{
								Name: region1Name,
								Zones: []gardencorev1alpha1.AvailabilityZone{
									{Name: region1Zone1},
								},
							},
						},
						SeedSelector: &seedSelector,
						Type:         providerType,
						VolumeTypes: []gardencorev1alpha1.VolumeType{
							{
								Class:  volumeType1Class,
								Name:   volumeType1Name,
								Usable: &volumeType1Usable,
							},
						},
					},
				}
			)

			It("should correctly convert core.gardener.cloud/v1alpha1.CloudProfile -> garden.sapcloud.io/v1beta1.CloudProfile -> core.gardener.cloud/v1alpha1.CloudProfile", func() {
				out1 := &garden.CloudProfile{}
				Expect(scheme.Convert(in, out1, nil)).To(BeNil())

				out2 := &gardencorev1alpha1.CloudProfile{}
				Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
				Expect(out2).To(Equal(expectedOut))

				out3 := &garden.CloudProfile{}
				Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

				out4 := &gardenv1beta1.CloudProfile{}
				Expect(scheme.Convert(out3, out4, nil)).To(BeNil())
				Expect(out4).To(Equal(in))
			})
		})
	})
})
