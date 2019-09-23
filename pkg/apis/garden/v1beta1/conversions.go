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

	gardencorev1alpha1helper "github.com/gardener/gardener/pkg/apis/core/v1alpha1/helper"
	"github.com/gardener/gardener/pkg/apis/garden"
	"github.com/gardener/gardener/pkg/apis/garden/helper"

	alicloudv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-alicloud/pkg/apis/alicloud/v1alpha1"
	awsv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-aws/pkg/apis/aws/v1alpha1"
	azureinstall "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/v1alpha1"
	gcpv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-gcp/pkg/apis/gcp/v1alpha1"
	openstackinstall "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/v1alpha1"
	packetv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-packet/pkg/apis/packet/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/klog"
)

func init() {
	localSchemeBuilder.Register(addConversionFuncs)
}

// Convert_v1beta1_MachineVersion_To_garden_MachineVersion
func Convert_v1beta1_MachineImage_To_garden_MachineImage(in *MachineImage, out *garden.MachineImage, s conversion.Scope) error {
	if err := autoConvert_v1beta1_MachineImage_To_garden_MachineImage(in, out, s); err != nil {
		return err
	}

	if len(in.Version) > 0 {
		out.Versions = make([]garden.MachineImageVersion, len(in.Versions)+1)
		out.Versions[0] = garden.MachineImageVersion{
			Version: in.Version,
		}
	} else {
		out.Versions = make([]garden.MachineImageVersion, len(in.Versions))
	}

	for index, externalVersion := range in.Versions {
		internalVersion := &garden.MachineImageVersion{}
		if err := autoConvert_v1beta1_MachineImageVersion_To_garden_MachineImageVersion(&externalVersion, internalVersion, s); err != nil {
			return err
		}
		if len(in.Version) > 0 {
			out.Versions[index+1] = *internalVersion
		} else {
			out.Versions[index] = *internalVersion
		}
	}

	return nil
}

// Convert_garden_MachineImage_To_v1beta1_MachineImage
func Convert_garden_MachineImage_To_v1beta1_MachineImage(in *garden.MachineImage, out *MachineImage, s conversion.Scope) error {
	if err := autoConvert_garden_MachineImage_To_v1beta1_MachineImage(in, out, s); err != nil {
		return err
	}

	out.Versions = make([]MachineImageVersion, len(in.Versions))
	for index, internalVersion := range in.Versions {
		externalVersion := &MachineImageVersion{}
		if err := autoConvert_garden_MachineImageVersion_To_v1beta1_MachineImageVersion(&internalVersion, externalVersion, s); err != nil {
			return err
		}
		out.Versions[index] = *externalVersion
	}

	return nil
}

func Convert_v1beta1_KubernetesConstraints_To_garden_KubernetesConstraints(in *KubernetesConstraints, out *garden.KubernetesConstraints, s conversion.Scope) error {
	out.OfferedVersions = []garden.KubernetesVersion{}
	duplicates := map[string]int{}
	for index, externalVersion := range in.Versions {
		internalVersion := &garden.KubernetesVersion{Version: externalVersion}
		if _, exists := duplicates[externalVersion]; exists {
			continue
		}
		out.OfferedVersions = append(out.OfferedVersions, *internalVersion)
		duplicates[externalVersion] = index
	}
	for _, externalVersion := range in.OfferedVersions {
		internalVersion := &garden.KubernetesVersion{}
		if err := Convert_v1beta1_KubernetesVersion_To_garden_KubernetesVersion(&externalVersion, internalVersion, s); err != nil {
			return err
		}
		if _, exists := duplicates[externalVersion.Version]; exists {
			if externalVersion.ExpirationDate == nil {
				continue
			}
			out.OfferedVersions[duplicates[externalVersion.Version]].ExpirationDate = externalVersion.ExpirationDate
			continue
		}
		out.OfferedVersions = append(out.OfferedVersions, *internalVersion)
	}
	return nil
}

func Convert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(in *garden.KubernetesConstraints, out *KubernetesConstraints, s conversion.Scope) error {
	if err := autoConvert_garden_KubernetesConstraints_To_v1beta1_KubernetesConstraints(in, out, s); err != nil {
		return err
	}

	for _, version := range in.OfferedVersions {
		out.Versions = append(out.Versions, version.Version)
	}

	return nil
}

func addConversionFuncs(scheme *runtime.Scheme) error {
	return scheme.AddFieldLabelConversionFunc(SchemeGroupVersion.WithKind("Shoot"),
		func(label, value string) (string, string, error) {
			switch label {
			case "metadata.name", "metadata.namespace", garden.ShootSeedName:
				return label, value, nil
			default:
				return "", "", fmt.Errorf("field label not supported: %s", label)
			}
		},
	)
}

func Convert_v1beta1_Seed_To_garden_Seed(in *Seed, out *garden.Seed, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Seed_To_garden_Seed(in, out, s); err != nil {
		return err
	}

	if a := in.Annotations; a != nil {
		if v, ok := a[garden.MigrationSeedProviderType]; ok {
			out.Spec.Provider.Type = v
		}
		if v, ok := a[garden.MigrationSeedProviderRegion]; ok {
			out.Spec.Provider.Region = v
		}

		if v, ok := a[garden.MigrationSeedTaints]; ok {
			for _, key := range strings.Split(v, ",") {
				out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
					Key: key,
				})
			}
		}

		volumeMinimumSize, ok := a[garden.MigrationSeedVolumeMinimumSize]
		volumeProviders, ok2 := a[garden.MigrationSeedVolumeProviders]
		legacyVolumeMinimumSizeAnnotationValue, ok3 := a["persistentvolume.garden.sapcloud.io/minimumSize"]
		legacyVolumeProviderAnnotationValue, ok4 := a["persistentvolume.garden.sapcloud.io/provider"]

		if ok || ok2 || ok3 || ok4 {
			out.Spec.Volume = &garden.SeedVolume{}
		}

		if ok {
			quantity, err := resource.ParseQuantity(volumeMinimumSize)
			if err != nil {
				return err
			}
			out.Spec.Volume.MinimumSize = &quantity
		}
		if ok3 {
			quantity, err := resource.ParseQuantity(legacyVolumeMinimumSizeAnnotationValue)
			if err != nil {
				return err
			}
			out.Spec.Volume.MinimumSize = &quantity
		}

		if ok4 {
			out.Spec.Volume.Providers = append(out.Spec.Volume.Providers, garden.SeedVolumeProvider{
				Purpose: garden.SeedVolumeProviderPurposeEtcdMain,
				Name:    legacyVolumeProviderAnnotationValue,
			})
		}
		if ok2 {
			var obj []garden.SeedVolumeProvider
			if err := json.Unmarshal([]byte(volumeProviders), &obj); err != nil {
				return err
			}

			out.Spec.Volume.Providers = append(out.Spec.Volume.Providers, obj...)
		}
	}

	out.Spec.Provider.Region = in.Spec.Cloud.Region

	if p := in.Spec.Protected; p != nil && *p && !helper.TaintsHave(out.Spec.Taints, garden.SeedTaintProtected) {
		out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
			Key: garden.SeedTaintProtected,
		})
	}

	if v := in.Spec.Visible; v != nil && !*v && !helper.TaintsHave(out.Spec.Taints, garden.SeedTaintInvisible) {
		out.Spec.Taints = append(out.Spec.Taints, garden.SeedTaint{
			Key: garden.SeedTaintInvisible,
		})
	}

	return nil
}

