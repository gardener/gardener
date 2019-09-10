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

package v1alpha1

import (
	"encoding/json"
	"strings"

	"github.com/gardener/gardener/pkg/apis/garden"

	azureinstall "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/v1alpha1"
	openstackinstall "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
)

func addConversionFuncs(scheme *runtime.Scheme) error {
	// Add non-generated conversion functions
	return scheme.AddConversionFuncs(
		Convert_v1alpha1_Seed_To_garden_Seed,
		Convert_garden_Seed_To_v1alpha1_Seed,
		Convert_v1alpha1_CloudProfile_To_garden_CloudProfile,
		Convert_garden_CloudProfile_To_v1alpha1_CloudProfile,
	)
}

func Convert_v1alpha1_Seed_To_garden_Seed(in *Seed, out *garden.Seed, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_Seed_To_garden_Seed(in, out, s); err != nil {
		return err
	}

	if a := in.Annotations; a != nil {
		if v, ok := a[garden.MigrationSeedCloudProfile]; ok {
			out.Spec.Cloud.Profile = v
		}

		if v, ok := a[garden.MigrationSeedCloudRegion]; ok {
			out.Spec.Cloud.Region = v
		}
	}

	out.Spec.IngressDomain = in.Spec.DNS.IngressDomain
	out.Spec.Cloud.Region = in.Spec.Provider.Region

	return nil
}

func Convert_garden_Seed_To_v1alpha1_Seed(in *garden.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_garden_Seed_To_v1alpha1_Seed(in, out, s); err != nil {
		return err
	}

	if len(in.Spec.Cloud.Profile) > 0 || len(in.Spec.Cloud.Region) > 0 || in.Spec.Volume != nil {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old)+2)
		for k, v := range old {
			if k != "persistentvolume.garden.sapcloud.io/provider" && k != "persistentvolume.garden.sapcloud.io/minimumSize" {
				out.Annotations[k] = v
			}
		}
	}

	if len(in.Spec.Cloud.Profile) > 0 {
		out.Annotations[garden.MigrationSeedCloudProfile] = in.Spec.Cloud.Profile
	}

	if len(in.Spec.Cloud.Region) > 0 {
		out.Annotations[garden.MigrationSeedCloudRegion] = in.Spec.Cloud.Region
	}

	if len(in.Spec.IngressDomain) > 0 {
		out.Spec.DNS = SeedDNS{
			IngressDomain: in.Spec.IngressDomain,
		}
	}

	return nil
}

func Convert_garden_SeedSpec_To_v1alpha1_SeedSpec(in *garden.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_garden_SeedSpec_To_v1alpha1_SeedSpec(in, out, s)
}

func Convert_v1alpha1_SeedSpec_To_garden_SeedSpec(in *SeedSpec, out *garden.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_SeedSpec_To_garden_SeedSpec(in, out, s)
}

func Convert_v1alpha1_ProjectSpec_To_garden_ProjectSpec(in *ProjectSpec, out *garden.ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_ProjectSpec_To_garden_ProjectSpec(in, out, s); err != nil {
		return err
	}

	for _, member := range in.Members {
		out.ProjectMembers = append(out.ProjectMembers, garden.ProjectMember{
			Subject: member.Subject,
			Role:    member.Role,
		})
	}

	return nil
}

func Convert_garden_ProjectSpec_To_v1alpha1_ProjectSpec(in *garden.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_garden_ProjectSpec_To_v1alpha1_ProjectSpec(in, out, s); err != nil {
		return err
	}

	for _, member := range in.ProjectMembers {
		out.Members = append(out.Members, ProjectMember{
			Subject: member.Subject,
			Role:    member.Role,
		})
	}

	return nil
}

