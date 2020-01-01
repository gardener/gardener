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

package v1beta1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/utils"

	alicloudinstall "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/install"
	alicloudv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/v1alpha1"
	awsinstall "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/install"
	awsv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/v1alpha1"
	azureinstall "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/v1alpha1"
	gcpinstall "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/install"
	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"
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
	if err := scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", garden.ShootSeedName, garden.ShootCloudProfileName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	); err != nil {
		return err
	}

	// Add non-generated conversion functions
	return scheme.AddConversionFuncs(
		Convert_v1beta1_Seed_To_garden_Seed,
		Convert_garden_Seed_To_v1beta1_Seed,
		Convert_v1beta1_CloudProfile_To_garden_CloudProfile,
		Convert_garden_CloudProfile_To_v1beta1_CloudProfile,
	)
}

func Convert_v1beta1_Seed_To_garden_Seed(in *Seed, out *garden.Seed, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Seed_To_garden_Seed(in, out, s); err != nil {
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

func Convert_garden_Seed_To_v1beta1_Seed(in *garden.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_garden_Seed_To_v1beta1_Seed(in, out, s); err != nil {
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

func Convert_garden_SeedSpec_To_v1beta1_SeedSpec(in *garden.SeedSpec, out *SeedSpec, s conversion.Scope) error {
	return autoConvert_garden_SeedSpec_To_v1beta1_SeedSpec(in, out, s)
}

func Convert_v1beta1_SeedSpec_To_garden_SeedSpec(in *SeedSpec, out *garden.SeedSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_SeedSpec_To_garden_SeedSpec(in, out, s)
}

func Convert_v1beta1_ProjectSpec_To_garden_ProjectSpec(in *ProjectSpec, out *garden.ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ProjectSpec_To_garden_ProjectSpec(in, out, s); err != nil {
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

func Convert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in *garden.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in, out, s); err != nil {
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

func Convert_v1beta1_CloudProfile_To_garden_CloudProfile(in *CloudProfile, out *garden.CloudProfile, s conversion.Scope) error {
	if err := autoConvert_v1beta1_CloudProfile_To_garden_CloudProfile(in, out, s); err != nil {
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
				if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.AWS.Constraints.MachineTypes = append(out.Spec.AWS.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.AWS.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
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
				if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.Azure.Constraints.MachineTypes = append(out.Spec.Azure.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.Azure.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
					return err
				}
				out.Spec.Azure.Constraints.VolumeTypes = append(out.Spec.Azure.Constraints.VolumeTypes, o)
			}
		}

		for _, region := range in.Spec.Regions {
			if !zonesHaveName(out.Spec.Azure.Constraints.Zones, region.Name) {
				z := garden.Zone{Region: region.Name}
				for _, zones := range region.Zones {
					z.Names = append(z.Names, zones.Name)
				}
				out.Spec.Azure.Constraints.Zones = append(out.Spec.Azure.Constraints.Zones, z)
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
			switch {
			case in.Spec.ProviderConfig.Object != nil:
				var ok bool
				cloudProfileConfig, ok = in.Spec.ProviderConfig.Object.(*azurev1alpha1.CloudProfileConfig)
				if !ok {
					klog.Errorf("Cannot cast providerConfig of core.gardener.cloud/v1beta1.CloudProfile %s", in.Name)
				}
			case in.Spec.ProviderConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1beta1.CloudProfile %s", in.Name)
				}
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
				if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.GCP.Constraints.MachineTypes = append(out.Spec.GCP.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.GCP.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
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
				if err := Convert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				t := garden.OpenStackMachineType{MachineType: o}
				if machineType.Storage != nil {
					t.Storage = &garden.MachineTypeStorage{
						Class: machineType.Storage.Class,
						Size:  machineType.Storage.Size,
						Type:  machineType.Storage.Type,
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
			switch {
			case in.Spec.ProviderConfig.Object != nil:
				var ok bool
				cloudProfileConfig, ok = in.Spec.ProviderConfig.Object.(*openstackv1alpha1.CloudProfileConfig)
				if !ok {
					klog.Errorf("Cannot cast providerConfig of core.gardener.cloud/v1beta1.CloudProfile %s", in.Name)
				}
			case in.Spec.ProviderConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.ProviderConfig.Raw, nil, cloudProfileConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1beta1.CloudProfile %s", in.Name)
				}
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
				if err := Convert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
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
				if err := Convert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
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
				if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
					return err
				}
				out.Spec.Packet.Constraints.MachineTypes = append(out.Spec.Packet.Constraints.MachineTypes, o)
			}
		}

		for _, volumeType := range in.Spec.VolumeTypes {
			if !volumeTypesHaveName(out.Spec.Packet.Constraints.VolumeTypes, volumeType.Name) {
				var o garden.VolumeType
				if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
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

func Convert_garden_CloudProfile_To_v1beta1_CloudProfile(in *garden.CloudProfile, out *CloudProfile, s conversion.Scope) error {
	if err := autoConvert_garden_CloudProfile_To_v1beta1_CloudProfile(in, out, s); err != nil {
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
		out.Spec.VolumeTypes = nil

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

func Convert_v1beta1_CloudProfileSpec_To_garden_CloudProfileSpec(in *CloudProfileSpec, out *garden.CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_CloudProfileSpec_To_garden_CloudProfileSpec(in, out, s)
}

func Convert_garden_CloudProfileSpec_To_v1beta1_CloudProfileSpec(in *garden.CloudProfileSpec, out *CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_garden_CloudProfileSpec_To_v1beta1_CloudProfileSpec(in, out, s)
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

func Convert_v1beta1_Shoot_To_garden_Shoot(in *Shoot, out *garden.Shoot, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Shoot_To_garden_Shoot(in, out, s); err != nil {
		return err
	}

	var addonClusterAutoScaler *garden.AddonClusterAutoscaler
	if data, ok := in.Annotations[garden.MigrationShootAddonsClusterAutoscaler]; ok {
		addonClusterAutoScaler = &garden.AddonClusterAutoscaler{}
		if err := json.Unmarshal([]byte(data), addonClusterAutoScaler); err != nil {
			return err
		}
	}
	out.Spec.Addons.ClusterAutoscaler = addonClusterAutoScaler

	var addonHeapster *garden.Heapster
	if data, ok := in.Annotations[garden.MigrationShootAddonsHeapster]; ok {
		addonHeapster = &garden.Heapster{}
		if err := json.Unmarshal([]byte(data), addonHeapster); err != nil {
			return err
		}
	}
	out.Spec.Addons.Heapster = addonHeapster

	var addonKubeLego *garden.KubeLego
	if data, ok := in.Annotations[garden.MigrationShootAddonsKubeLego]; ok {
		addonKubeLego = &garden.KubeLego{}
		if err := json.Unmarshal([]byte(data), addonKubeLego); err != nil {
			return err
		}
	}
	out.Spec.Addons.KubeLego = addonKubeLego

	var addonKube2IAM *garden.Kube2IAM
	if data, ok := in.Annotations[garden.MigrationShootAddonsKube2IAM]; ok {
		addonKube2IAM = &garden.Kube2IAM{}
		if err := json.Unmarshal([]byte(data), addonKube2IAM); err != nil {
			return err
		}
	}
	out.Spec.Addons.Kube2IAM = addonKube2IAM

	var addonMonocular *garden.Monocular
	if data, ok := in.Annotations[garden.MigrationShootAddonsMonocular]; ok {
		addonMonocular = &garden.Monocular{}
		if err := json.Unmarshal([]byte(data), addonMonocular); err != nil {
			return err
		}
	}
	out.Spec.Addons.Monocular = addonMonocular

	switch in.Spec.Provider.Type {
	case "aws":
		if out.Spec.Cloud.AWS == nil {
			out.Spec.Cloud.AWS = &garden.AWSCloud{}
		}

		extensionsScheme := runtime.NewScheme()
		if err := awsinstall.AddToScheme(extensionsScheme); err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()

		infrastructureConfig := &awsv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}
		if in.Spec.Provider.InfrastructureConfig != nil {
			switch {
			case in.Spec.Provider.InfrastructureConfig.Object != nil:
				var ok bool
				infrastructureConfig, ok = in.Spec.Provider.InfrastructureConfig.Object.(*awsv1alpha1.InfrastructureConfig)
				if !ok {
					klog.Errorf("Cannot cast infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			case in.Spec.Provider.InfrastructureConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			}
		}

		out.Spec.Cloud.AWS.Zones = nil
		out.Spec.Cloud.AWS.Networks.Internal = nil
		out.Spec.Cloud.AWS.Networks.Public = nil
		out.Spec.Cloud.AWS.Networks.Workers = nil
		for _, zone := range infrastructureConfig.Networks.Zones {
			out.Spec.Cloud.AWS.Zones = append(out.Spec.Cloud.AWS.Zones, zone.Name)
			out.Spec.Cloud.AWS.Networks.Internal = append(out.Spec.Cloud.AWS.Networks.Internal, zone.Internal)
			out.Spec.Cloud.AWS.Networks.Public = append(out.Spec.Cloud.AWS.Networks.Public, zone.Public)
			out.Spec.Cloud.AWS.Networks.Workers = append(out.Spec.Cloud.AWS.Networks.Workers, zone.Workers)
		}
		out.Spec.Cloud.AWS.Networks.VPC.CIDR = infrastructureConfig.Networks.VPC.CIDR
		out.Spec.Cloud.AWS.Networks.VPC.ID = infrastructureConfig.Networks.VPC.ID
		out.Spec.Cloud.AWS.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.AWS.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.AWS.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.AWS.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.AWS.MachineImage = nil
		}

		controlPlaneConfig := &awsv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}
		if in.Spec.Provider.ControlPlaneConfig != nil {
			switch {
			case in.Spec.Provider.ControlPlaneConfig.Object != nil:
				var ok bool
				controlPlaneConfig, ok = in.Spec.Provider.ControlPlaneConfig.Object.(*awsv1alpha1.ControlPlaneConfig)
				if !ok {
					klog.Errorf("Cannot cast controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			case in.Spec.Provider.ControlPlaneConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.ControlPlaneConfig.Raw, nil, controlPlaneConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			}
		}

		if controlPlaneConfig.CloudControllerManager == nil {
			out.Spec.Kubernetes.CloudControllerManager = nil
		} else {
			out.Spec.Kubernetes.CloudControllerManager = &garden.CloudControllerManagerConfig{
				KubernetesConfig: garden.KubernetesConfig{
					FeatureGates: controlPlaneConfig.CloudControllerManager.FeatureGates,
				},
			}
		}

		out.Spec.Cloud.AWS.Workers = nil
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.AWS.Workers = append(out.Spec.Cloud.AWS.Workers, o)
		}

	case "azure":
		if out.Spec.Cloud.Azure == nil {
			out.Spec.Cloud.Azure = &garden.AzureCloud{}
		}

		extensionsScheme := runtime.NewScheme()
		if err := azureinstall.AddToScheme(extensionsScheme); err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()

		infrastructureConfig := &azurev1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}
		if in.Spec.Provider.InfrastructureConfig != nil {
			switch {
			case in.Spec.Provider.InfrastructureConfig.Object != nil:
				var ok bool
				infrastructureConfig, ok = in.Spec.Provider.InfrastructureConfig.Object.(*azurev1alpha1.InfrastructureConfig)
				if !ok {
					klog.Errorf("Cannot cast infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			case in.Spec.Provider.InfrastructureConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			}
		}

		out.Spec.Cloud.Azure.ResourceGroup = nil
		if infrastructureConfig.ResourceGroup != nil {
			out.Spec.Cloud.Azure.ResourceGroup = &garden.AzureResourceGroup{
				Name: infrastructureConfig.ResourceGroup.Name,
			}
		}

		out.Spec.Cloud.Azure.Networks.Workers = infrastructureConfig.Networks.Workers
		out.Spec.Cloud.Azure.Networks.VNet.CIDR = infrastructureConfig.Networks.VNet.CIDR
		out.Spec.Cloud.Azure.Networks.VNet.Name = infrastructureConfig.Networks.VNet.Name
		out.Spec.Cloud.Azure.Networks.VNet.ResourceGroup = infrastructureConfig.Networks.VNet.ResourceGroup
		out.Spec.Cloud.Azure.Networks.ServiceEndpoints = infrastructureConfig.Networks.ServiceEndpoints
		out.Spec.Cloud.Azure.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.Azure.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.Azure.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.Azure.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.Azure.MachineImage = nil
		}

		controlPlaneConfig := &azurev1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}
		if in.Spec.Provider.ControlPlaneConfig != nil {
			switch {
			case in.Spec.Provider.ControlPlaneConfig.Object != nil:
				var ok bool
				controlPlaneConfig, ok = in.Spec.Provider.ControlPlaneConfig.Object.(*azurev1alpha1.ControlPlaneConfig)
				if !ok {
					klog.Errorf("Cannot cast controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			case in.Spec.Provider.ControlPlaneConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.ControlPlaneConfig.Raw, nil, controlPlaneConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			}
		}

		if controlPlaneConfig.CloudControllerManager == nil {
			out.Spec.Kubernetes.CloudControllerManager = nil
		} else {
			out.Spec.Kubernetes.CloudControllerManager = &garden.CloudControllerManagerConfig{
				KubernetesConfig: garden.KubernetesConfig{
					FeatureGates: controlPlaneConfig.CloudControllerManager.FeatureGates,
				},
			}
		}

		out.Spec.Cloud.Azure.Zones = nil
		out.Spec.Cloud.Azure.Workers = nil
		zones := sets.NewString()
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.Azure.Workers = append(out.Spec.Cloud.Azure.Workers, o)
			for _, zone := range o.Zones {
				if !zones.Has(zone) {
					out.Spec.Cloud.Azure.Zones = append(out.Spec.Cloud.Azure.Zones, zone)
					zones.Insert(zone)
				}
			}
		}

	case "gcp":
		if out.Spec.Cloud.GCP == nil {
			out.Spec.Cloud.GCP = &garden.GCPCloud{}
		}

		extensionsScheme := runtime.NewScheme()
		if err := gcpinstall.AddToScheme(extensionsScheme); err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()

		infrastructureConfig := &gcpv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}
		if in.Spec.Provider.InfrastructureConfig != nil {
			switch {
			case in.Spec.Provider.InfrastructureConfig.Object != nil:
				var ok bool
				infrastructureConfig, ok = in.Spec.Provider.InfrastructureConfig.Object.(*gcpv1alpha1.InfrastructureConfig)
				if !ok {
					klog.Errorf("Cannot cast infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			case in.Spec.Provider.InfrastructureConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			}
		}

		if infrastructureConfig.Networks.VPC != nil {
			out.Spec.Cloud.GCP.Networks.VPC = &garden.GCPVPC{
				Name: infrastructureConfig.Networks.VPC.Name,
			}
		}

		out.Spec.Cloud.GCP.Networks.Internal = infrastructureConfig.Networks.Internal
		out.Spec.Cloud.GCP.Networks.Workers = []string{infrastructureConfig.Networks.Worker}
		out.Spec.Cloud.GCP.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.GCP.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.GCP.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.GCP.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.GCP.MachineImage = nil
		}

		controlPlaneConfig := &gcpv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}
		if in.Spec.Provider.ControlPlaneConfig != nil {
			switch {
			case in.Spec.Provider.ControlPlaneConfig.Object != nil:
				var ok bool
				controlPlaneConfig, ok = in.Spec.Provider.ControlPlaneConfig.Object.(*gcpv1alpha1.ControlPlaneConfig)
				if !ok {
					klog.Errorf("Cannot cast controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			case in.Spec.Provider.ControlPlaneConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.ControlPlaneConfig.Raw, nil, controlPlaneConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			}
		}

		if controlPlaneConfig.CloudControllerManager == nil {
			out.Spec.Kubernetes.CloudControllerManager = nil
		} else {
			out.Spec.Kubernetes.CloudControllerManager = &garden.CloudControllerManagerConfig{
				KubernetesConfig: garden.KubernetesConfig{
					FeatureGates: controlPlaneConfig.CloudControllerManager.FeatureGates,
				},
			}
		}

		out.Spec.Cloud.GCP.Workers = nil
		zones := sets.NewString()
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.GCP.Workers = append(out.Spec.Cloud.GCP.Workers, o)
			for _, zone := range o.Zones {
				if !zones.Has(zone) {
					out.Spec.Cloud.GCP.Zones = append(out.Spec.Cloud.GCP.Zones, zone)
					zones.Insert(zone)
				}
			}
		}

	case "openstack":
		if out.Spec.Cloud.OpenStack == nil {
			out.Spec.Cloud.OpenStack = &garden.OpenStackCloud{}
		}

		extensionsScheme := runtime.NewScheme()
		if err := openstackinstall.AddToScheme(extensionsScheme); err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()

		infrastructureConfig := &openstackv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}
		if in.Spec.Provider.InfrastructureConfig != nil {
			switch {
			case in.Spec.Provider.InfrastructureConfig.Object != nil:
				var ok bool
				infrastructureConfig, ok = in.Spec.Provider.InfrastructureConfig.Object.(*openstackv1alpha1.InfrastructureConfig)
				if !ok {
					klog.Errorf("Cannot cast infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			case in.Spec.Provider.InfrastructureConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			}
		}

		if infrastructureConfig.Networks.Router != nil {
			out.Spec.Cloud.OpenStack.Networks.Router = &garden.OpenStackRouter{
				ID: infrastructureConfig.Networks.Router.ID,
			}
		}
		out.Spec.Cloud.OpenStack.FloatingPoolName = infrastructureConfig.FloatingPoolName
		out.Spec.Cloud.OpenStack.Networks.Workers = []string{infrastructureConfig.Networks.Worker}
		out.Spec.Cloud.OpenStack.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.OpenStack.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.OpenStack.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.OpenStack.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.OpenStack.MachineImage = nil
		}

		controlPlaneConfig := &openstackv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}
		if in.Spec.Provider.ControlPlaneConfig != nil {
			switch {
			case in.Spec.Provider.ControlPlaneConfig.Object != nil:
				var ok bool
				controlPlaneConfig, ok = in.Spec.Provider.ControlPlaneConfig.Object.(*openstackv1alpha1.ControlPlaneConfig)
				if !ok {
					klog.Errorf("Cannot cast controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			case in.Spec.Provider.ControlPlaneConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.ControlPlaneConfig.Raw, nil, controlPlaneConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			}
		}

		if controlPlaneConfig.CloudControllerManager == nil {
			out.Spec.Kubernetes.CloudControllerManager = nil
		} else {
			out.Spec.Kubernetes.CloudControllerManager = &garden.CloudControllerManagerConfig{
				KubernetesConfig: garden.KubernetesConfig{
					FeatureGates: controlPlaneConfig.CloudControllerManager.FeatureGates,
				},
			}
		}

		var loadBalancerClasses = make([]garden.OpenStackLoadBalancerClass, 0, len(controlPlaneConfig.LoadBalancerClasses))
		for _, loadBalancerClass := range controlPlaneConfig.LoadBalancerClasses {
			loadBalancerClasses = append(loadBalancerClasses, garden.OpenStackLoadBalancerClass{
				Name:              loadBalancerClass.Name,
				FloatingSubnetID:  loadBalancerClass.FloatingSubnetID,
				FloatingNetworkID: loadBalancerClass.FloatingNetworkID,
				SubnetID:          loadBalancerClass.SubnetID,
			})
		}
		out.Spec.Cloud.OpenStack.LoadBalancerClasses = loadBalancerClasses
		out.Spec.Cloud.OpenStack.LoadBalancerProvider = controlPlaneConfig.LoadBalancerProvider

		out.Spec.Cloud.OpenStack.Workers = nil
		zones := sets.NewString()
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.OpenStack.Workers = append(out.Spec.Cloud.OpenStack.Workers, o)
			for _, zone := range o.Zones {
				if !zones.Has(zone) {
					out.Spec.Cloud.OpenStack.Zones = append(out.Spec.Cloud.OpenStack.Zones, zone)
					zones.Insert(zone)
				}
			}
		}

	case "alicloud":
		if out.Spec.Cloud.Alicloud == nil {
			out.Spec.Cloud.Alicloud = &garden.Alicloud{}
		}

		extensionsScheme := runtime.NewScheme()
		if err := alicloudinstall.AddToScheme(extensionsScheme); err != nil {
			return err
		}
		decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()

		infrastructureConfig := &alicloudv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}
		if in.Spec.Provider.InfrastructureConfig != nil {
			switch {
			case in.Spec.Provider.InfrastructureConfig.Object != nil:
				var ok bool
				infrastructureConfig, ok = in.Spec.Provider.InfrastructureConfig.Object.(*alicloudv1alpha1.InfrastructureConfig)
				if !ok {
					klog.Errorf("Cannot cast infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			case in.Spec.Provider.InfrastructureConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.InfrastructureConfig.Raw, nil, infrastructureConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode infrastructureConfig of core.gardener.cloud.Shoot %s", in.Name)
				}
			}
		}

		out.Spec.Cloud.Alicloud.Zones = nil
		out.Spec.Cloud.Alicloud.Networks.Workers = nil
		for _, zone := range infrastructureConfig.Networks.Zones {
			out.Spec.Cloud.Alicloud.Zones = append(out.Spec.Cloud.Alicloud.Zones, zone.Name)
			out.Spec.Cloud.Alicloud.Networks.Workers = append(out.Spec.Cloud.Alicloud.Networks.Workers, zone.Worker)
		}
		out.Spec.Cloud.Alicloud.Networks.VPC.CIDR = infrastructureConfig.Networks.VPC.CIDR
		out.Spec.Cloud.Alicloud.Networks.VPC.ID = infrastructureConfig.Networks.VPC.ID
		out.Spec.Cloud.Alicloud.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.Alicloud.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.Alicloud.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.Alicloud.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.Alicloud.MachineImage = nil
		}

		controlPlaneConfig := &alicloudv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}
		if in.Spec.Provider.ControlPlaneConfig != nil {
			switch {
			case in.Spec.Provider.ControlPlaneConfig.Object != nil:
				var ok bool
				controlPlaneConfig, ok = in.Spec.Provider.ControlPlaneConfig.Object.(*alicloudv1alpha1.ControlPlaneConfig)
				if !ok {
					klog.Errorf("Cannot cast controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			case in.Spec.Provider.ControlPlaneConfig.Raw != nil:
				if _, _, err := decoder.Decode(in.Spec.Provider.ControlPlaneConfig.Raw, nil, controlPlaneConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode controlPlaneConfig of garden.sapcloud.io.Shoot %s", in.Name)
				}
			}
		}

		if controlPlaneConfig.CloudControllerManager == nil {
			out.Spec.Kubernetes.CloudControllerManager = nil
		} else {
			out.Spec.Kubernetes.CloudControllerManager = &garden.CloudControllerManagerConfig{
				KubernetesConfig: garden.KubernetesConfig{
					FeatureGates: controlPlaneConfig.CloudControllerManager.FeatureGates,
				},
			}
		}

		out.Spec.Cloud.Alicloud.Workers = nil
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.Alicloud.Workers = append(out.Spec.Cloud.Alicloud.Workers, o)
		}

	case "packet":
		if out.Spec.Cloud.Packet == nil {
			out.Spec.Cloud.Packet = &garden.PacketCloud{}
		}

		out.Spec.Cloud.Packet.Zones = nil
		out.Spec.Cloud.Packet.Networks.Pods = in.Spec.Networking.Pods
		out.Spec.Cloud.Packet.Networks.Services = in.Spec.Networking.Services
		out.Spec.Cloud.Packet.Networks.Nodes = &in.Spec.Networking.Nodes

		if data, ok := in.Annotations[garden.MigrationShootGlobalMachineImage]; ok {
			var machineImage garden.ShootMachineImage
			if err := json.Unmarshal([]byte(data), &machineImage); err != nil {
				return err
			}
			out.Spec.Cloud.Packet.MachineImage = &machineImage
		} else {
			out.Spec.Cloud.Packet.MachineImage = nil
		}

		out.Spec.Cloud.Packet.Workers = nil
		zones := sets.NewString()
		for _, worker := range in.Spec.Provider.Workers {
			var o garden.Worker
			if err := autoConvert_v1beta1_Worker_To_garden_Worker(&worker, &o, s); err != nil {
				return err
			}
			out.Spec.Cloud.Packet.Workers = append(out.Spec.Cloud.Packet.Workers, o)
			for _, zone := range o.Zones {
				if !zones.Has(zone) {
					out.Spec.Cloud.Packet.Zones = append(out.Spec.Cloud.Packet.Zones, zone)
					zones.Insert(zone)
				}
			}
		}

		var cloudControllerManager *garden.CloudControllerManagerConfig
		if data, ok := in.Annotations[garden.MigrationShootCloudControllerManager]; ok {
			cloudControllerManager = &garden.CloudControllerManagerConfig{}
			if err := json.Unmarshal([]byte(data), cloudControllerManager); err != nil {
				return err
			}
		}
		out.Spec.Kubernetes.CloudControllerManager = cloudControllerManager
	}

	out.Spec.Cloud.Profile = in.Spec.CloudProfileName
	out.Spec.Cloud.Region = in.Spec.Region
	out.Spec.Cloud.SecretBindingRef.Name = in.Spec.SecretBindingName
	out.Spec.Cloud.Seed = in.Spec.SeedName

	return nil
}

func Convert_garden_Shoot_To_v1beta1_Shoot(in *garden.Shoot, out *Shoot, s conversion.Scope) error {
	if err := autoConvert_garden_Shoot_To_v1beta1_Shoot(in, out, s); err != nil {
		return err
	}

	var addons *Addons
	if in.Spec.Addons != nil {
		addons = &Addons{}
		if err := autoConvert_garden_Addons_To_v1beta1_Addons(in.Spec.Addons, addons, s); err != nil {
			return err
		}

		if in.Spec.Addons.ClusterAutoscaler != nil {
			data, err := json.Marshal(in.Spec.Addons.ClusterAutoscaler)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootAddonsClusterAutoscaler, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootAddonsClusterAutoscaler)
		}
		if in.Spec.Addons.Heapster != nil {
			data, err := json.Marshal(in.Spec.Addons.Heapster)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootAddonsHeapster, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootAddonsHeapster)
		}
		if in.Spec.Addons.Kube2IAM != nil {
			data, err := json.Marshal(in.Spec.Addons.Kube2IAM)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootAddonsKube2IAM, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootAddonsKube2IAM)
		}
		if in.Spec.Addons.KubeLego != nil {
			data, err := json.Marshal(in.Spec.Addons.KubeLego)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootAddonsKubeLego, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootAddonsKubeLego)
		}
		if in.Spec.Addons.Monocular != nil {
			data, err := json.Marshal(in.Spec.Addons.Monocular)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootAddonsMonocular, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootAddonsMonocular)
		}
	}

	out.Spec.CloudProfileName = in.Spec.Cloud.Profile
	out.Spec.Region = in.Spec.Cloud.Region
	out.Spec.SecretBindingName = in.Spec.Cloud.SecretBindingRef.Name
	out.Spec.SeedName = in.Spec.Cloud.Seed

	if email, ok := in.Annotations[constants.AnnotationShootOperatedBy]; ok && utils.TestEmail(email) {
		exists := false
		if in.Spec.Monitoring == nil {
			out.Spec.Monitoring = &Monitoring{
				Alerting: &Alerting{},
			}
		}
		if in.Spec.Monitoring != nil && in.Spec.Monitoring.Alerting == nil {
			out.Spec.Monitoring.Alerting = &Alerting{}
		}
		if in.Spec.Monitoring != nil && in.Spec.Monitoring.Alerting != nil {
			for _, receiver := range in.Spec.Monitoring.Alerting.EmailReceivers {
				if receiver == email {
					exists = true
					break
				}
			}
		}
		if !exists {
			out.Spec.Monitoring.Alerting.EmailReceivers = append(out.Spec.Monitoring.Alerting.EmailReceivers, email)
		}
		// Always delete annotation (Either email gets appended to emailReceivers or email already exists).
		delete(out.Annotations, constants.AnnotationShootOperatedBy)
	}

	switch in.Spec.Provider.Type {
	case "aws":
		if in.Spec.Cloud.AWS != nil && in.Spec.Cloud.AWS.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.AWS.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

	case "azure":
		if in.Spec.Cloud.Azure != nil && in.Spec.Cloud.Azure.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.Azure.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

	case "gcp":
		if in.Spec.Cloud.GCP != nil && in.Spec.Cloud.GCP.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.GCP.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

	case "openstack":
		if in.Spec.Cloud.OpenStack != nil && in.Spec.Cloud.OpenStack.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.OpenStack.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

	case "alicloud":
		if in.Spec.Cloud.Alicloud != nil && in.Spec.Cloud.Alicloud.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.Alicloud.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

	case "packet":
		if in.Spec.Cloud.Packet != nil && in.Spec.Cloud.Packet.MachineImage != nil {
			data, err := json.Marshal(in.Spec.Cloud.Packet.MachineImage)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootGlobalMachineImage, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootGlobalMachineImage)
		}

		if in.Spec.Kubernetes.CloudControllerManager != nil {
			data, err := json.Marshal(in.Spec.Kubernetes.CloudControllerManager)
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootCloudControllerManager, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootCloudControllerManager)
		}
	}

	return nil
}

func Convert_garden_ShootSpec_To_v1beta1_ShootSpec(in *garden.ShootSpec, out *ShootSpec, s conversion.Scope) error {
	return autoConvert_garden_ShootSpec_To_v1beta1_ShootSpec(in, out, s)
}

func Convert_v1beta1_ShootSpec_To_garden_ShootSpec(in *ShootSpec, out *garden.ShootSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_ShootSpec_To_garden_ShootSpec(in, out, s)
}

func Convert_garden_ShootStatus_To_v1beta1_ShootStatus(in *garden.ShootStatus, out *ShootStatus, s conversion.Scope) error {
	if err := autoConvert_garden_ShootStatus_To_v1beta1_ShootStatus(in, out, s); err != nil {
		return err
	}

	if in.IsHibernated == nil {
		out.IsHibernated = false
	} else {
		out.IsHibernated = *in.IsHibernated
	}

	return nil
}

func Convert_v1beta1_ShootStatus_To_garden_ShootStatus(in *ShootStatus, out *garden.ShootStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ShootStatus_To_garden_ShootStatus(in, out, s); err != nil {
		return err
	}

	out.IsHibernated = &in.IsHibernated

	return nil
}

func Convert_garden_Addons_To_v1beta1_Addons(in *garden.Addons, out *Addons, s conversion.Scope) error {
	return autoConvert_garden_Addons_To_v1beta1_Addons(in, out, s)
}

func Convert_v1beta1_Addons_To_garden_Addons(in *Addons, out *garden.Addons, s conversion.Scope) error {
	return autoConvert_v1beta1_Addons_To_garden_Addons(in, out, s)
}

func Convert_garden_Kubernetes_To_v1beta1_Kubernetes(in *garden.Kubernetes, out *Kubernetes, s conversion.Scope) error {
	return autoConvert_garden_Kubernetes_To_v1beta1_Kubernetes(in, out, s)
}

func Convert_v1beta1_Kubernetes_To_garden_Kubernetes(in *Kubernetes, out *garden.Kubernetes, s conversion.Scope) error {
	return autoConvert_v1beta1_Kubernetes_To_garden_Kubernetes(in, out, s)
}