func Convert_garden_Seed_To_v1beta1_Seed(in *garden.Seed, out *Seed, s conversion.Scope) error {
	if err := autoConvert_garden_Seed_To_v1beta1_Seed(in, out, s); err != nil {
		return err
	}

	if len(in.Spec.Provider.Type) > 0 || len(in.Spec.Provider.Region) > 0 || in.Spec.Volume != nil {
		old := out.Annotations
		out.Annotations = make(map[string]string, len(old)+3)
		for k, v := range old {
			out.Annotations[k] = v
		}
	}

	if len(in.Spec.Provider.Type) > 0 {
		out.Annotations[garden.MigrationSeedProviderType] = in.Spec.Provider.Type
	}

	if len(in.Spec.Provider.Region) > 0 {
		out.Annotations[garden.MigrationSeedProviderRegion] = in.Spec.Provider.Region
	}

	if v := in.Spec.Volume; v != nil {
		if v.MinimumSize != nil {
			out.Annotations[garden.MigrationSeedVolumeMinimumSize] = v.MinimumSize.String()
			out.Annotations["persistentvolume.garden.sapcloud.io/minimumSize"] = v.MinimumSize.String()
		}

		var volumeProviders []garden.SeedVolumeProvider
		for _, provider := range in.Spec.Volume.Providers {
			if provider.Purpose == garden.SeedVolumeProviderPurposeEtcdMain {
				out.Annotations["persistentvolume.garden.sapcloud.io/provider"] = provider.Name
			} else {
				volumeProviders = append(volumeProviders, provider)
			}
		}

		if len(volumeProviders) > 0 {
			data, err := json.Marshal(volumeProviders)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationSeedVolumeProviders] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationSeedVolumeProviders)
		}
	}

	var (
		trueVar   = true
		falseVar  = false
		taintKeys []string
	)

	for _, taint := range in.Spec.Taints {
		taintKeys = append(taintKeys, taint.Key)

		switch taint.Key {
		case garden.SeedTaintProtected:
			out.Spec.Protected = &trueVar
		case garden.SeedTaintInvisible:
			out.Spec.Visible = &falseVar
		}
	}

	if len(taintKeys) > 0 {
		out.Annotations[garden.MigrationSeedTaints] = strings.Join(taintKeys, ",")
	} else {
		delete(out.Annotations, garden.MigrationSeedTaints)
	}

	if out.Spec.Visible == nil {
		out.Spec.Visible = &trueVar
	}
	if out.Spec.Protected == nil {
		out.Spec.Protected = &falseVar
	}

	var (
		defaultPodCIDR             = DefaultPodNetworkCIDR
		defaultServiceCIDR         = DefaultServiceNetworkCIDR
		defaultPodCIDRAlicloud     = DefaultPodNetworkCIDRAlicloud
		defaultServiceCIDRAlicloud = DefaultServiceNetworkCIDRAlicloud
	)

	if out.Spec.Networks.ShootDefaults == nil {
		out.Spec.Networks.ShootDefaults = &ShootNetworks{}
	}

	if v, ok := out.Annotations[garden.MigrationSeedProviderType]; ok && v == "alicloud" {
		if out.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Pods, defaultPodCIDRAlicloud) {
			out.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDRAlicloud
		}
		if out.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Services, defaultServiceCIDRAlicloud) {
			out.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDRAlicloud
		}
	} else {
		if out.Spec.Networks.ShootDefaults.Pods == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Pods, defaultPodCIDR) {
			out.Spec.Networks.ShootDefaults.Pods = &defaultPodCIDR
		}
		if out.Spec.Networks.ShootDefaults.Services == nil && !gardencorev1alpha1helper.NetworksIntersect(out.Spec.Networks.Services, defaultServiceCIDR) {
			out.Spec.Networks.ShootDefaults.Services = &defaultServiceCIDR
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
			Subject: member,
			Role:    garden.ProjectMemberAdmin,
		})
	}

	for _, viewer := range in.Viewers {
		out.ProjectMembers = append(out.ProjectMembers, garden.ProjectMember{
			Subject: viewer,
			Role:    garden.ProjectMemberViewer,
		})
	}

	return nil
}

func Convert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in *garden.ProjectSpec, out *ProjectSpec, s conversion.Scope) error {
	if err := autoConvert_garden_ProjectSpec_To_v1beta1_ProjectSpec(in, out, s); err != nil {
		return err
	}

	for _, member := range in.ProjectMembers {
		if member.Role == garden.ProjectMemberAdmin {
			out.Members = append(out.Members, member.Subject)
		}
		if member.Role == garden.ProjectMemberViewer {
			out.Viewers = append(out.Viewers, member.Subject)
		}
	}

	return nil
}

func Convert_v1beta1_QuotaSpec_To_garden_QuotaSpec(in *QuotaSpec, out *garden.QuotaSpec, s conversion.Scope) error {
	if err := autoConvert_v1beta1_QuotaSpec_To_garden_QuotaSpec(in, out, s); err != nil {
		return err
	}

	switch in.Scope {
	case QuotaScopeProject:
		out.Scope = corev1.ObjectReference{
			APIVersion: "core.gardener.cloud/v1alpha1",
			Kind:       "Project",
		}
	case QuotaScopeSecret:
		out.Scope = corev1.ObjectReference{
			APIVersion: "v1",
			Kind:       "Secret",
		}
	}

	return nil
}

