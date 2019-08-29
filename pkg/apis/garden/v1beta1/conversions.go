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

	azureinstall "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/install"
	azurev1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-azure/pkg/apis/azure/v1alpha1"
	openstackinstall "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/install"
	openstackv1alpha1 "github.com/gardener/gardener-extensions/controllers/provider-openstack/pkg/apis/openstack/v1alpha1"
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

func Convert_v1beta1_Worker_To_garden_Worker(in *Worker, out *garden.Worker, s conversion.Scope) error {
	if err := autoConvert_v1beta1_Worker_To_garden_Worker(in, out, s); err != nil {
		return err
	}

	if in.MaxSurge == nil {
		out.MaxSurge = DefaultWorkerMaxSurge
	} else {
		out.MaxSurge = *in.MaxSurge
	}
	if in.MaxUnavailable == nil {
		out.MaxUnavailable = DefaultWorkerMaxUnavailable
	} else {
		out.MaxUnavailable = *in.MaxUnavailable
	}

	return nil
}

func Convert_garden_Worker_To_v1beta1_Worker(in *garden.Worker, out *Worker, s conversion.Scope) error {
	if err := autoConvert_garden_Worker_To_v1beta1_Worker(in, out, s); err != nil {
		return err
	}

	out.MaxSurge = &in.MaxSurge
	out.MaxUnavailable = &in.MaxUnavailable

	return nil
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
			if _, _, err := decoder.Decode(providerConfig.Raw, nil, cloudProfileConfig); err != nil {
				// If an error occurs then the provider config information contains invalid syntax, and in this
				// case we don't want to fail here. We rather don't try to migrate.
				klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
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
		out.Spec.ProviderConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Object: cloudProfileConfig,
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
			if _, _, err := decoder.Decode(providerConfig.Raw, nil, cloudProfileConfig); err != nil {
				// If an error occurs then the provider config information contains invalid syntax, and in this
				// case we don't want to fail here. We rather don't try to migrate.
				klog.Errorf("Cannot decode providerConfig of core.gardener.cloud/v1alpha1.CloudProfile %s", in.Name)
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
		out.Spec.ProviderConfig = &garden.ProviderConfig{
			RawExtension: runtime.RawExtension{
				Object: cloudProfileConfig,
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
	} else {
		out.Spec.ProviderConfig = nil
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