func Convert_v1alpha1_CloudProfile_To_garden_CloudProfile(in *CloudProfile, out *garden.CloudProfile, s conversion.Scope) error {
	if err := autoConvert_v1alpha1_CloudProfile_To_garden_CloudProfile(in, out, s); err != nil {
		return err
	}

	if out.Annotations == nil {
		out.Annotations = make(map[string]string)
	}

	switch in.Spec.Type {
	case "aws":
		if out.Spec.AWS == nil {
			out.Spec.AWS = &garden.AWSProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.AWS.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.AWS.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.AWS.Constraints.Kubernetes.OfferedVersions = append(out.Spec.AWS.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.AWS.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.AWS.Constraints.MachineImages = append(out.Spec.AWS.Constraints.MachineImages, m)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !machineTypesHaveName(out.Spec.AWS.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := autoConvert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.AWS.Constraints.MachineTypes = append(out.Spec.AWS.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.AWS.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1alpha1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				out.Spec.AWS.Constraints.VolumeTypes = append(out.Spec.AWS.Constraints.VolumeTypes, o)
			}
		}

		for _, region := range in.Spec.Regions {
			if !zonesHaveName(out.Spec.AWS.Constraints.Zones, region.Name) {
				z := garden.Zone{Region: region.Name}
				for _, zones := range region.Zones {
					z.Names = append(z.Names, zones.Name)
				}
				out.Spec.AWS.Constraints.Zones = append(out.Spec.AWS.Constraints.Zones, z)
			}
		}

	case "azure":
		if out.Spec.Azure == nil {
			out.Spec.Azure = &garden.AzureProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.Azure.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.Azure.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.Azure.Constraints.Kubernetes.OfferedVersions = append(out.Spec.Azure.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.Azure.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.Azure.Constraints.MachineImages = append(out.Spec.Azure.Constraints.MachineImages, m)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !machineTypesHaveName(out.Spec.Azure.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := autoConvert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.Azure.Constraints.MachineTypes = append(out.Spec.Azure.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.Azure.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1alpha1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				out.Spec.Azure.Constraints.VolumeTypes = append(out.Spec.Azure.Constraints.VolumeTypes, o)
			}
		}

		cloudProfileConfig := &azurev1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		if in.Spec.ProviderConfig != nil {
			extensionsScheme := runtime.NewScheme()
			if err := azureinstall.AddToScheme(extensionsScheme); err != nil {
				return err
			}
			decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()
			if _, _, err := decoder.Decode(in.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
				// If an error occurs then the provider config information contains invalid syntax, and in this
				// case we don't want to fail here. We rather don't try to migrate.
				klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
			}
		}
		for _, c := range cloudProfileConfig.CountFaultDomains {
			if !domainCountsHaveRegion(out.Spec.Azure.CountFaultDomains, c.Region) {
				out.Spec.Azure.CountFaultDomains = append(out.Spec.Azure.CountFaultDomains, garden.AzureDomainCount{
					Region: c.Region,
					Count:  c.Count,
				})
			}
		}
		for _, c := range cloudProfileConfig.CountUpdateDomains {
			if !domainCountsHaveRegion(out.Spec.Azure.CountUpdateDomains, c.Region) {
				out.Spec.Azure.CountUpdateDomains = append(out.Spec.Azure.CountUpdateDomains, garden.AzureDomainCount{
					Region: c.Region,
					Count:  c.Count,
				})
			}
		}

	case "gcp":
		if out.Spec.GCP == nil {
			out.Spec.GCP = &garden.GCPProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.GCP.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.GCP.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.GCP.Constraints.Kubernetes.OfferedVersions = append(out.Spec.GCP.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.GCP.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.GCP.Constraints.MachineImages = append(out.Spec.GCP.Constraints.MachineImages, m)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !machineTypesHaveName(out.Spec.GCP.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := autoConvert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.GCP.Constraints.MachineTypes = append(out.Spec.GCP.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.GCP.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1alpha1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				out.Spec.GCP.Constraints.VolumeTypes = append(out.Spec.GCP.Constraints.VolumeTypes, o)
			}
		}

		for _, region := range in.Spec.Regions {
			if !zonesHaveName(out.Spec.GCP.Constraints.Zones, region.Name) {
				z := garden.Zone{Region: region.Name}
				for _, zones := range region.Zones {
					z.Names = append(z.Names, zones.Name)
				}
				out.Spec.GCP.Constraints.Zones = append(out.Spec.GCP.Constraints.Zones, z)
			}
		}

	case "openstack":
		if out.Spec.OpenStack == nil {
			out.Spec.OpenStack = &garden.OpenStackProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.OpenStack.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions = append(out.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.OpenStack.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.OpenStack.Constraints.MachineImages = append(out.Spec.OpenStack.Constraints.MachineImages, m)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !openStackMachineTypesHaveName(out.Spec.OpenStack.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := Convert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				t := garden.OpenStackMachineType{MachineType: o}
				if machineType.Storage != nil {
					t.Storage = &garden.MachineTypeStorage{
						Size: machineType.Storage.Size,
						Type: machineType.Storage.Type,
					}
					t.VolumeSize = machineType.Storage.Size
					t.VolumeType = machineType.Storage.Type
				}
				out.Spec.OpenStack.Constraints.MachineTypes = append(out.Spec.OpenStack.Constraints.MachineTypes, t)
			}
		}

		for _, region := range in.Spec.Regions {
			if !zonesHaveName(out.Spec.OpenStack.Constraints.Zones, region.Name) {
				z := garden.Zone{Region: region.Name}
				for _, zones := range region.Zones {
					z.Names = append(z.Names, zones.Name)
				}
				out.Spec.OpenStack.Constraints.Zones = append(out.Spec.OpenStack.Constraints.Zones, z)
			}
		}

		cloudProfileConfig := &openstackv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		if in.Spec.ProviderConfig != nil {
			extensionsScheme := runtime.NewScheme()
			if err := openstackinstall.AddToScheme(extensionsScheme); err != nil {
				return err
			}
			decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()
			if _, _, err := decoder.Decode(in.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
				// If an error occurs then the provider config information contains invalid syntax, and in this
				// case we don't want to fail here. We rather don't try to migrate.
				klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
			}
		}
		for _, p := range cloudProfileConfig.Constraints.LoadBalancerProviders {
			if !loadBalancerProvidersHaveName(out.Spec.OpenStack.Constraints.LoadBalancerProviders, p.Name) {
				out.Spec.OpenStack.Constraints.LoadBalancerProviders = append(out.Spec.OpenStack.Constraints.LoadBalancerProviders, garden.OpenStackLoadBalancerProvider{
					Name: p.Name,
				})
			}
		}
		for _, p := range cloudProfileConfig.Constraints.FloatingPools {
			if !floatingPoolsHavePool(out.Spec.OpenStack.Constraints.FloatingPools, p.Name) {
				f := garden.OpenStackFloatingPool{
					Name: p.Name,
				}
				for _, c := range p.LoadBalancerClasses {
					f.LoadBalancerClasses = append(f.LoadBalancerClasses, garden.OpenStackLoadBalancerClass{
						Name:              c.Name,
						FloatingSubnetID:  c.FloatingSubnetID,
						FloatingNetworkID: c.FloatingNetworkID,
						SubnetID:          c.SubnetID,
					})
				}
				out.Spec.OpenStack.Constraints.FloatingPools = append(out.Spec.OpenStack.Constraints.FloatingPools, f)
			}
		}
		out.Spec.OpenStack.DNSServers = cloudProfileConfig.DNSServers
		out.Spec.OpenStack.DHCPDomain = cloudProfileConfig.DHCPDomain
		out.Spec.OpenStack.KeyStoneURL = cloudProfileConfig.KeyStoneURL
		out.Spec.OpenStack.RequestTimeout = cloudProfileConfig.RequestTimeout

	case "alicloud":
		if out.Spec.Alicloud == nil {
			out.Spec.Alicloud = &garden.AlicloudProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.Alicloud.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions = append(out.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.Alicloud.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.Alicloud.Constraints.MachineImages = append(out.Spec.Alicloud.Constraints.MachineImages, m)
			}
		}

		allZones := sets.NewString()
		unavailableMachineTypesPerZone := map[string][]string{}
		unavailableVolumeTypesPerZone := map[string][]string{}
		for _, region := range in.Spec.Regions {
			z := garden.Zone{Region: region.Name}
			for _, zones := range region.Zones {
				z.Names = append(z.Names, zones.Name)
				allZones.Insert(zones.Name)
				for _, t := range zones.UnavailableMachineTypes {
					unavailableMachineTypesPerZone[zones.Name] = append(unavailableMachineTypesPerZone[zones.Name], t)
				}
				for _, t := range zones.UnavailableVolumeTypes {
					unavailableVolumeTypesPerZone[zones.Name] = append(unavailableVolumeTypesPerZone[zones.Name], t)
				}
			}
			if !zonesHaveName(out.Spec.Alicloud.Constraints.Zones, region.Name) {
				out.Spec.Alicloud.Constraints.Zones = append(out.Spec.Alicloud.Constraints.Zones, z)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !alicloudMachineTypesHaveName(out.Spec.Alicloud.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := Convert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				var zones []string
				for _, zone := range allZones.List() {
					if !zoneHaveAlicloudType(unavailableMachineTypesPerZone, zone, machineType.Name) {
						zones = append(zones, zone)
					}
				}
				out.Spec.Alicloud.Constraints.MachineTypes = append(out.Spec.Alicloud.Constraints.MachineTypes, garden.AlicloudMachineType{
					MachineType: o,
					Zones:       zones,
				})
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !alicloudVolumeTypesHaveName(out.Spec.Alicloud.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := Convert_v1alpha1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				var zones []string
				for _, zone := range allZones.List() {
					if !zoneHaveAlicloudType(unavailableVolumeTypesPerZone, zone, volumeType.Name) {
						zones = append(zones, zone)
					}
				}
				out.Spec.Alicloud.Constraints.VolumeTypes = append(out.Spec.Alicloud.Constraints.VolumeTypes, garden.AlicloudVolumeType{
					VolumeType: o,
					Zones:      zones,
				})
			}
		}

	case "packet":
		if out.Spec.Packet == nil {
			out.Spec.Packet = &garden.PacketProfile{}
		}

		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			out.Spec.Packet.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

		for _, version := range in.Spec.Kubernetes.Versions {
			if !offeredVersionsHaveVersion(out.Spec.Packet.Constraints.Kubernetes.OfferedVersions, version.Version) {
				out.Spec.Packet.Constraints.Kubernetes.OfferedVersions = append(out.Spec.Packet.Constraints.Kubernetes.OfferedVersions, garden.KubernetesVersion{
					Version:        version.Version,
					ExpirationDate: version.ExpirationDate,
				})
			}
		}

		for _, machineImage := range in.Spec.MachineImages {
			if !machineImagesHaveImage(out.Spec.Packet.Constraints.MachineImages, machineImage.Name) {
				m := garden.MachineImage{Name: machineImage.Name}
				for _, version := range machineImage.Versions {
					m.Versions = append(m.Versions, garden.MachineImageVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
				out.Spec.Packet.Constraints.MachineImages = append(out.Spec.Packet.Constraints.MachineImages, m)
			}
		}

		for _, machineType := range in.Spec.MachineTypes {
			if !machineTypesHaveName(out.Spec.Packet.Constraints.MachineTypes, machineType.Name) {
				var o garden.MachineType
				if err := autoConvert_v1alpha1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.Packet.Constraints.MachineTypes = append(out.Spec.Packet.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.Packet.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1alpha1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				out.Spec.Packet.Constraints.VolumeTypes = append(out.Spec.Packet.Constraints.VolumeTypes, o)
			}
		}

		for _, region := range in.Spec.Regions {
			if !zonesHaveName(out.Spec.Packet.Constraints.Zones, region.Name) {
				z := garden.Zone{Region: region.Name}
				for _, zones := range region.Zones {
					z.Names = append(z.Names, zones.Name)
				}
				out.Spec.Packet.Constraints.Zones = append(out.Spec.Packet.Constraints.Zones, z)
			}
		}

	default:
		out.Annotations[garden.MigrationCloudProfileType] = in.Spec.Type
	}

	if in.Spec.ProviderConfig != nil {
		data, err := json.Marshal(in.Spec.ProviderConfig)
		if err != nil {
			return err
		}
		out.Annotations[garden.MigrationCloudProfileProviderConfig] = string(data)
	} else {
		delete(out.Annotations, garden.MigrationCloudProfileProviderConfig)
	}

	if in.Spec.SeedSelector != nil {
		data, err := json.Marshal(in.Spec.SeedSelector)
		if err != nil {
			return err
		}
		out.Annotations[garden.MigrationCloudProfileSeedSelector] = string(data)
	} else {
		delete(out.Annotations, garden.MigrationCloudProfileSeedSelector)
	}

	return nil
}

func Convert_garden_CloudProfile_To_v1alpha1_CloudProfile(in *garden.CloudProfile, out *CloudProfile, s conversion.Scope) error {
	if err := autoConvert_garden_CloudProfile_To_v1alpha1_CloudProfile(in, out, s); err != nil {
		return err
	}

	switch {
	case in.Spec.AWS != nil:
		out.Spec.Type = "aws"

		if len(in.Spec.AWS.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.AWS.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}

	case in.Spec.Azure != nil:
		out.Spec.Type = "azure"

		if len(in.Spec.Azure.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.Azure.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}

	case in.Spec.GCP != nil:
		out.Spec.Type = "gcp"

		if len(in.Spec.GCP.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.GCP.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}

	case in.Spec.OpenStack != nil:
		out.Spec.Type = "openstack"

		if len(in.Spec.OpenStack.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.OpenStack.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}

	case in.Spec.Alicloud != nil:
		out.Spec.Type = "alicloud"

		if len(in.Spec.Alicloud.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.Alicloud.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}

	case in.Spec.Packet != nil:
		out.Spec.Type = "packet"

		if len(in.Spec.Packet.Constraints.DNSProviders) > 0 {
			out.Annotations[garden.MigrationCloudProfileDNSProviders] = strings.Join(dnsProviderConstraintToStringSlice(in.Spec.Packet.Constraints.DNSProviders), ",")
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileDNSProviders)
		}
	}

	return nil
}

func Convert_v1alpha1_CloudProfileSpec_To_garden_CloudProfileSpec(in *CloudProfileSpec, out *garden.CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_v1alpha1_CloudProfileSpec_To_garden_CloudProfileSpec(in, out, s)
}

func Convert_garden_CloudProfileSpec_To_v1alpha1_CloudProfileSpec(in *garden.CloudProfileSpec, out *CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_garden_CloudProfileSpec_To_v1alpha1_CloudProfileSpec(in, out, s)
}

func dnsProviderConstraintToStringSlice(dnsConstraints []garden.DNSProviderConstraint) []string {
	out := make([]string, 0, len(dnsConstraints))
	for _, d := range dnsConstraints {
		out = append(out, d.Name)
	}
	return out
}

func stringSliceToDNSProviderConstraint(slice []string) []garden.DNSProviderConstraint {
	dnsConstraints := make([]garden.DNSProviderConstraint, 0, len(slice))
	for _, s := range slice {
		dnsConstraints = append(dnsConstraints, garden.DNSProviderConstraint{Name: s})
	}
	return dnsConstraints
}

func offeredVersionsHaveVersion(offeredVersions []garden.KubernetesVersion, version string) bool {
	for _, v := range offeredVersions {
		if v.Version == version {
			return true
		}
	}
	return false
}

func machineImagesHaveImage(machineImages []garden.MachineImage, name string) bool {
	for _, i := range machineImages {
		if i.Name == name {
			return true
		}
	}
	return false
}

func machineTypesHaveName(machineTypes []garden.MachineType, name string) bool {
	for _, i := range machineTypes {
		if i.Name == name {
			return true
		}
	}
	return false
}

func volumeTypesHaveName(volumeTypes []garden.VolumeType, name string) bool {
	for _, i := range volumeTypes {
		if i.Name == name {
			return true
		}
	}
	return false
}

func alicloudMachineTypesHaveName(machineTypes []garden.AlicloudMachineType, name string) bool {
	for _, i := range machineTypes {
		if i.Name == name {
			return true
		}
	}
	return false
}

func alicloudVolumeTypesHaveName(volumeTypes []garden.AlicloudVolumeType, name string) bool {
	for _, i := range volumeTypes {
		if i.Name == name {
			return true
		}
	}
	return false
}

func openStackMachineTypesHaveName(machineTypes []garden.OpenStackMachineType, name string) bool {
	for _, i := range machineTypes {
		if i.Name == name {
			return true
		}
	}
	return false
}

func zonesHaveName(zones []garden.Zone, name string) bool {
	for _, z := range zones {
		if z.Region == name {
			return true
		}
	}
	return false
}

func zoneHaveAlicloudType(typesPerZone map[string][]string, name, typeName string) bool {
	types, ok := typesPerZone[name]
	if !ok {
		return false
	}

	for _, t := range types {
		if t == typeName {
			return true
		}
	}
	return false
}

func domainCountsHaveRegion(domainCount []garden.AzureDomainCount, regionName string) bool {
	for _, d := range domainCount {
		if d.Region == regionName {
			return true
		}
	}
	return false
}

func loadBalancerProvidersHaveName(providers []garden.OpenStackLoadBalancerProvider, providerName string) bool {
	for _, d := range providers {
		if d.Name == providerName {
			return true
		}
	}
	return false
}

func floatingPoolsHavePool(pools []garden.OpenStackFloatingPool, poolName string) bool {
	for _, d := range pools {
		if d.Name == poolName {
			return true
		}
	}
	return false
}