func Convert_garden_QuotaSpec_To_v1beta1_QuotaSpec(in *garden.QuotaSpec, out *QuotaSpec, s conversion.Scope) error {
	if err := autoConvert_garden_QuotaSpec_To_v1beta1_QuotaSpec(in, out, s); err != nil {
		return err
	}

	if in.Scope.APIVersion == "core.gardener.cloud/v1alpha1" && in.Scope.Kind == "Project" {
		out.Scope = QuotaScopeProject
	}
	if in.Scope.APIVersion == "v1" && in.Scope.Kind == "Secret" {
		out.Scope = QuotaScopeSecret
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

	switch {
	case in.Spec.AWS != nil:
		out.Spec.Type = "aws"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.AWS.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.AWS.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.AWS.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		for _, machineType := range in.Spec.AWS.Constraints.MachineTypes {
			var o garden.MachineType
			if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
				return err
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o)
		}

		for _, volumeType := range in.Spec.AWS.Constraints.VolumeTypes {
			var o garden.VolumeType
			if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
				return err
			}
			out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, o)
		}

		for _, zone := range in.Spec.AWS.Constraints.Zones {
			r := garden.Region{Name: zone.Region}
			for _, name := range zone.Names {
				r.Zones = append(r.Zones, garden.AvailabilityZone{
					Name: name,
				})
			}
			out.Spec.Regions = append(out.Spec.Regions, r)
		}

	case in.Spec.Azure != nil:
		out.Spec.Type = "azure"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.Azure.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.Azure.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.Azure.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		for _, machineType := range in.Spec.Azure.Constraints.MachineTypes {
			var o garden.MachineType
			if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
				return err
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o)
		}

		for _, volumeType := range in.Spec.Azure.Constraints.VolumeTypes {
			var o garden.VolumeType
			if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
				return err
			}
			out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, o)
		}

		if regionsJSON, ok := in.Annotations[garden.MigrationCloudProfileRegions]; ok {
			var regions []garden.Region
			if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
				return err
			}
			out.Spec.Regions = regions
		} else {
			out.Spec.Regions = nil
		}

		providerConfig := &garden.ProviderConfig{}
		if pc, ok := in.Annotations[garden.MigrationCloudProfileProviderConfig]; ok {
			if err := json.Unmarshal([]byte(pc), providerConfig); err != nil {
				return err
			}
		}
		cloudProfileConfig := &azurev1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		if providerConfig != nil {
			extensionsScheme := runtime.NewScheme()
			if err := azureinstall.AddToScheme(extensionsScheme); err != nil {
				return err
			}
			decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()
			switch {
			case providerConfig.Object != nil:
				var ok bool
				cloudProfileConfig = providerConfig.Object.(*azurev1alpha1.CloudProfileConfig)
				if !ok {
					klog.Errorf("Cannot cast providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
				}
			case providerConfig.Raw != nil:
				if _, _, err := decoder.Decode(providerConfig.Raw, nil, cloudProfileConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
				}
			}
		}
		if len(cloudProfileConfig.MachineImages) == 0 {
			cloudProfileConfig.MachineImages = []azurev1alpha1.MachineImages{}
		}
		cloudProfileConfig.CountFaultDomains = nil
		for _, c := range in.Spec.Azure.CountFaultDomains {
			if !azureV1alpha1DomainCountsHaveRegion(cloudProfileConfig.CountUpdateDomains, c.Region) {
				cloudProfileConfig.CountFaultDomains = append(cloudProfileConfig.CountFaultDomains, azurev1alpha1.DomainCount{
					Region: c.Region,
					Count:  c.Count,
				})
			}
		}
		cloudProfileConfig.CountUpdateDomains = nil
		for _, c := range in.Spec.Azure.CountUpdateDomains {
			if !azureV1alpha1DomainCountsHaveRegion(cloudProfileConfig.CountUpdateDomains, c.Region) {
				cloudProfileConfig.CountUpdateDomains = append(cloudProfileConfig.CountUpdateDomains, azurev1alpha1.DomainCount{
					Region: c.Region,
					Count:  c.Count,
				})
			}
		}

		data, err := json.Marshal(cloudProfileConfig)
		if err != nil {
			return err
		}
		out.Spec.ProviderConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

	case in.Spec.GCP != nil:
		out.Spec.Type = "gcp"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.GCP.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.GCP.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.GCP.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		for _, machineType := range in.Spec.GCP.Constraints.MachineTypes {
			var o garden.MachineType
			if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
				return err
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o)
		}

		for _, volumeType := range in.Spec.GCP.Constraints.VolumeTypes {
			var o garden.VolumeType
			if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
				return err
			}
			out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, o)
		}

		for _, zone := range in.Spec.GCP.Constraints.Zones {
			r := garden.Region{Name: zone.Region}
			for _, name := range zone.Names {
				r.Zones = append(r.Zones, garden.AvailabilityZone{
					Name: name,
				})
			}
			out.Spec.Regions = append(out.Spec.Regions, r)
		}

	case in.Spec.OpenStack != nil:
		out.Spec.Type = "openstack"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.OpenStack.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.OpenStack.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.OpenStack.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		if volumeTypesJSON, ok := in.Annotations[garden.MigrationCloudProfileVolumeTypes]; ok {
			var volumeTypes []garden.VolumeType
			if err := json.Unmarshal([]byte(volumeTypesJSON), &volumeTypes); err != nil {
				return err
			}
			out.Spec.VolumeTypes = volumeTypes
		} else {
			out.Spec.VolumeTypes = nil
		}

		for _, machineType := range in.Spec.OpenStack.Constraints.MachineTypes {
			var o garden.MachineType
			if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType.MachineType, &o, s); err != nil {
				return err
			}
			o.Storage = &garden.MachineTypeStorage{
				Size: machineType.VolumeSize,
				Type: machineType.VolumeType,
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o)

			if !volumeTypesHaveName(out.Spec.VolumeTypes, machineType.Name) {
				out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, garden.VolumeType{
					Name:   machineType.Name,
					Class:  machineType.VolumeType,
					Usable: machineType.Usable,
				})
			}
		}

		for _, zone := range in.Spec.OpenStack.Constraints.Zones {
			r := garden.Region{Name: zone.Region}
			for _, name := range zone.Names {
				r.Zones = append(r.Zones, garden.AvailabilityZone{
					Name: name,
				})
			}
			out.Spec.Regions = append(out.Spec.Regions, r)
		}

		providerConfig := &garden.ProviderConfig{}
		if pc, ok := in.Annotations[garden.MigrationCloudProfileProviderConfig]; ok {
			if err := json.Unmarshal([]byte(pc), providerConfig); err != nil {
				return err
			}
		}
		cloudProfileConfig := &openstackv1alpha1.CloudProfileConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "CloudProfileConfig",
			},
		}
		if providerConfig != nil {
			extensionsScheme := runtime.NewScheme()
			if err := openstackinstall.AddToScheme(extensionsScheme); err != nil {
				return err
			}
			decoder := serializer.NewCodecFactory(extensionsScheme).UniversalDecoder()
			switch {
			case providerConfig.Object != nil:
				var ok bool
				cloudProfileConfig = providerConfig.Object.(*openstackv1alpha1.CloudProfileConfig)
				if !ok {
					klog.Errorf("Cannot cast providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
				}
			case providerConfig.Raw != nil:
				if _, _, err := decoder.Decode(providerConfig.Raw, nil, cloudProfileConfig); err != nil {
					// If an error occurs then the provider config information contains invalid syntax, and in this
					// case we don't want to fail here. We rather don't try to migrate.
					klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
				}
			}
		}
		if len(cloudProfileConfig.MachineImages) == 0 {
			cloudProfileConfig.MachineImages = []openstackv1alpha1.MachineImages{}
		}
		cloudProfileConfig.Constraints.LoadBalancerProviders = nil
		for _, p := range in.Spec.OpenStack.Constraints.LoadBalancerProviders {
			if !openstackV1alpha1LoadBalancerProvidersHaveProvider(cloudProfileConfig.Constraints.LoadBalancerProviders, p.Name) {
				cloudProfileConfig.Constraints.LoadBalancerProviders = append(cloudProfileConfig.Constraints.LoadBalancerProviders, openstackv1alpha1.LoadBalancerProvider{
					Name: p.Name,
				})
			}
		}
		cloudProfileConfig.Constraints.FloatingPools = nil
		for _, p := range in.Spec.OpenStack.Constraints.FloatingPools {
			if !openstackV1alpha1FloatingPoolsHavePool(cloudProfileConfig.Constraints.FloatingPools, p.Name) {
				var loadBalancerClasses []openstackv1alpha1.LoadBalancerClass
				for _, c := range p.LoadBalancerClasses {
					loadBalancerClasses = append(loadBalancerClasses, openstackv1alpha1.LoadBalancerClass{
						Name:              c.Name,
						FloatingSubnetID:  c.FloatingSubnetID,
						FloatingNetworkID: c.FloatingNetworkID,
						SubnetID:          c.SubnetID,
					})
				}
				cloudProfileConfig.Constraints.FloatingPools = append(cloudProfileConfig.Constraints.FloatingPools, openstackv1alpha1.FloatingPool{
					Name:                p.Name,
					LoadBalancerClasses: loadBalancerClasses,
				})
			}
		}
		cloudProfileConfig.DNSServers = in.Spec.OpenStack.DNSServers
		cloudProfileConfig.DHCPDomain = in.Spec.OpenStack.DHCPDomain
		cloudProfileConfig.KeyStoneURL = in.Spec.OpenStack.KeyStoneURL
		cloudProfileConfig.RequestTimeout = in.Spec.OpenStack.RequestTimeout

		data, err := json.Marshal(cloudProfileConfig)
		if err != nil {
			return err
		}
		out.Spec.ProviderConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

	case in.Spec.Alicloud != nil:
		out.Spec.Type = "alicloud"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.Alicloud.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.Alicloud.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.Alicloud.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		availableMachineTypesPerZone := map[string][]string{}
		for _, machineType := range in.Spec.Alicloud.Constraints.MachineTypes {
			var o garden.AlicloudMachineType
			if err := autoConvert_v1beta1_AlicloudMachineType_To_garden_AlicloudMachineType(&machineType, &o, s); err != nil {
				return err
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o.MachineType)
			for _, zone := range machineType.Zones {
				availableMachineTypesPerZone[zone] = append(availableMachineTypesPerZone[zone], machineType.Name)
			}
		}

		availableVolumeTypesPerZone := map[string][]string{}
		for _, volumeType := range in.Spec.Alicloud.Constraints.VolumeTypes {
			var o garden.AlicloudVolumeType
			if err := autoConvert_v1beta1_AlicloudVolumeType_To_garden_AlicloudVolumeType(&volumeType, &o, s); err != nil {
				return err
			}
			out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, o.VolumeType)
			for _, zone := range volumeType.Zones {
				availableVolumeTypesPerZone[zone] = append(availableVolumeTypesPerZone[zone], volumeType.Name)
			}
		}

		for _, zone := range in.Spec.Alicloud.Constraints.Zones {
			r := garden.Region{Name: zone.Region}
			for _, name := range zone.Names {
				var unavailableMachineTypes []string
				for _, machineType := range in.Spec.Alicloud.Constraints.MachineTypes {
					if !zoneHasAlicloudType(availableMachineTypesPerZone, name, machineType.Name) {
						unavailableMachineTypes = append(unavailableMachineTypes, machineType.Name)
					}
				}
				var unavailableVolumeTypes []string
				for _, volumeType := range in.Spec.Alicloud.Constraints.VolumeTypes {
					if !zoneHasAlicloudType(availableVolumeTypesPerZone, name, volumeType.Name) {
						unavailableVolumeTypes = append(unavailableVolumeTypes, volumeType.Name)
					}
				}
				r.Zones = append(r.Zones, garden.AvailabilityZone{
					Name:                    name,
					UnavailableMachineTypes: unavailableMachineTypes,
					UnavailableVolumeTypes:  unavailableVolumeTypes,
				})
			}
			out.Spec.Regions = append(out.Spec.Regions, r)
		}

	case in.Spec.Packet != nil:
		out.Spec.Type = "packet"

		versions := map[string]struct{}{}
		for _, version := range in.Spec.Packet.Constraints.Kubernetes.OfferedVersions {
			versions[version.Version] = struct{}{}
			out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
				Version:        version.Version,
				ExpirationDate: version.ExpirationDate,
			})
		}
		for _, version := range in.Spec.Packet.Constraints.Kubernetes.Versions {
			if _, ok := versions[version]; !ok {
				out.Spec.Kubernetes.Versions = append(out.Spec.Kubernetes.Versions, garden.ExpirableVersion{
					Version: version,
				})
			}
		}

		for _, image := range in.Spec.Packet.Constraints.MachineImages {
			i := garden.CloudProfileMachineImage{Name: image.Name}
			if len(image.Version) > 0 {
				i.Versions = append(i.Versions, garden.ExpirableVersion{
					Version: image.Version,
				})
			}
			for _, version := range image.Versions {
				if version.Version != image.Version {
					i.Versions = append(i.Versions, garden.ExpirableVersion{
						Version:        version.Version,
						ExpirationDate: version.ExpirationDate,
					})
				}
			}
			out.Spec.MachineImages = append(out.Spec.MachineImages, i)
		}

		for _, machineType := range in.Spec.Packet.Constraints.MachineTypes {
			var o garden.MachineType
			if err := autoConvert_v1beta1_MachineType_To_garden_MachineType(&machineType, &o, s); err != nil {
				return err
			}
			out.Spec.MachineTypes = append(out.Spec.MachineTypes, o)
		}

		for _, volumeType := range in.Spec.Packet.Constraints.VolumeTypes {
			var o garden.VolumeType
			if err := autoConvert_v1beta1_VolumeType_To_garden_VolumeType(&volumeType, &o, s); err != nil {
				return err
			}
			out.Spec.VolumeTypes = append(out.Spec.VolumeTypes, o)
		}

		for _, zone := range in.Spec.Packet.Constraints.Zones {
			r := garden.Region{Name: zone.Region}
			for _, name := range zone.Names {
				r.Zones = append(r.Zones, garden.AvailabilityZone{
					Name: name,
				})
			}
			out.Spec.Regions = append(out.Spec.Regions, r)
		}

	default:
		if providerType, ok := in.Annotations[garden.MigrationCloudProfileType]; ok {
			out.Spec.Type = providerType
		} else {
			out.Spec.Type = ""
		}

		if kubernetesJSON, ok := in.Annotations[garden.MigrationCloudProfileKubernetes]; ok {
			var kubernetes garden.KubernetesSettings
			if err := json.Unmarshal([]byte(kubernetesJSON), &kubernetes); err != nil {
				return err
			}
			out.Spec.Kubernetes = kubernetes
		} else {
			out.Spec.Kubernetes = garden.KubernetesSettings{}
		}

		if machineTypesJSON, ok := in.Annotations[garden.MigrationCloudProfileMachineTypes]; ok {
			var machineTypes []garden.MachineType
			if err := json.Unmarshal([]byte(machineTypesJSON), &machineTypes); err != nil {
				return err
			}
			out.Spec.MachineTypes = machineTypes
		} else {
			out.Spec.MachineTypes = nil
		}

		if machineImagesJSON, ok := in.Annotations[garden.MigrationCloudProfileMachineImages]; ok {
			var machineImages []garden.CloudProfileMachineImage
			if err := json.Unmarshal([]byte(machineImagesJSON), &machineImages); err != nil {
				return err
			}
			out.Spec.MachineImages = machineImages
		} else {
			out.Spec.MachineImages = nil
		}

		if regionsJSON, ok := in.Annotations[garden.MigrationCloudProfileRegions]; ok {
			var regions []garden.Region
			if err := json.Unmarshal([]byte(regionsJSON), &regions); err != nil {
				return err
			}
			out.Spec.Regions = regions
		} else {
			out.Spec.Regions = nil
		}

		if volumeTypesJSON, ok := in.Annotations[garden.MigrationCloudProfileVolumeTypes]; ok {
			var volumeTypes []garden.VolumeType
			if err := json.Unmarshal([]byte(volumeTypesJSON), &volumeTypes); err != nil {
				return err
			}
			out.Spec.VolumeTypes = volumeTypes
		} else {
			out.Spec.VolumeTypes = nil
		}
	}

	if out.Spec.Regions == nil {
		out.Spec.Regions = []garden.Region{}
	}

	if seedSelectorJSON, ok := in.Annotations[garden.MigrationCloudProfileSeedSelector]; ok {
		var seedSelector metav1.LabelSelector
		if err := json.Unmarshal([]byte(seedSelectorJSON), &seedSelector); err != nil {
			return err
		}
		out.Spec.SeedSelector = &seedSelector
	} else {
		out.Spec.SeedSelector = nil
	}

	if providerConfigJSON, ok := in.Annotations[garden.MigrationCloudProfileProviderConfig]; ok {
		var providerConfig garden.ProviderConfig
		if err := json.Unmarshal([]byte(providerConfigJSON), &providerConfig); err != nil {
			return err
		}
		out.Spec.ProviderConfig = &providerConfig
	}

	return nil
}

