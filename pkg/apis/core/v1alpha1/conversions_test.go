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

package v1alpha1_test

import (
	. "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Conversion", func() {
	var scheme *runtime.Scheme

	BeforeSuite(func() {
		scheme = runtime.NewScheme()
		Expect(scheme.AddConversionFuncs(
			Convert_v1alpha1_Seed_To_garden_Seed,
			Convert_garden_Seed_To_v1alpha1_Seed,
			Convert_v1alpha1_Quota_To_garden_Quota,
			Convert_garden_Quota_To_v1alpha1_Quota,
		)).NotTo(HaveOccurred())
	})

	Context("seed conversions", func() {
		var (
			cloudProfileName = "cloudprofile1"
			providerName     = "provider1"
			regionName       = "region1"
			annotations      = map[string]string{
				garden.MigrationSeedCloudProfile: cloudProfileName,
				garden.MigrationSeedCloudRegion:  regionName,
			}
			ingressDomain                = "foo.example.com"
			secretRefName                = "seed-secret"
			secretRefNamespace           = "garden"
			nodesCIDR                    = CIDR("1.2.3.4/5")
			podsCIDR                     = CIDR("6.7.8.9/10")
			servicesCIDR                 = CIDR("11.12.13.14/15")
			blockCIDR                    = "16.17.18.19/20"
			minimumVolumeSize            = "20Gi"
			minimumVolumeSizeQuantity, _ = resource.ParseQuantity(minimumVolumeSize)
			volumeProviderPurpose1       = "etcd-main"
			volumeProviderName1          = "flexvolume"
			volumeProviderPurpose2       = "foo"
			volumeProviderName2          = "bar"
		)

		Describe("#Convert_v1alpha1_Seed_To_garden_Seed", func() {
			var (
				out = &garden.Seed{}
				in  = &Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: SeedSpec{
						Provider: SeedProvider{
							Type:   providerName,
							Region: regionName,
						},
						DNS: SeedDNS{
							IngressDomain: ingressDomain,
						},
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: SeedNetworks{
							Nodes:    nodesCIDR,
							Pods:     podsCIDR,
							Services: servicesCIDR,
						},
						BlockCIDRs: []CIDR{CIDR(blockCIDR)},
						Taints: []SeedTaint{
							{
								Key: SeedTaintProtected,
							},
							{
								Key: SeedTaintInvisible,
							},
						},
						Volume: &SeedVolume{
							MinimumSize: &minimumVolumeSizeQuantity,
							Providers: []SeedVolumeProvider{
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
			)

			It("should correctly convert", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&garden.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: garden.SeedSpec{
						Cloud: garden.SeedCloud{
							Profile: cloudProfileName,
							Region:  regionName,
						},
						Provider: garden.SeedProvider{
							Type:   providerName,
							Region: regionName,
						},
						IngressDomain: ingressDomain,
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: garden.SeedNetworks{
							Nodes:    garden.CIDR(nodesCIDR),
							Pods:     garden.CIDR(podsCIDR),
							Services: garden.CIDR(servicesCIDR),
						},
						BlockCIDRs: []garden.CIDR{garden.CIDR(blockCIDR)},
						Taints: []garden.SeedTaint{
							{
								Key: SeedTaintProtected,
							},
							{
								Key: SeedTaintInvisible,
							},
						},
						Volume: &garden.SeedVolume{
							MinimumSize: &minimumVolumeSizeQuantity,
							Providers: []garden.SeedVolumeProvider{
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
				}))
			})
		})

		Describe("#Convert_garden_Seed_To_v1alpha1_Seed", func() {
			var (
				out = &Seed{}
				in  = &garden.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
							"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
						},
					},
					Spec: garden.SeedSpec{
						Cloud: garden.SeedCloud{
							Profile: cloudProfileName,
							Region:  regionName,
						},
						Provider: garden.SeedProvider{
							Type:   providerName,
							Region: regionName,
						},
						IngressDomain: ingressDomain,
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: garden.SeedNetworks{
							Nodes:    garden.CIDR(nodesCIDR),
							Pods:     garden.CIDR(podsCIDR),
							Services: garden.CIDR(servicesCIDR),
						},
						Taints: []garden.SeedTaint{
							{
								Key: SeedTaintProtected,
							},
							{
								Key: SeedTaintInvisible,
							},
						},
						BlockCIDRs: []garden.CIDR{garden.CIDR(blockCIDR)},
						Volume: &garden.SeedVolume{
							MinimumSize: &minimumVolumeSizeQuantity,
							Providers: []garden.SeedVolumeProvider{
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
			)

			It("should correctly convert", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: SeedSpec{
						Provider: SeedProvider{
							Type:   providerName,
							Region: regionName,
						},
						DNS: SeedDNS{
							IngressDomain: ingressDomain,
						},
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: SeedNetworks{
							Nodes:    nodesCIDR,
							Pods:     podsCIDR,
							Services: servicesCIDR,
						},
						BlockCIDRs: []CIDR{CIDR(blockCIDR)},
						Taints: []SeedTaint{
							{
								Key: SeedTaintProtected,
							},
							{
								Key: SeedTaintInvisible,
							},
						},
						Volume: &SeedVolume{
							MinimumSize: &minimumVolumeSizeQuantity,
							Providers: []SeedVolumeProvider{
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
				}))
			})
		})
	})
})
