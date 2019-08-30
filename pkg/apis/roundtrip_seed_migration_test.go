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
	"fmt"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"
	gardenv1beta1 "github.com/gardener/gardener/pkg/apis/garden/v1beta1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("roundtripper seed migration", func() {
	var scheme *runtime.Scheme

	BeforeSuite(func() {
		scheme = runtime.NewScheme()
		Expect(scheme.AddConversionFuncs(
			gardencorev1alpha1.Convert_v1alpha1_Seed_To_garden_Seed,
			gardencorev1alpha1.Convert_garden_Seed_To_v1alpha1_Seed,
			gardenv1beta1.Convert_v1beta1_Seed_To_garden_Seed,
			gardenv1beta1.Convert_garden_Seed_To_v1beta1_Seed,
		)).NotTo(HaveOccurred())
	})

	var (
		cloudProfileName                   = "cloudprofile1"
		providerName                       = "provider1"
		regionName                         = "region1"
		ingressDomain                      = "foo.example.com"
		secretRefName                      = "seed-secret"
		secretRefNamespace                 = "garden"
		nodesCIDR                          = gardencorev1alpha1.CIDR("1.2.3.4/5")
		podsCIDR                           = gardencorev1alpha1.CIDR("6.7.8.9/10")
		servicesCIDR                       = gardencorev1alpha1.CIDR("11.12.13.14/15")
		blockCIDR                          = gardencorev1alpha1.CIDR("16.17.18.19/20")
		shootDefaultPodCIDR                = gardencorev1alpha1.CIDR("100.96.0.0/11")
		shootDefaultServiceCIDR            = gardencorev1alpha1.CIDR("100.64.0.0/13")
		minimumVolumeSize                  = "20Gi"
		minimumVolumeSizeQuantity, _       = resource.ParseQuantity(minimumVolumeSize)
		volumeProviderPurpose1             = "etcd-main"
		volumeProviderName1                = "flexvolume"
		volumeProviderPurpose2             = "foo"
		volumeProviderName2                = "bar"
		migrationVolumeProvidersAnnotation = `[{"Purpose":"` + volumeProviderPurpose2 + `","Name":"` + volumeProviderName2 + `"}]`

		trueVar  = true
		falseVar = false
	)

	Describe("core/v1alpha1.Seed roundtrip", func() {
		var (
			in = &gardencorev1alpha1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						garden.MigrationSeedCloudProfile: cloudProfileName,
						garden.MigrationSeedCloudRegion:  regionName,
					},
				},
				Spec: gardencorev1alpha1.SeedSpec{
					Provider: gardencorev1alpha1.SeedProvider{
						Type:   providerName,
						Region: regionName,
					},
					DNS: gardencorev1alpha1.SeedDNS{
						IngressDomain: ingressDomain,
					},
					SecretRef: corev1.SecretReference{
						Name:      secretRefName,
						Namespace: secretRefNamespace,
					},
					Networks: gardencorev1alpha1.SeedNetworks{
						Nodes:    nodesCIDR,
						Pods:     podsCIDR,
						Services: servicesCIDR,
						ShootDefaults: &gardencorev1alpha1.ShootNetworks{
							Pods:     &shootDefaultPodCIDR,
							Services: &shootDefaultServiceCIDR,
						},
					},
					BlockCIDRs: []gardencorev1alpha1.CIDR{blockCIDR},
					Taints: []gardencorev1alpha1.SeedTaint{
						{
							Key: gardencorev1alpha1.SeedTaintProtected,
						},
						{
							Key: gardencorev1alpha1.SeedTaintInvisible,
						},
					},
					Volume: &gardencorev1alpha1.SeedVolume{
						MinimumSize: &minimumVolumeSizeQuantity,
						Providers: []gardencorev1alpha1.SeedVolumeProvider{
							{
								Purpose: volumeProviderPurpose1,
								Name:    volumeProviderName1,
							},
							{
								Purpose: volumeProviderPurpose2,
								Name:    volumeProviderName2,
							},
						},
					},
				},
			}
			expectedOut = &gardenv1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
						"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
						garden.MigrationSeedVolumeProviders:               migrationVolumeProvidersAnnotation,
						garden.MigrationSeedVolumeMinimumSize:             minimumVolumeSize,
						garden.MigrationSeedCloudProfile:                  cloudProfileName,
						garden.MigrationSeedCloudRegion:                   regionName,
						garden.MigrationSeedProviderType:                  providerName,
						garden.MigrationSeedProviderRegion:                regionName,
						garden.MigrationSeedTaints:                        fmt.Sprintf("%s,%s", garden.SeedTaintProtected, garden.SeedTaintInvisible),
					},
				},
				Spec: gardenv1beta1.SeedSpec{
					Cloud: gardenv1beta1.SeedCloud{
						Profile: cloudProfileName,
						Region:  regionName,
					},
					IngressDomain: ingressDomain,
					SecretRef: corev1.SecretReference{
						Name:      secretRefName,
						Namespace: secretRefNamespace,
					},
					Networks: gardenv1beta1.SeedNetworks{
						Nodes:    nodesCIDR,
						Pods:     podsCIDR,
						Services: servicesCIDR,
						ShootDefaults: &gardenv1beta1.ShootNetworks{
							Pods:     &shootDefaultPodCIDR,
							Services: &shootDefaultServiceCIDR,
						},
					},
					BlockCIDRs: []gardencorev1alpha1.CIDR{blockCIDR},
					Protected:  &trueVar,
					Visible:    &falseVar,
				},
			}
		)

		It("should correctly convert core/v1alpha1.Seed -> garden/v1beta1.Seed -> core/v1alpha1.Seed", func() {
			out1 := &garden.Seed{}
			Expect(scheme.Convert(in, out1, nil)).To(BeNil())

			out2 := &gardenv1beta1.Seed{}
			Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
			Expect(out2).To(Equal(expectedOut))

			out3 := &garden.Seed{}
			Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

			out4 := &gardencorev1alpha1.Seed{}
			Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

			expectedOutAfterRoundTrip := in.DeepCopy()
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedVolumeProviders] = migrationVolumeProvidersAnnotation
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedVolumeMinimumSize] = minimumVolumeSize
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedProviderType] = providerName
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedProviderRegion] = regionName
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedTaints] = fmt.Sprintf("%s,%s", garden.SeedTaintProtected, garden.SeedTaintInvisible)
			Expect(out4).To(Equal(expectedOutAfterRoundTrip))
		})
	})

	Describe("garden/v1beta1.Seed roundtrip", func() {
		var (
			in = &gardenv1beta1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
						"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
						garden.MigrationSeedProviderType:                  providerName,
						garden.MigrationSeedProviderRegion:                regionName,
					},
				},
				Spec: gardenv1beta1.SeedSpec{
					Cloud: gardenv1beta1.SeedCloud{
						Profile: cloudProfileName,
						Region:  regionName,
					},
					IngressDomain: ingressDomain,
					SecretRef: corev1.SecretReference{
						Name:      secretRefName,
						Namespace: secretRefNamespace,
					},
					Networks: gardenv1beta1.SeedNetworks{
						Nodes:    nodesCIDR,
						Pods:     podsCIDR,
						Services: servicesCIDR,
						ShootDefaults: &gardenv1beta1.ShootNetworks{
							Pods:     &shootDefaultPodCIDR,
							Services: &shootDefaultServiceCIDR,
						},
					},
					BlockCIDRs: []gardencorev1alpha1.CIDR{blockCIDR},
					Protected:  &trueVar,
					Visible:    &falseVar,
				},
			}
			expectedOut = &gardencorev1alpha1.Seed{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						garden.MigrationSeedCloudProfile:   cloudProfileName,
						garden.MigrationSeedCloudRegion:    regionName,
						garden.MigrationSeedProviderType:   providerName,
						garden.MigrationSeedProviderRegion: regionName,
					},
				},
				Spec: gardencorev1alpha1.SeedSpec{
					Provider: gardencorev1alpha1.SeedProvider{
						Type:   providerName,
						Region: regionName,
					},
					DNS: gardencorev1alpha1.SeedDNS{
						IngressDomain: ingressDomain,
					},
					SecretRef: corev1.SecretReference{
						Name:      secretRefName,
						Namespace: secretRefNamespace,
					},
					Networks: gardencorev1alpha1.SeedNetworks{
						Nodes:    nodesCIDR,
						Pods:     podsCIDR,
						Services: servicesCIDR,
						ShootDefaults: &gardencorev1alpha1.ShootNetworks{
							Pods:     &shootDefaultPodCIDR,
							Services: &shootDefaultServiceCIDR,
						},
					},
					BlockCIDRs: []gardencorev1alpha1.CIDR{blockCIDR},
					Taints: []gardencorev1alpha1.SeedTaint{
						{
							Key: gardencorev1alpha1.SeedTaintProtected,
						},
						{
							Key: gardencorev1alpha1.SeedTaintInvisible,
						},
					},
					Volume: &gardencorev1alpha1.SeedVolume{
						MinimumSize: &minimumVolumeSizeQuantity,
						Providers: []gardencorev1alpha1.SeedVolumeProvider{
							{
								Purpose: volumeProviderPurpose1,
								Name:    volumeProviderName1,
							},
						},
					},
				},
			}
		)

		It("should correctly convert garden/v1beta1.Seed -> core/v1alpha1.Seed -> core/v1alpha1.Seed", func() {
			out1 := &garden.Seed{}
			Expect(scheme.Convert(in, out1, nil)).To(BeNil())

			out2 := &gardencorev1alpha1.Seed{}
			Expect(scheme.Convert(out1, out2, nil)).To(BeNil())
			Expect(out2).To(Equal(expectedOut))

			out3 := &garden.Seed{}
			Expect(scheme.Convert(out2, out3, nil)).To(BeNil())

			out4 := &gardenv1beta1.Seed{}
			Expect(scheme.Convert(out3, out4, nil)).To(BeNil())

			expectedOutAfterRoundTrip := in.DeepCopy()
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedVolumeMinimumSize] = minimumVolumeSize
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedCloudProfile] = cloudProfileName
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedCloudRegion] = regionName
			expectedOutAfterRoundTrip.Annotations[garden.MigrationSeedTaints] = fmt.Sprintf("%s,%s", garden.SeedTaintProtected, garden.SeedTaintInvisible)
			Expect(out4).To(Equal(expectedOutAfterRoundTrip))
		})
	})
})