func Convert_garden_CloudProfile_To_v1beta1_CloudProfile(in *garden.CloudProfile, out *CloudProfile, s conversion.Scope) error {
	if err := autoConvert_garden_CloudProfile_To_v1beta1_CloudProfile(in, out, s); err != nil {
		return err
	}

	switch in.Spec.Type {
	case "aws":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.AWS == nil {
				out.Spec.AWS = &AWSProfile{}
			}
			out.Spec.AWS.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

	case "azure":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.Azure == nil {
				out.Spec.Azure = &AzureProfile{}
			}
			out.Spec.Azure.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}
		if len(in.Spec.Regions) > 0 {
			data, err := json.Marshal(in.Spec.Regions)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileRegions] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileRegions)
		}

	case "gcp":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.GCP == nil {
				out.Spec.GCP = &GCPProfile{}
			}
			out.Spec.GCP.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

	case "openstack":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.OpenStack == nil {
				out.Spec.OpenStack = &OpenStackProfile{}
			}
			out.Spec.OpenStack.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}
		if len(in.Spec.VolumeTypes) > 0 {
			data, err := json.Marshal(in.Spec.VolumeTypes)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileVolumeTypes] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileVolumeTypes)
		}

	case "alicloud":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.Alicloud == nil {
				out.Spec.Alicloud = &AlicloudProfile{}
			}
			out.Spec.Alicloud.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

	case "packet":
		if dnsProviders, ok := in.Annotations[garden.MigrationCloudProfileDNSProviders]; ok {
			if out.Spec.Packet == nil {
				out.Spec.Packet = &PacketProfile{}
			}
			out.Spec.Packet.Constraints.DNSProviders = stringSliceToDNSProviderConstraint(strings.Split(dnsProviders, ","))
		}

	default:
		out.Annotations[garden.MigrationCloudProfileType] = in.Spec.Type

		data, err := json.Marshal(in.Spec.Kubernetes)
		if err != nil {
			return err
		}
		out.Annotations[garden.MigrationCloudProfileKubernetes] = string(data)

		if len(in.Spec.MachineImages) > 0 {
			data, err := json.Marshal(in.Spec.MachineImages)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileMachineImages] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileMachineImages)
		}

		if len(in.Spec.MachineTypes) > 0 {
			data, err := json.Marshal(in.Spec.MachineTypes)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileMachineTypes] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileMachineTypes)
		}

		if len(in.Spec.Regions) > 0 {
			data, err := json.Marshal(in.Spec.Regions)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileRegions] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileRegions)
		}

		if len(in.Spec.VolumeTypes) > 0 {
			data, err := json.Marshal(in.Spec.VolumeTypes)
			if err != nil {
				return err
			}
			out.Annotations[garden.MigrationCloudProfileVolumeTypes] = string(data)
		} else {
			delete(out.Annotations, garden.MigrationCloudProfileVolumeTypes)
		}
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

