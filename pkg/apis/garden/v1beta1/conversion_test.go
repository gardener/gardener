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

package v1beta1

import (
	"fmt"
	"time"

	gardencorev1alpha1 "github.com/gardener/gardener/pkg/apis/core/v1alpha1"
	"github.com/gardener/gardener/pkg/apis/garden"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

var _ = Describe("Machine Image Conversion", func() {
	var (
		expirationDate     = &metav1.Time{Time: time.Now().Add(time.Second * 20)}
		v1betaMachineImage *MachineImage
		gardenMachineImage *garden.MachineImage

		scheme *runtime.Scheme
	)

	BeforeSuite(func() {
		scheme = runtime.NewScheme()
		Expect(scheme.AddConversionFuncs(
			Convert_v1beta1_Seed_To_garden_Seed,
			Convert_garden_Seed_To_v1beta1_Seed,
			Convert_v1beta1_Quota_To_garden_Quota,
			Convert_garden_Quota_To_v1beta1_Quota,
		)).NotTo(HaveOccurred())
	})

	BeforeEach(func() {
		v1betaMachineImage = &MachineImage{
			Name: "coreos",
			Versions: []MachineImageVersion{
				{
					Version:        "0.0.9",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.7",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.8",
					ExpirationDate: expirationDate,
				},
			},
		}

		gardenMachineImage = &garden.MachineImage{
			Name: "coreos",
			Versions: []garden.MachineImageVersion{
				{
					Version: "1.0.0",
				},
				{
					Version:        "0.0.9",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.7",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.8",
					ExpirationDate: expirationDate,
				},
			},
		}
	})

	Describe("#V1Beta1MachineImageToGardenMachineImage", func() {
		It("external machine image should be properly converted to internal machine image", func() {
			v1betaMachineImage.Version = "1.0.0"
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			Expect(internalMachineImage.Name).To(Equal("coreos"))
			Expect(internalMachineImage.Versions).To(HaveLen(4))
			Expect(internalMachineImage.Versions[0].Version).To(Equal("1.0.0"))
			Expect(internalMachineImage.Versions[0].ExpirationDate).To(BeNil())
		})

		It("external machine image (no version set) should be properly converted to internal machine image", func() {
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			Expect(internalMachineImage.Name).To(Equal("coreos"))
			Expect(internalMachineImage.Versions).To(HaveLen(3))
			Expect(internalMachineImage.Versions[0].Version).To(Equal("0.0.9"))
		})
	})

	Describe("#GardenMachineImageToV1Beta1MachineImage", func() {
		It("internal machine image should be properly converted to external machine image", func() {
			v1betaMachineImage := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(gardenMachineImage, v1betaMachineImage, nil)

			Expect(v1betaMachineImage.Name).To(Equal("coreos"))
			Expect(v1betaMachineImage.Version).To(HaveLen(0))
			Expect(v1betaMachineImage.Versions).To(HaveLen(4))
			Expect(v1betaMachineImage.Versions[0].Version).To(Equal("1.0.0"))
			Expect(v1betaMachineImage.Versions[0].ExpirationDate).To(BeNil())
		})
	})

	Describe("#GardenMachineImageBackAndForth", func() {
		It("assure no structural change in resulting external version after back and forth conversion", func() {
			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			v1betaMachineImageResult := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(internalMachineImage, v1betaMachineImageResult, nil)

			Expect(v1betaMachineImageResult).To(Equal(v1betaMachineImage))
		})

		It("assure expected structural change (when image.Version is set in v1beta1) in resulting external version after back and forth conversion", func() {
			v1betaMachineImage.Version = "1.0.0"

			internalMachineImage := &garden.MachineImage{}

			Convert_v1beta1_MachineImage_To_garden_MachineImage(v1betaMachineImage, internalMachineImage, nil)

			v1betaMachineImageResult := &MachineImage{}

			Convert_garden_MachineImage_To_v1beta1_MachineImage(internalMachineImage, v1betaMachineImageResult, nil)

			Expect(v1betaMachineImageResult).ToNot(Equal(v1betaMachineImage))
			Expect(v1betaMachineImageResult.Version).To(HaveLen(0))
			Expect(v1betaMachineImageResult.Versions).To(HaveLen(4))
			Expect(v1betaMachineImageResult.Versions[0].Version).To(Equal("1.0.0"))
			Expect(v1betaMachineImageResult.Versions[0].ExpirationDate).To(BeNil())
		})
	})

	Context("seed conversions", func() {
		var (
			cloudProfileName             = "cloudprofile1"
			providerName                 = "provider1"
			regionName                   = "region1"
			ingressDomain                = "foo.example.com"
			secretRefName                = "seed-secret"
			secretRefNamespace           = "garden"
			nodesCIDR                    = gardencorev1alpha1.CIDR("1.2.3.4/5")
			podsCIDR                     = gardencorev1alpha1.CIDR("6.7.8.9/10")
			servicesCIDR                 = gardencorev1alpha1.CIDR("11.12.13.14/15")
			defaultPodCIDR               = DefaultPodNetworkCIDR
			defaultServiceCIDR           = DefaultServiceNetworkCIDR
			blockCIDR                    = "16.17.18.19/20"
			taintKeyOtherOne             = "some-other-taint-key"
			taintKeyOtherTwo             = "yet-some-other-taint-key"
			minimumVolumeSize            = "20Gi"
			minimumVolumeSizeQuantity, _ = resource.ParseQuantity(minimumVolumeSize)
			volumeProviderPurpose1       = "etcd-main"
			volumeProviderName1          = "flexvolume"
			volumeProviderPurpose2       = "foo"
			volumeProviderName2          = "bar"

			trueVar  = true
			falseVar = false
		)

		Describe("#Convert_v1beta1_Seed_To_garden_Seed", func() {
			var (
				annotations = map[string]string{
					garden.MigrationSeedProviderType:                  providerName,
					garden.MigrationSeedProviderRegion:                regionName,
					garden.MigrationSeedVolumeMinimumSize:             minimumVolumeSize,
					garden.MigrationSeedVolumeProviders:               `[{"Purpose":"` + volumeProviderPurpose2 + `","Name":"` + volumeProviderName2 + `"}]`,
					"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
					"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
					garden.MigrationSeedTaints:                        fmt.Sprintf("%s,%s,%s,%s", garden.SeedTaintProtected, garden.SeedTaintInvisible, taintKeyOtherOne, taintKeyOtherTwo),
				}

				out = &garden.Seed{}
				in  = &Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: annotations,
					},
					Spec: SeedSpec{
						Cloud: SeedCloud{
							Profile: cloudProfileName,
							Region:  regionName,
						},
						IngressDomain: ingressDomain,
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: SeedNetworks{
							Nodes:    nodesCIDR,
							Pods:     podsCIDR,
							Services: servicesCIDR,
						},
						BlockCIDRs: []gardencorev1alpha1.CIDR{gardencorev1alpha1.CIDR(blockCIDR)},
						Protected:  &trueVar,
						Visible:    &falseVar,
					},
				}
			)

			It("should correctly convert", func() {
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(BeEquivalentTo(&garden.Seed{
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
							{Key: garden.SeedTaintProtected},
							{Key: garden.SeedTaintInvisible},
							{Key: taintKeyOtherOne},
							{Key: taintKeyOtherTwo},
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

		Describe("#Convert_garden_Seed_To_v1beta1_Seed", func() {
			var (
				out = &Seed{}
				in  = &garden.Seed{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{
							garden.MigrationSeedProviderType:                  providerName,
							garden.MigrationSeedProviderRegion:                regionName,
							garden.MigrationSeedVolumeMinimumSize:             minimumVolumeSize,
							garden.MigrationSeedVolumeProviders:               `[{"Purpose":"` + volumeProviderPurpose2 + `","Name":"` + volumeProviderName2 + `"}]`,
							"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
							"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
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
						BlockCIDRs: []garden.CIDR{garden.CIDR(blockCIDR)},
						Taints: []garden.SeedTaint{
							{Key: garden.SeedTaintProtected},
							{Key: taintKeyOtherOne},
							{Key: taintKeyOtherTwo},
						},
						Volume: &garden.SeedVolume{
							MinimumSize: &minimumVolumeSizeQuantity,
							Providers: []garden.SeedVolumeProvider{
								{
									Purpose: volumeProviderPurpose2,
									Name:    volumeProviderName2,
								},
								{
									Purpose: volumeProviderPurpose1,
									Name:    volumeProviderName1,
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
						Annotations: map[string]string{
							garden.MigrationSeedProviderType:                  providerName,
							garden.MigrationSeedProviderRegion:                regionName,
							garden.MigrationSeedVolumeMinimumSize:             minimumVolumeSize,
							garden.MigrationSeedVolumeProviders:               `[{"Purpose":"` + volumeProviderPurpose2 + `","Name":"` + volumeProviderName2 + `"}]`,
							"persistentvolume.garden.sapcloud.io/minimumSize": minimumVolumeSize,
							"persistentvolume.garden.sapcloud.io/provider":    volumeProviderName1,
							garden.MigrationSeedTaints:                        fmt.Sprintf("%s,%s,%s", garden.SeedTaintProtected, taintKeyOtherOne, taintKeyOtherTwo),
						},
					},
					Spec: SeedSpec{
						Cloud: SeedCloud{
							Profile: cloudProfileName,
							Region:  regionName,
						},
						IngressDomain: ingressDomain,
						SecretRef: corev1.SecretReference{
							Name:      secretRefName,
							Namespace: secretRefNamespace,
						},
						Networks: SeedNetworks{
							Nodes:    nodesCIDR,
							Pods:     podsCIDR,
							Services: servicesCIDR,
							ShootDefaults: &ShootNetworks{
								Pods:     &defaultPodCIDR,
								Services: &defaultServiceCIDR,
							},
						},
						BlockCIDRs: []gardencorev1alpha1.CIDR{gardencorev1alpha1.CIDR(blockCIDR)},
						Protected:  &trueVar,
						Visible:    &trueVar,
					},
				}))
			})
		})
	})

	Context("quota conversions", func() {
		var (
			metrics = corev1.ResourceList{
				"foo": resource.Quantity{Format: "bar"},
			}
			clusterLifetimeDays = 12
		)

		Describe("#Convert_v1beta1_Quota_To_garden_Quota", func() {
			var (
				out = &garden.Quota{}
				in  *Quota
			)

			BeforeEach(func() {
				in = &Quota{
					Spec: QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
					},
				}
			})

			It("should correctly convert for scope=project", func() {
				in.Spec.Scope = QuotaScopeProject
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(BeEquivalentTo(&garden.Quota{
					Spec: garden.QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
						Scope: corev1.ObjectReference{
							APIVersion: "core.gardener.cloud/v1alpha1",
							Kind:       "Project",
						},
					},
				}))
			})

			It("should correctly convert for scope=secret", func() {
				in.Spec.Scope = QuotaScopeSecret
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(BeEquivalentTo(&garden.Quota{
					Spec: garden.QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
						Scope: corev1.ObjectReference{
							APIVersion: "v1",
							Kind:       "Secret",
						},
					},
				}))
			})
		})

		Describe("#Convert_garden_Quota_To_v1beta1_Quota", func() {
			var (
				out = &Quota{}
				in  *garden.Quota
			)

			BeforeEach(func() {
				in = &garden.Quota{
					Spec: garden.QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
					},
				}
			})

			It("should correctly convert for scopeRef=core.gardener.cloud/v1alpha1.Project", func() {
				in.Spec.Scope = corev1.ObjectReference{
					APIVersion: "core.gardener.cloud/v1alpha1",
					Kind:       "Project",
				}
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Quota{
					Spec: QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
						Scope:               QuotaScopeProject,
					},
				}))
			})

			It("should correctly convert for scopeRef=v1.Secret", func() {
				in.Spec.Scope = corev1.ObjectReference{
					APIVersion: "v1",
					Kind:       "Secret",
				}
				Expect(scheme.Convert(in, out, nil)).To(BeNil())
				Expect(out).To(Equal(&Quota{
					Spec: QuotaSpec{
						ClusterLifetimeDays: &clusterLifetimeDays,
						Metrics:             metrics,
						Scope:               QuotaScopeSecret,
					},
				}))
			})
		})
	})
})