func Convert_garden_CloudProfileSpec_To_v1beta1_CloudProfileSpec(in *garden.CloudProfileSpec, out *CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_garden_CloudProfileSpec_To_v1beta1_CloudProfileSpec(in, out, s)
}

func Convert_v1beta1_CloudProfileSpec_To_garden_CloudProfileSpec(in *CloudProfileSpec, out *garden.CloudProfileSpec, s conversion.Scope) error {
	return autoConvert_v1beta1_CloudProfileSpec_To_garden_CloudProfileSpec(in, out, s)
}

func stringSliceToDNSProviderConstraint(slice []string) []DNSProviderConstraint {
	dnsConstraints := make([]DNSProviderConstraint, 0, len(slice))
	for _, s := range slice {
		dnsConstraints = append(dnsConstraints, DNSProviderConstraint{s})
	}
	return dnsConstraints
}

func zoneHasAlicloudType(typesPerZone map[string][]string, name, typeName string) bool {
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

func azureV1alpha1DomainCountsHaveRegion(domainCount []azurev1alpha1.DomainCount, regionName string) bool {
	for _, d := range domainCount {
		if d.Region == regionName {
			return true
		}
	}
	return false
}

func openstackV1alpha1LoadBalancerProvidersHaveProvider(providers []openstackv1alpha1.LoadBalancerProvider, providerName string) bool {
	for _, p := range providers {
		if p.Name == providerName {
			return true
		}
	}
	return false
}

func openstackV1alpha1FloatingPoolsHavePool(pools []openstackv1alpha1.FloatingPool, poolName string) bool {
	for _, p := range pools {
		if p.Name == poolName {
			return true
		}
	}
	return false
}

func volumeTypesHaveName(volumeTypes []garden.VolumeType, name string) bool {
	for _, v := range volumeTypes {
		if v.Name == name {
			return true
		}
	}
	return false
}

func Convert_v1beta1_Shoot_To_garden_Shoot(in *Shoot, out *garden.Shoot, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Shoot_To_garden_Shoot(in, out, s); err != nil {
		return err
	}

	var networking garden.Networking
	if in.Spec.Networking != nil {
		if err := Convert_v1beta1_Networking_To_garden_Networking(in.Spec.Networking, &networking, s); err != nil {
			return err
		}
	}
	out.Spec.Networking = networking

	var dns garden.DNS
	if err := autoConvert_v1beta1_DNS_To_garden_DNS(&in.Spec.DNS, &dns, s); err != nil {
		return err
	}

	var provider garden.DNSProvider
	if in.Spec.DNS.ExcludeDomains != nil || in.Spec.DNS.ExcludeZones != nil || in.Spec.DNS.IncludeDomains != nil || in.Spec.DNS.IncludeZones != nil || in.Spec.DNS.Provider != nil || in.Spec.DNS.SecretName != nil {
		provider.SecretName = in.Spec.DNS.SecretName
		provider.Type = in.Spec.DNS.Provider

		var domains *garden.DNSIncludeExclude
		if in.Spec.DNS.IncludeDomains != nil || in.Spec.DNS.ExcludeDomains != nil {
			domains = &garden.DNSIncludeExclude{}
			for _, val := range in.Spec.DNS.IncludeDomains {
				domains.Include = append(domains.Include, val)
			}
			for _, val := range in.Spec.DNS.ExcludeDomains {
				domains.Exclude = append(domains.Exclude, val)
			}
		}
		provider.Domains = domains

		var zones *garden.DNSIncludeExclude
		if in.Spec.DNS.IncludeZones != nil || in.Spec.DNS.ExcludeZones != nil {
			zones = &garden.DNSIncludeExclude{}
			for _, val := range in.Spec.DNS.IncludeZones {
				zones.Include = append(zones.Include, val)
			}
			for _, val := range in.Spec.DNS.ExcludeZones {
				zones.Exclude = append(zones.Exclude, val)
			}
		}
		provider.Zones = zones

		dns.Providers = append(dns.Providers, provider)
	}

	if additionalProviders, ok := in.Annotations[garden.MigrationShootDNSProviders]; ok {
		var providers []garden.DNSProvider
		if err := json.Unmarshal([]byte(additionalProviders), &providers); err != nil {
			return err
		}
		dns.Providers = append(dns.Providers, providers...)
	}
	out.Spec.DNS = &dns

	var workerMigrationInfo garden.WorkerMigrationInfo
	if data, ok := in.Annotations[garden.MigrationShootWorkers]; ok {
		if err := json.Unmarshal([]byte(data), &workerMigrationInfo); err != nil {
			return err
		}
	}

	switch {
	case in.Spec.Cloud.AWS != nil:
		out.Spec.Provider.Type = "aws"

		infrastructureConfig := &awsv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		if len(in.Spec.Cloud.AWS.Networks.Internal) != len(in.Spec.Cloud.AWS.Zones) {
			return fmt.Errorf("aws internal networks must have same number of entries like zones")
		}
		if len(in.Spec.Cloud.AWS.Networks.Public) != len(in.Spec.Cloud.AWS.Zones) {
			return fmt.Errorf("aws public networks must have same number of entries like zones")
		}
		if len(in.Spec.Cloud.AWS.Networks.Workers) != len(in.Spec.Cloud.AWS.Zones) {
			return fmt.Errorf("aws workers networks must have same number of entries like zones")
		}

		zones := make([]awsv1alpha1.Zone, 0, len(in.Spec.Cloud.AWS.Zones))
		for i, zone := range in.Spec.Cloud.AWS.Zones {
			zones = append(zones, awsv1alpha1.Zone{
				Name:     zone,
				Internal: in.Spec.Cloud.AWS.Networks.Internal[i],
				Public:   in.Spec.Cloud.AWS.Networks.Public[i],
				Workers:  in.Spec.Cloud.AWS.Networks.Workers[i],
			})
		}

		var vpcCIDR *string
		if c := in.Spec.Cloud.AWS.Networks.VPC.CIDR; c != nil {
			cidr := *c
			vpcCIDR = &cidr
		}

		infrastructureConfig.Networks = awsv1alpha1.Networks{
			VPC: awsv1alpha1.VPC{
				ID:   in.Spec.Cloud.AWS.Networks.VPC.ID,
				CIDR: vpcCIDR,
			},
			Zones: zones,
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &awsv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: awsv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		var cloudControllerManager *awsv1alpha1.CloudControllerManagerConfig
		if in.Spec.Kubernetes.CloudControllerManager != nil {
			cloudControllerManager = &awsv1alpha1.CloudControllerManagerConfig{
				FeatureGates: in.Spec.Kubernetes.CloudControllerManager.FeatureGates,
			}
		}
		controlPlaneConfig.CloudControllerManager = cloudControllerManager

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.AWS.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
				Volume: &garden.Volume{
					Size: worker.VolumeSize,
					Type: worker.VolumeType,
				},
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			if w.Zones == nil {
				w.Zones = in.Spec.Cloud.AWS.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.AWS.Workers = workers

		if in.Spec.Cloud.AWS.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.AWS.Networks.Nodes
		}
		if in.Spec.Cloud.AWS.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.AWS.Networks.Pods
		}
		if in.Spec.Cloud.AWS.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.AWS.Networks.Services
		}

	case in.Spec.Cloud.Azure != nil:
		out.Spec.Provider.Type = "azure"

		infrastructureConfig := &azurev1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		var resourceGroup *azurev1alpha1.ResourceGroup
		if in.Spec.Cloud.Azure.ResourceGroup != nil {
			resourceGroup = &azurev1alpha1.ResourceGroup{
				Name: in.Spec.Cloud.Azure.ResourceGroup.Name,
			}
		}

		var vnet azurev1alpha1.VNet
		if in.Spec.Cloud.Azure.Networks.VNet.CIDR != nil {
			cidr := string(*in.Spec.Cloud.Azure.Networks.VNet.CIDR)
			vnet.CIDR = &cidr
		}
		if in.Spec.Cloud.Azure.Networks.VNet.Name != nil {
			vnet.Name = in.Spec.Cloud.Azure.Networks.VNet.Name
		}

		infrastructureConfig.ResourceGroup = resourceGroup
		infrastructureConfig.Networks = azurev1alpha1.NetworkConfig{
			VNet:    vnet,
			Workers: in.Spec.Cloud.Azure.Networks.Workers,
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &azurev1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: azurev1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		var cloudControllerManager *azurev1alpha1.CloudControllerManagerConfig
		if in.Spec.Kubernetes.CloudControllerManager != nil {
			cloudControllerManager = &azurev1alpha1.CloudControllerManagerConfig{
				FeatureGates: in.Spec.Kubernetes.CloudControllerManager.FeatureGates,
			}
		}
		controlPlaneConfig.CloudControllerManager = cloudControllerManager

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.Azure.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
				Volume: &garden.Volume{
					Size: worker.VolumeSize,
					Type: worker.VolumeType,
				},
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.Azure.Workers = workers

		if in.Spec.Cloud.Azure.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.Azure.Networks.Nodes
		}
		if in.Spec.Cloud.Azure.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.Azure.Networks.Pods
		}
		if in.Spec.Cloud.Azure.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.Azure.Networks.Services
		}

	case in.Spec.Cloud.GCP != nil:
		out.Spec.Provider.Type = "gcp"

		infrastructureConfig := &gcpv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		if len(in.Spec.Cloud.GCP.Networks.Workers) != 1 {
			return fmt.Errorf("gcp worker networks must only have exactly one entry")
		}
		if len(in.Spec.Cloud.GCP.Zones) == 0 {
			return fmt.Errorf("gcp zones must have at least one entry")
		}

		var vpc *gcpv1alpha1.VPC
		if in.Spec.Cloud.GCP.Networks.VPC != nil {
			vpc = &gcpv1alpha1.VPC{
				Name: in.Spec.Cloud.GCP.Networks.VPC.Name,
			}
		}

		var internalCIDR *string
		if c := in.Spec.Cloud.GCP.Networks.Internal; c != nil {
			cidr := *c
			internalCIDR = &cidr
		}

		infrastructureConfig.Networks = gcpv1alpha1.NetworkConfig{
			VPC:      vpc,
			Worker:   in.Spec.Cloud.GCP.Networks.Workers[0],
			Internal: internalCIDR,
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &gcpv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: gcpv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		var cloudControllerManager *gcpv1alpha1.CloudControllerManagerConfig
		if in.Spec.Kubernetes.CloudControllerManager != nil {
			cloudControllerManager = &gcpv1alpha1.CloudControllerManagerConfig{
				FeatureGates: in.Spec.Kubernetes.CloudControllerManager.FeatureGates,
			}
		}
		controlPlaneConfig.CloudControllerManager = cloudControllerManager
		controlPlaneConfig.Zone = in.Spec.Cloud.GCP.Zones[0]

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.GCP.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
				Volume: &garden.Volume{
					Size: worker.VolumeSize,
					Type: worker.VolumeType,
				},
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			if w.Zones == nil {
				w.Zones = in.Spec.Cloud.GCP.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.GCP.Workers = workers

		if in.Spec.Cloud.GCP.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.GCP.Networks.Nodes
		}
		if in.Spec.Cloud.GCP.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.GCP.Networks.Pods
		}
		if in.Spec.Cloud.GCP.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.GCP.Networks.Services
		}

	case in.Spec.Cloud.OpenStack != nil:
		out.Spec.Provider.Type = "openstack"

		infrastructureConfig := &openstackv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		if len(in.Spec.Cloud.OpenStack.Networks.Workers) != 1 {
			return fmt.Errorf("openstack worker networks must only have exactly one entry")
		}

		var router *openstackv1alpha1.Router
		if in.Spec.Cloud.OpenStack.Networks.Router != nil {
			router = &openstackv1alpha1.Router{
				ID: in.Spec.Cloud.OpenStack.Networks.Router.ID,
			}
		}

		infrastructureConfig.FloatingPoolName = in.Spec.Cloud.OpenStack.FloatingPoolName
		infrastructureConfig.Networks = openstackv1alpha1.Networks{
			Router: router,
			Worker: in.Spec.Cloud.OpenStack.Networks.Workers[0],
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &openstackv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: openstackv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		var cloudControllerManager *openstackv1alpha1.CloudControllerManagerConfig
		if in.Spec.Kubernetes.CloudControllerManager != nil {
			cloudControllerManager = &openstackv1alpha1.CloudControllerManagerConfig{
				FeatureGates: in.Spec.Kubernetes.CloudControllerManager.FeatureGates,
			}
		}

		var loadBalancerClasses = make([]openstackv1alpha1.LoadBalancerClass, 0, len(in.Spec.Cloud.OpenStack.LoadBalancerClasses))
		for _, loadBalancerClass := range in.Spec.Cloud.OpenStack.LoadBalancerClasses {
			loadBalancerClasses = append(loadBalancerClasses, openstackv1alpha1.LoadBalancerClass{
				Name:              loadBalancerClass.Name,
				FloatingSubnetID:  loadBalancerClass.FloatingSubnetID,
				FloatingNetworkID: loadBalancerClass.FloatingNetworkID,
				SubnetID:          loadBalancerClass.SubnetID,
			})
		}

		controlPlaneConfig.CloudControllerManager = cloudControllerManager
		controlPlaneConfig.LoadBalancerProvider = in.Spec.Cloud.OpenStack.LoadBalancerProvider
		controlPlaneConfig.LoadBalancerClasses = loadBalancerClasses
		controlPlaneConfig.Zone = in.Spec.Cloud.OpenStack.Zones[0]

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.OpenStack.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			if w.Zones == nil {
				w.Zones = in.Spec.Cloud.OpenStack.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.OpenStack.Workers = workers

		if in.Spec.Cloud.OpenStack.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.OpenStack.Networks.Nodes
		}
		if in.Spec.Cloud.OpenStack.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.OpenStack.Networks.Pods
		}
		if in.Spec.Cloud.OpenStack.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.OpenStack.Networks.Services
		}

	case in.Spec.Cloud.Alicloud != nil:
		out.Spec.Provider.Type = "alicloud"

		infrastructureConfig := &alicloudv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		if len(in.Spec.Cloud.Alicloud.Networks.Workers) != len(in.Spec.Cloud.Alicloud.Zones) {
			return fmt.Errorf("alicloud workers networks must have same number of entries like zones")
		}
		if len(in.Spec.Cloud.Alicloud.Zones) == 0 {
			return fmt.Errorf("alicloud zones must have at least one entry")
		}

		zones := make([]alicloudv1alpha1.Zone, 0, len(in.Spec.Cloud.Alicloud.Zones))
		for i, zone := range in.Spec.Cloud.Alicloud.Zones {
			zones = append(zones, alicloudv1alpha1.Zone{
				Name:   zone,
				Worker: in.Spec.Cloud.Alicloud.Networks.Workers[i],
			})
		}

		var vpcCIDR *string
		if c := in.Spec.Cloud.Alicloud.Networks.VPC.CIDR; c != nil {
			cidr := *c
			vpcCIDR = &cidr
		}

		infrastructureConfig.Networks = alicloudv1alpha1.Networks{
			VPC: alicloudv1alpha1.VPC{
				ID:   in.Spec.Cloud.Alicloud.Networks.VPC.ID,
				CIDR: vpcCIDR,
			},
			Zones: zones,
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &alicloudv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: alicloudv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		var cloudControllerManager *alicloudv1alpha1.CloudControllerManagerConfig
		if in.Spec.Kubernetes.CloudControllerManager != nil {
			cloudControllerManager = &alicloudv1alpha1.CloudControllerManagerConfig{
				FeatureGates: in.Spec.Kubernetes.CloudControllerManager.FeatureGates,
			}
		}
		controlPlaneConfig.CloudControllerManager = cloudControllerManager
		controlPlaneConfig.Zone = in.Spec.Cloud.Alicloud.Zones[0]

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.Alicloud.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
				Volume: &garden.Volume{
					Size: worker.VolumeSize,
					Type: worker.VolumeType,
				},
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			if w.Zones == nil {
				w.Zones = in.Spec.Cloud.Alicloud.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.Alicloud.Workers = workers

		if in.Spec.Cloud.Alicloud.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.Alicloud.Networks.Nodes
		}
		if in.Spec.Cloud.Alicloud.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.Alicloud.Networks.Pods
		}
		if in.Spec.Cloud.Alicloud.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.Alicloud.Networks.Services
		}

	case in.Spec.Cloud.Packet != nil:
		out.Spec.Provider.Type = "packet"

		infrastructureConfig := &packetv1alpha1.InfrastructureConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
				Kind:       "InfrastructureConfig",
			},
		}

		data, err := json.Marshal(infrastructureConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.InfrastructureConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		controlPlaneConfig := &packetv1alpha1.ControlPlaneConfig{
			TypeMeta: metav1.TypeMeta{
				APIVersion: packetv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ControlPlaneConfig",
			},
		}

		data, err = json.Marshal(controlPlaneConfig)
		if err != nil {
			return err
		}
		out.Spec.Provider.ControlPlaneConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Raw: data,
			},
		}

		var workers []garden.Worker
		out.Spec.Provider.Workers = nil

		for _, worker := range in.Spec.Cloud.Packet.Workers {
			w := garden.Worker{
				Annotations: worker.Annotations,
				CABundle:    worker.CABundle,
				Labels:      worker.Labels,
				Name:        worker.Name,
				Machine: garden.Machine{
					Type: worker.MachineType,
				},
				Maximum:        worker.AutoScalerMax,
				Minimum:        worker.AutoScalerMin,
				MaxSurge:       worker.MaxSurge,
				MaxUnavailable: worker.MaxUnavailable,
				Taints:         worker.Taints,
				Volume: &garden.Volume{
					Size: worker.VolumeSize,
					Type: worker.VolumeType,
				},
			}

			var machineImage *garden.ShootMachineImage
			if worker.MachineImage != nil {
				machineImage = &garden.ShootMachineImage{}
				if err := autoConvert_v1beta1_ShootMachineImage_To_garden_ShootMachineImage(worker.MachineImage, machineImage, s); err != nil {
					return err
				}
			}
			w.Machine.Image = machineImage

			if worker.Kubelet != nil {
				kubeletConfig := &garden.KubeletConfig{}
				if err := autoConvert_v1beta1_KubeletConfig_To_garden_KubeletConfig(worker.Kubelet, kubeletConfig, s); err != nil {
					return err
				}
				w.Kubernetes = &garden.WorkerKubernetes{Kubelet: kubeletConfig}
			}

			if data, ok := workerMigrationInfo[worker.Name]; ok {
				w.ProviderConfig = data.ProviderConfig
				w.Zones = data.Zones
			}

			if w.Zones == nil {
				w.Zones = in.Spec.Cloud.Packet.Zones
			}

			out.Spec.Provider.Workers = append(out.Spec.Provider.Workers, w)
			workers = append(workers, w)
		}
		out.Spec.Cloud.Packet.Workers = workers

		if in.Spec.Cloud.Packet.Networks.Nodes != nil {
			out.Spec.Networking.Nodes = *in.Spec.Cloud.Packet.Networks.Nodes
		}
		if in.Spec.Cloud.Packet.Networks.Pods != nil {
			out.Spec.Networking.Pods = in.Spec.Cloud.Packet.Networks.Pods
		}
		if in.Spec.Cloud.Packet.Networks.Services != nil {
			out.Spec.Networking.Services = in.Spec.Cloud.Packet.Networks.Services
		}

	default:
		if data, ok := in.Annotations[garden.MigrationShootProvider]; ok {
			var provider garden.Provider
			if err := json.Unmarshal([]byte(data), &provider); err != nil {
				return err
			}
			out.Spec.Provider = provider
		}
	}

	out.Spec.CloudProfileName = in.Spec.Cloud.Profile
	out.Spec.Region = in.Spec.Cloud.Region
	out.Spec.SecretBindingName = in.Spec.Cloud.SecretBindingRef.Name
	out.Spec.SeedName = in.Spec.Cloud.Seed

	return nil
}

func Convert_garden_Shoot_To_v1beta1_Shoot(in *garden.Shoot, out *Shoot, s conversion.Scope) error {
	if err := autoConvert_garden_Shoot_To_v1beta1_Shoot(in, out, s); err != nil {
		return err
	}

	networking := &Networking{}
	if err := Convert_garden_Networking_To_v1beta1_Networking(&in.Spec.Networking, networking, s); err != nil {
		return err
	}
	out.Spec.Networking = networking

	var dns DNS
	if in.Spec.DNS != nil {
		if err := autoConvert_garden_DNS_To_v1beta1_DNS(in.Spec.DNS, &dns, s); err != nil {
			return err
		}

		if len(in.Spec.DNS.Providers) > 0 {
			dns.Provider = in.Spec.DNS.Providers[0].Type
			dns.SecretName = in.Spec.DNS.Providers[0].SecretName

			if in.Spec.DNS.Providers[0].Domains != nil {
				dns.IncludeDomains = in.Spec.DNS.Providers[0].Domains.Include
				dns.ExcludeDomains = in.Spec.DNS.Providers[0].Domains.Exclude
			}

			if in.Spec.DNS.Providers[0].Zones != nil {
				dns.IncludeZones = in.Spec.DNS.Providers[0].Zones.Include
				dns.ExcludeZones = in.Spec.DNS.Providers[0].Zones.Exclude
			}
		}

		if len(in.Spec.DNS.Providers) > 1 {
			data, err := json.Marshal(in.Spec.DNS.Providers[1:])
			if err != nil {
				return err
			}
			metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootDNSProviders, string(data))
		} else {
			delete(out.Annotations, garden.MigrationShootDNSProviders)
		}
	}
	out.Spec.DNS = dns

	out.Spec.Cloud.Profile = in.Spec.CloudProfileName
	out.Spec.Cloud.Region = in.Spec.Region
	out.Spec.Cloud.SecretBindingRef.Name = in.Spec.SecretBindingName
	out.Spec.Cloud.Seed = in.Spec.SeedName

	if in.Spec.Cloud.AWS != nil || in.Spec.Cloud.Azure != nil || in.Spec.Cloud.GCP != nil || in.Spec.Cloud.OpenStack != nil || in.Spec.Cloud.Alicloud != nil || in.Spec.Cloud.Packet != nil {
		workerMigrationInfo := make(garden.WorkerMigrationInfo, len(in.Spec.Provider.Workers))
		for _, worker := range in.Spec.Provider.Workers {
			workerMigrationInfo[worker.Name] = garden.WorkerMigrationData{
				ProviderConfig: worker.ProviderConfig,
				Zones:          worker.Zones,
			}
		}
		data, err := json.Marshal(workerMigrationInfo)
		if err != nil {
			return err
		}
		metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootWorkers, string(data))
	} else {
		data, err := json.Marshal(in.Spec.Provider)
		if err != nil {
			return err
		}
		metav1.SetMetaDataAnnotation(&out.ObjectMeta, garden.MigrationShootProvider, string(data))
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

	if in.Seed != nil {
		out.Seed = *in.Seed
	} else {
		out.Seed = ""
	}

	return nil
}