var _ = Describe("Kubernetes Constraint Conversion", func() {
	var (
		expirationDate             = &metav1.Time{Time: time.Now().Add(time.Second * 20)}
		v1betaKubernetesConstraint *KubernetesConstraints
	)

	BeforeEach(func() {
		v1betaKubernetesConstraint = &KubernetesConstraints{
			OfferedVersions: []KubernetesVersion{
				{
					Version:        "0.0.9",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.7",
					ExpirationDate: expirationDate,
				},
				{
					Version:        "0.0.8",
					ExpirationDate: expirationDate,
				},
				{
					Version: "0.0.6",
				},
			},
		}
	})

	Describe("#V1Beta1KubernetesConstraintToGardenKubernetesConstraint", func() {
		It("external kubernetes constraints should be properly converted to internal kubernetes constraints", func() {
			v1betaKubernetesConstraint.Versions = []string{"0.0.1", "0.0.2", "0.0.8", "0.0.6"}
			internal := &garden.KubernetesConstraints{}

			Convert_v1beta1_KubernetesConstraints_To_garden_KubernetesConstraints(v1betaKubernetesConstraint, internal, nil)

			// versions "0.0.1", "0.0.2" are distinct in the versions, "0.0.8", "0.0.6" exist also in the offered versions
			Expect(internal.OfferedVersions).To(HaveLen(len(v1betaKubernetesConstraint.OfferedVersions) + len(v1betaKubernetesConstraint.Versions) - 2))
			Expect(internal.OfferedVersions[0].Version).To(Equal("0.0.1"))
			Expect(internal.OfferedVersions[0].ExpirationDate).To(BeNil())
			// 0.0.8 should have expiration date - exists both in the versions and the offered versions
			Expect(internal.OfferedVersions[2].ExpirationDate).ToNot(BeNil())
			// 0.0.6 should have no expiration date - exists both in the versions and the offered versions
			Expect(internal.OfferedVersions[3].ExpirationDate).To(BeNil())
		})
		It("external kubernetes constraints (no version set) should be properly converted to internal kubernetes constraints", func() {
			internal := &garden.KubernetesConstraints{}

			Convert_v1beta1_KubernetesConstraints_To_garden_KubernetesConstraints(v1betaKubernetesConstraint, internal, nil)

			Expect(internal.OfferedVersions).To(HaveLen(len(v1betaKubernetesConstraint.OfferedVersions)))
			Expect(internal.OfferedVersions[0].Version).To(Equal("0.0.9"))
		})
	})

	Describe("#GardenKubernetesConstraintBackAndForth", func() {
		It("assure expected structural change (when constraints.OfferedVersions is set in v1beta1) in resulting external version after back and forth conversion", func() {
			v1betaKubernetesConstraint.Versions = []string{"0.0.1", "0.0.2"}

			internal := &garden.KubernetesConstraints{}
			Convert_v1beta1_KubernetesConstraints_To_garden_KubernetesConstraints(v1betaKubernetesConstraint, internal, nil)

			v1betaKubernetesConstraintResult := &KubernetesConstraints{}

			Convert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(internal, v1betaKubernetesConstraintResult, nil)

			Expect(v1betaKubernetesConstraintResult).ToNot(Equal(internal))
			Expect(v1betaKubernetesConstraintResult.OfferedVersions).To(HaveLen(len(v1betaKubernetesConstraint.Versions) + len(v1betaKubernetesConstraint.OfferedVersions)))
			Expect(v1betaKubernetesConstraintResult.Versions).To(HaveLen(len(v1betaKubernetesConstraintResult.OfferedVersions)))
			Expect(v1betaKubernetesConstraintResult.OfferedVersions[0].Version).To(Equal("0.0.1"))
			Expect(v1betaKubernetesConstraintResult.Versions[0]).To(Equal("0.0.1"))
			Expect(v1betaKubernetesConstraintResult.OfferedVersions[0].ExpirationDate).To(BeNil())
		})
	})
})