func Convert_v1beta1_ShootStatus_To_garden_ShootStatus(in *ShootStatus, out *garden.ShootStatus, s conversion.Scope) error {
	if err := autoConvert_v1beta1_ShootStatus_To_garden_ShootStatus(in, out, s); err != nil {
		return err
	}

	if len(in.Seed) > 0 {
		out.Seed = &in.Seed
	} else {
		out.Seed = nil
	}

	return nil
}

func Convert_garden_DNS_To_v1beta1_DNS(in *garden.DNS, out *DNS, s conversion.Scope) error {
	return autoConvert_garden_DNS_To_v1beta1_DNS(in, out, s)
}

func Convert_v1beta1_DNS_To_garden_DNS(in *DNS, out *garden.DNS, s conversion.Scope) error {
	return autoConvert_v1beta1_DNS_To_garden_DNS(in, out, s)
}

func Convert_garden_Worker_To_v1beta1_Worker(in *garden.Worker, out *Worker, s conversion.Scope) error {
	return autoConvert_garden_Worker_To_v1beta1_Worker(in, out, s)
}

func Convert_v1beta1_Worker_To_garden_Worker(in *Worker, out *garden.Worker, s conversion.Scope) error {
	return autoConvert_v1beta1_Worker_To_garden_Worker(in, out, s)
}

func Convert_garden_Worker_To_v1beta1_AWSWorker(in *garden.Worker, out *AWSWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	if in.Volume != nil {
		out.VolumeSize = in.Volume.Size
		out.VolumeType = in.Volume.Type
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_AWSWorker_To_garden_Worker(in *AWSWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Worker_To_v1beta1_AzureWorker(in *garden.Worker, out *AzureWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	if in.Volume != nil {
		out.VolumeSize = in.Volume.Size
		out.VolumeType = in.Volume.Type
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_AzureWorker_To_garden_Worker(in *AzureWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Worker_To_v1beta1_GCPWorker(in *garden.Worker, out *GCPWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	if in.Volume != nil {
		out.VolumeSize = in.Volume.Size
		out.VolumeType = in.Volume.Type
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_GCPWorker_To_garden_Worker(in *GCPWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Worker_To_v1beta1_OpenStackWorker(in *garden.Worker, out *OpenStackWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_OpenStackWorker_To_garden_Worker(in *OpenStackWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Worker_To_v1beta1_AlicloudWorker(in *garden.Worker, out *AlicloudWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	if in.Volume != nil {
		out.VolumeSize = in.Volume.Size
		out.VolumeType = in.Volume.Type
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_AlicloudWorker_To_garden_Worker(in *AlicloudWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Worker_To_v1beta1_PacketWorker(in *garden.Worker, out *PacketWorker, s conversion.Scope) error {
	out.Name = in.Name
	out.MachineType = in.Machine.Type
	out.AutoScalerMin = in.Minimum
	out.AutoScalerMax = in.Maximum
	out.MaxSurge = in.MaxSurge
	out.MaxUnavailable = in.MaxUnavailable
	out.Annotations = in.Annotations
	out.Labels = in.Labels
	out.Taints = in.Taints
	out.CABundle = in.CABundle

	var machineImage *ShootMachineImage
	if in.Machine.Image != nil {
		machineImage = &ShootMachineImage{}
		if err := autoConvert_garden_ShootMachineImage_To_v1beta1_ShootMachineImage(in.Machine.Image, machineImage, s); err != nil {
			return err
		}
		out.MachineImage = machineImage
	}

	if in.Volume != nil {
		out.VolumeSize = in.Volume.Size
		out.VolumeType = in.Volume.Type
	}

	var kubeletConfig *KubeletConfig
	if in.Kubernetes != nil {
		kubeletConfig = &KubeletConfig{}
		if err := autoConvert_garden_KubeletConfig_To_v1beta1_KubeletConfig(in.Kubernetes.Kubelet, kubeletConfig, s); err != nil {
			return err
		}
	}
	out.Kubelet = kubeletConfig

	return nil
}

func Convert_v1beta1_PacketWorker_To_garden_Worker(in *PacketWorker, out *garden.Worker, s conversion.Scope) error {
	return nil
}

func Convert_garden_Networking_To_v1beta1_Networking(in *garden.Networking, out *Networking, s conversion.Scope) error {
	if err := autoConvert_garden_Networking_To_v1beta1_Networking(in, out, s); err != nil {
		return err
	}

	out.K8SNetworks = K8SNetworks{
		Nodes:    &in.Nodes,
		Pods:     in.Pods,
		Services: in.Services,
	}

	return nil
}

func Convert_v1beta1_Networking_To_garden_Networking(in *Networking, out *garden.Networking, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Networking_To_garden_Networking(in, out, s); err != nil {
		return err
	}

	if in.K8SNetworks.Nodes != nil {
		out.Nodes = *in.K8SNetworks.Nodes
	}
	out.Pods = in.K8SNetworks.Pods
	out.Services = in.K8SNetworks.Services

	return nil
}
